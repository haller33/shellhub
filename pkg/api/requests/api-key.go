package requests

import (
	"github.com/shellhub-io/shellhub/pkg/api/query"
)

type CreateAPIKey struct {
	UserID    string `header:"X-ID"`
	TenantID  string `header:"X-Tenant-ID"`
	Role      string `header:"X-Role"`
	Name      string `json:"name" validate:"required,api-key_name"`
	ExpiresAt int    `json:"expires_at" validate:"required,api-key_expires-at"`
	Key       string `json:"key" validate:"omitempty,uuid"`
	OptRole   string `json:"role" validate:"omitempty,role"`
}

type ListAPIKey struct {
	TenantID string `header:"X-Tenant-ID"`
	query.Paginator
	query.Sorter
}

type UpdateAPIKey struct {
	UserID   string `header:"X-ID"`
	TenantID string `header:"X-Tenant-ID"`
	// CurrentName is the current stored name. It is different from [UpdateAPIKey.Name], which is used
	// to handle the new target name (optional).
	CurrentName string `param:"name" validate:"required"`
	Name        string `json:"name" validate:"omitempty,api-key_name"`
	Role        string `json:"role" validate:"omitempty,role"`
}

type DeleteAPIKey struct {
	TenantID string `header:"X-Tenant-ID"`
	Name     string `param:"name" validate:"required"`
}
