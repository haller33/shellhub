package services

import (
	"context"
	"errors"
	"strings"

	"github.com/shellhub-io/shellhub/api/store"
	"github.com/shellhub-io/shellhub/pkg/api/auth"
	req "github.com/shellhub-io/shellhub/pkg/api/internalclient"
	"github.com/shellhub-io/shellhub/pkg/api/query"
	"github.com/shellhub-io/shellhub/pkg/api/requests"
	"github.com/shellhub-io/shellhub/pkg/envs"
	"github.com/shellhub-io/shellhub/pkg/models"
	"github.com/shellhub-io/shellhub/pkg/uuid"
)

type NamespaceService interface {
	ListNamespaces(ctx context.Context, paginator query.Paginator, filters query.Filters, export bool) ([]models.Namespace, int, error)
	CreateNamespace(ctx context.Context, namespace requests.NamespaceCreate, userID string) (*models.Namespace, error)
	GetNamespace(ctx context.Context, tenantID string) (*models.Namespace, error)
	DeleteNamespace(ctx context.Context, tenantID string) error

	// EditNamespace updates a namespace for the specified requests.NamespaceEdit#Tenant.
	// It returns the namespace with the updated fields and an error, if any.
	EditNamespace(ctx context.Context, req *requests.NamespaceEdit) (*models.Namespace, error)

	AddNamespaceUser(ctx context.Context, memberUsername, memberRole, tenantID, userID string) (*models.Namespace, error)
	RemoveNamespaceUser(ctx context.Context, tenantID, memberID, userID string) (*models.Namespace, error)
	EditNamespaceUser(ctx context.Context, tenantID, userID, memberID, memberNewRole string) error
	EditSessionRecordStatus(ctx context.Context, sessionRecord bool, tenantID string) error
	GetSessionRecord(ctx context.Context, tenantID string) (bool, error)
}

// ListNamespaces lists selected namespaces from a user.
//
// It receives a context, used to "control" the request flow, a pagination query, that indicate how many registers are
// requested per page, a filter string, a base64 encoded value what is converted to a slice of models.Filter and an
// export flag.
//
// ListNamespaces returns a slice of models.Namespace, the total of namespaces and an error. When error is not nil, the
// slice of models.Namespace is nil, total is zero.
func (s *service) ListNamespaces(ctx context.Context, paginator query.Paginator, filters query.Filters, export bool) ([]models.Namespace, int, error) {
	namespaces, count, err := s.store.NamespaceList(ctx, paginator, filters, export)
	if err != nil {
		return nil, 0, NewErrNamespaceList(err)
	}

	for index, namespace := range namespaces {
		members, err := s.fillMembersData(ctx, namespace.Members)
		if err != nil {
			return nil, 0, NewErrNamespaceMemberFillData(err)
		}

		namespaces[index].Members = members
	}

	return namespaces, count, nil
}

// CreateNamespace creates a new namespace.
func (s *service) CreateNamespace(ctx context.Context, namespace requests.NamespaceCreate, userID string) (*models.Namespace, error) {
	user, _, err := s.store.UserGetByID(ctx, userID, false)
	if err != nil || user == nil {
		return nil, NewErrUserNotFound(userID, err)
	}

	// When MaxNamespaces is less than zero, it means that the user has no limit of namespaces.
	if user.MaxNamespaces > 0 && user.MaxNamespaces <= user.Namespaces {
		return nil, NewErrNamespaceLimitReached(user.MaxNamespaces, nil)
	}

	ns := &models.Namespace{
		Name:  strings.ToLower(namespace.Name),
		Owner: user.ID,
		Members: []models.Member{
			{
				ID:   user.ID,
				Role: auth.RoleOwner,
			},
		},
		Settings: &models.NamespaceSettings{
			SessionRecord:          true,
			ConnectionAnnouncement: "",
		},
		TenantID: namespace.TenantID,
	}

	if ok, err := s.validator.Struct(ns); !ok || err != nil {
		return nil, NewErrNamespaceInvalid(err)
	}

	if namespace.TenantID == "" {
		ns.TenantID = uuid.Generate()
	}

	// Set limits according to ShellHub instance type
	if envs.IsCloud() {
		// cloud free plan is limited only by the max of devices
		ns.MaxDevices = 3
	} else {
		// we don't set limits on enterprise and community instances
		ns.MaxDevices = -1
	}

	otherNamespace, err := s.store.NamespaceGetByName(ctx, ns.Name)
	if err != nil && err != store.ErrNoDocuments {
		return nil, NewErrNamespaceNotFound(ns.Name, err)
	}

	if otherNamespace != nil {
		return nil, NewErrNamespaceDuplicated(nil)
	}

	if _, err := s.store.NamespaceCreate(ctx, ns); err != nil {
		return nil, NewErrNamespaceCreateStore(err)
	}

	return ns, nil
}

