package guard

type Permissions []int

const (
	DeviceAccept = iota + 1
	DeviceReject
	DeviceUpdate
	DeviceRemove
	DeviceConnect
	DeviceRename
	DeviceDetails

	DeviceCreateTag
	DeviceUpdateTag
	DeviceRemoveTag
	DeviceRenameTag
	DeviceDeleteTag

	SessionPlay
	SessionClose
	SessionRemove
	SessionDetails

	FirewallCreate
	FirewallEdit
	FirewallRemove

	FirewallAddTag
	FirewallRemoveTag
	FirewallUpdateTag

	PublicKeyCreate
	PublicKeyEdit
	PublicKeyRemove

	PublicKeyAddTag
	PublicKeyRemoveTag
	PublicKeyUpdateTag

	NamespaceUpdate
	NamespaceAddMember
	NamespaceRemoveMember
	NamespaceEditMember
	NamespaceEnableSessionRecord
	NamespaceDelete

	BillingCreateCustomer
	BillingChooseDevices
	BillingAddPaymentMethod
	BillingUpdatePaymentMethod
	BillingRemovePaymentMethod
	BillingCancelSubscription
	BillingCreateSubscription
	BillingGetPaymentMethod
	BillingGetSubscription

	APIKeyCreate
	APIKeyEdit
	APIKeyDelete

	ConnectorUpdate
	ConnectorSet
	ConnectorStatus
	ConnectorAction
)

var observerPermissions = Permissions{
	DeviceConnect,
	DeviceDetails,
	SessionDetails,
}

var operatorPermissions = Permissions{
	DeviceAccept,
	DeviceReject,
	DeviceConnect,
	DeviceRename,
	DeviceDetails,
	DeviceUpdate,

	DeviceCreateTag,
	DeviceUpdateTag,
	DeviceRemoveTag,
	DeviceRenameTag,
	DeviceDeleteTag,

	SessionDetails,
}

var adminPermissions = Permissions{
	DeviceAccept,
	DeviceReject,
	DeviceRemove,
	DeviceConnect,
	DeviceRename,
	DeviceDetails,
	DeviceUpdate,

	DeviceCreateTag,
	DeviceUpdateTag,
	DeviceRemoveTag,
	DeviceRenameTag,
	DeviceDeleteTag,

	DeviceUpdate,

	SessionPlay,
	SessionClose,
	SessionRemove,
	SessionDetails,

	FirewallCreate,
	FirewallEdit,
	FirewallRemove,
	FirewallAddTag,
	FirewallRemoveTag,
	FirewallUpdateTag,

	PublicKeyCreate,
	PublicKeyEdit,
	PublicKeyRemove,
	PublicKeyAddTag,
	PublicKeyRemoveTag,
	PublicKeyUpdateTag,

	NamespaceUpdate,
	NamespaceAddMember,
	NamespaceRemoveMember,
	NamespaceEditMember,
	NamespaceEnableSessionRecord,

	APIKeyCreate,
	APIKeyEdit,
	APIKeyDelete,

	ConnectorUpdate,
	ConnectorSet,
	ConnectorStatus,
	ConnectorAction,
}

var ownerPermissions = Permissions{
	DeviceAccept,
	DeviceReject,
	DeviceRemove,
	DeviceConnect,
	DeviceRename,
	DeviceDetails,
	DeviceUpdate,

	DeviceCreateTag,
	DeviceUpdateTag,
	DeviceRemoveTag,
	DeviceRenameTag,
	DeviceDeleteTag,

	DeviceUpdate,

	SessionPlay,
	SessionClose,
	SessionRemove,
	SessionDetails,

	FirewallCreate,
	FirewallEdit,
	FirewallRemove,
	FirewallAddTag,
	FirewallRemoveTag,
	FirewallUpdateTag,

	PublicKeyCreate,
	PublicKeyEdit,
	PublicKeyRemove,
	PublicKeyAddTag,
	PublicKeyRemoveTag,
	PublicKeyUpdateTag,

	NamespaceUpdate,
	NamespaceAddMember,
	NamespaceRemoveMember,
	NamespaceEditMember,
	NamespaceEnableSessionRecord,
	NamespaceDelete,

	BillingCreateCustomer,
	BillingChooseDevices,
	BillingAddPaymentMethod,
	BillingUpdatePaymentMethod,
	BillingRemovePaymentMethod,
	BillingCancelSubscription,
	BillingCreateSubscription,
	BillingGetSubscription,

	APIKeyCreate,
	APIKeyEdit,
	APIKeyDelete,

	ConnectorUpdate,
	ConnectorSet,
	ConnectorStatus,
	ConnectorAction,
}