// GetNamespace gets a namespace.
//
// It receives a context, used to "control" the request flow and the tenant ID from models.Namespace.
//
// GetNamespace returns a models.Namespace and an error. When error is not nil, the models.Namespace is nil.
func (s *service) GetNamespace(ctx context.Context, tenantID string) (*models.Namespace, error) {
	namespace, err := s.store.NamespaceGet(ctx, tenantID, true)
	if err != nil || namespace == nil {
		return nil, NewErrNamespaceNotFound(tenantID, err)
	}

	members, err := s.fillMembersData(ctx, namespace.Members)
	if err != nil {
		return nil, NewErrNamespaceMemberFillData(err)
	}

	namespace.Members = members

	return namespace, nil
}

// DeleteNamespace deletes a namespace.
//
// It receives a context, used to "control" the request flow and the tenant ID from models.Namespace.
//
// When cloud and billing is enabled, it will try to delete the namespace's billing information from the billing
// service if it exists.
func (s *service) DeleteNamespace(ctx context.Context, tenantID string) error {
	ns, err := s.store.NamespaceGet(ctx, tenantID, true)
	if err != nil {
		return NewErrNamespaceNotFound(tenantID, err)
	}

	ableToReportDeleteNamespace := func(ns *models.Namespace) bool {
		return !ns.Billing.IsNil() && ns.Billing.HasCutomer() && ns.Billing.HasSubscription()
	}

	if envs.IsCloud() && envs.HasBilling() && ableToReportDeleteNamespace(ns) {
		if err := s.BillingReport(s.client.(req.Client), tenantID, ReportNamespaceDelete); err != nil {
			return NewErrBillingReportNamespaceDelete(err)
		}
	}

	return s.store.NamespaceDelete(ctx, tenantID)
}

// fillMembersData fill the member data with the user data.
//
// This method exist because the namespace stores only the user ID and the role from its member as a list of models.Member.
// To avoid unnecessary calls to store for member information, member username, this "conversion" is ony made when
// required by the service.
//
// It receives a context, used to "control" the request flow and a slice of models.Member with just ID and return an
// other slice with ID, username and role set.
//
// fillMembersData returns a slice of models.Member and an error. When error is not nil, the slice of models.Member is nil.
func (s *service) fillMembersData(ctx context.Context, members []models.Member) ([]models.Member, error) {
	for index, member := range members {
		user, _, err := s.store.UserGetByID(ctx, member.ID, false)
		if err != nil || user == nil {
			return nil, NewErrUserNotFound(member.ID, err)
		}

		members[index] = models.Member{ID: user.ID, Username: user.Username, Role: member.Role}
	}

	return members, nil
}

func (s *service) EditNamespace(ctx context.Context, req *requests.NamespaceEdit) (*models.Namespace, error) {
	changes := &models.NamespaceChanges{
		Name:                   strings.ToLower(req.Name),
		SessionRecord:          req.Settings.SessionRecord,
		ConnectionAnnouncement: req.Settings.ConnectionAnnouncement,
	}

	if err := s.store.NamespaceEdit(ctx, req.Tenant, changes); err != nil {
		switch {
		case errors.Is(err, store.ErrNoDocuments):
			return nil, NewErrNamespaceNotFound(req.Tenant, err)
		default:
			return nil, err
		}
	}

	return s.store.NamespaceGet(ctx, req.Tenant, true)
}

// AddNamespaceUser adds a member to a namespace.
//
// It receives a context, used to "control" the request flow, the member's name, the member's role, the tenant ID from
// models.Namespace what receive the member and the user ID from models.User who is adding the new member.
//
// If user from user's ID has a role what does not allow to add a new member or the member's role is the same as the user
// one, AddNamespaceUser will return error.
//
// AddNamespaceUser returns a models.Namespace and an error. When error is not nil, the models.Namespace is nil.
// TODO: use a request here
func (s *service) AddNamespaceUser(ctx context.Context, memberUsername, role, tenantID, userID string) (*models.Namespace, error) {
	memberRole := auth.RoleFromString(role)

	if ok, err := s.validator.Struct(models.Member{Username: memberUsername, Role: memberRole}); !ok || err != nil {
		return nil, NewErrNamespaceMemberInvalid(err)
	}

	namespace, err := s.store.NamespaceGet(ctx, tenantID, true)
	if err != nil || namespace == nil {
		return nil, NewErrNamespaceNotFound(tenantID, err)
	}

	// user is the user who is adding the new member.
	user, _, err := s.store.UserGetByID(ctx, userID, false)
	if err != nil || user == nil {
		return nil, NewErrUserNotFound(userID, err)
	}

	// checks if the active member is in the namespace. user is the active member.
	active, ok := namespace.FindMember(user.ID)
	if !ok {
		return nil, NewErrNamespaceMemberNotFound(user.ID, err)
	}

	passive, err := s.store.UserGetByUsername(ctx, memberUsername)
	if err != nil {
		return nil, NewErrUserNotFound(memberUsername, err)
	}

	// checks if the passive member is in the namespace.
	if _, ok = namespace.FindMember(passive.ID); ok {
		return nil, NewErrNamespaceMemberDuplicated(passive.ID, nil)
	}

	if !active.Role.HasAuthority(memberRole) {
		return nil, NewErrRoleInvalid()
	}

	return s.store.NamespaceAddMember(ctx, tenantID, passive.ID, memberRole)
}

// RemoveNamespaceUser removes member from a namespace.
//
// It receives a context, used to "control" the request flow, the tenant ID from models.Namespace, member ID to remove
// and the user ID from models.User who is removing the member.
//
// If user from user's ID has a role what does not allow to remove a member or the member's role is the same as the user
// one, RemoveNamespaceUser will return error.
//
// RemoveNamespaceUser returns a models.Namespace and an error. When error is not nil, the models.Namespace is nil.
func (s *service) RemoveNamespaceUser(ctx context.Context, tenantID, memberID, userID string) (*models.Namespace, error) {
	namespace, err := s.store.NamespaceGet(ctx, tenantID, true)
	if err != nil {
		return nil, NewErrNamespaceNotFound(tenantID, err)
	}

	// checks if the user exist.
	// user is the user who is removing the member.
	user, _, err := s.store.UserGetByID(ctx, userID, false)
	if err != nil {
		return nil, NewErrUserNotFound(userID, err)
	}

	// checks if the member exist.
	// member is the member who will be removed.
	member, _, err := s.store.UserGetByID(ctx, memberID, false)
	if err != nil {
		return nil, NewErrUserNotFound(memberID, err)
	}

	// checks if the active member is in the namespace. user is the active member.
	active, ok := namespace.FindMember(user.ID)
	if !ok {
		return nil, NewErrNamespaceMemberNotFound(user.ID, err)
	}

	// checks if the passive member is in the namespace. member is the passive member.
	passive, ok := namespace.FindMember(member.ID)
	if !ok {
		return nil, NewErrNamespaceMemberNotFound(member.ID, err)
	}

	// checks if the active member can act over the passive member.
	if !active.Role.HasAuthority(passive.Role) {
		return nil, NewErrRoleInvalid()
	}

	removed, err := s.store.NamespaceRemoveMember(ctx, tenantID, member.ID)
	if err != nil {
		return nil, err
	}

	s.AuthUncacheToken(ctx, namespace.TenantID, member.ID) // nolint: errcheck

	return removed, nil
}

// EditNamespaceUser edits a member's role.
//
// It receives a context, used to "control" the request flow, the tenant ID from models.Namespace, user ID from
// models.User who is editing the member and the member's new role.
//
// If user from user's ID has a role what does not allow to edit a member or the member's role is the same as the user
// one, EditNamespaceUser will return error.
func (s *service) EditNamespaceUser(ctx context.Context, tenantID, userID, memberID, memberNewRole string) error {
	namespace, err := s.store.NamespaceGet(ctx, tenantID, true)
	if err != nil {
		return NewErrNamespaceNotFound(tenantID, err)
	}

	// user is the user who is editing the member.
	user, _, err := s.store.UserGetByID(ctx, userID, false)
	if err != nil {
		return NewErrUserNotFound(userID, err)
	}

	// member is the member who will be edited.
	member, _, err := s.store.UserGetByID(ctx, memberID, false)
	if err != nil {
		return NewErrUserNotFound(memberID, err)
	}

	// checks if the active member is in the namespace. user is the active member.
	active, ok := namespace.FindMember(user.ID)
	if !ok {
		return NewErrNamespaceMemberNotFound(user.ID, err)
	}

	// checks if the passive member is in the namespace. member is the passive member.
	passive, ok := namespace.FindMember(member.ID)
	if !ok {
		return NewErrNamespaceMemberNotFound(member.ID, err)
	}

	// checks if the active member can act over the passive member.
	if !active.Role.HasAuthority(passive.Role) {
		return NewErrRoleInvalid()
	}

	if err := s.store.NamespaceEditMember(ctx, tenantID, member.ID, auth.RoleFromString(memberNewRole)); err != nil {
		return err
	}

	s.AuthUncacheToken(ctx, namespace.TenantID, member.ID) // nolint: errcheck

	return nil
}

// EditSessionRecordStatus defines if the sessions will be recorded.
//
// It receives a context, used to "control" the request flow, a boolean to define if the sessions will be recorded and
// the tenant ID from models.Namespace.
func (s *service) EditSessionRecordStatus(ctx context.Context, sessionRecord bool, tenantID string) error {
	return s.store.NamespaceSetSessionRecord(ctx, sessionRecord, tenantID)
}

// GetSessionRecord gets the session record data.
//
// It receives a context, used to "control" the request flow, the tenant ID from models.Namespace.
//
// GetSessionRecord returns a boolean indicating the session record status and an error. When error is not nil,
// the boolean is false.
func (s *service) GetSessionRecord(ctx context.Context, tenantID string) (bool, error) {
	if _, err := s.store.NamespaceGet(ctx, tenantID, false); err != nil {
		return false, NewErrNamespaceNotFound(tenantID, err)
	}

	return s.store.NamespaceGetSessionRecord(ctx, tenantID)
}
