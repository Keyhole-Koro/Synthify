package domain

// Account represents an organizational account entity that holds subscription, quota, and billing state.
type Account struct {
	// AccountID is the unique identifier of the account.
	AccountID string `json:"account_id"`
	// Name is the display name of the account.
	Name string `json:"name"`
	// Plan is the current subscription plan tier ("free" | "pro").
	Plan string `json:"plan"`
	// StorageQuotaBytes is the total storage limit in bytes allocated for this account.
	StorageQuotaBytes int64 `json:"storage_quota_bytes"`
	// StorageUsedBytes is the current total storage used across the account.
	StorageUsedBytes int64 `json:"storage_used_bytes"`
	// MaxFileSizeBytes is the maximum allowed file size per upload.
	MaxFileSizeBytes int64 `json:"max_file_size_bytes"`
	// MaxUploadsPerFiveH is the upload rate limit in a rolling 5-hour window.
	MaxUploadsPerFiveH int64 `json:"max_uploads_per_5h"`
	// MaxUploadsPerWeek is the upload rate limit in a rolling 1-week window.
	MaxUploadsPerWeek int64 `json:"max_uploads_per_1week"`
	// StripeCustomerID is the Stripe customer ID associated with billing.
	StripeCustomerID string `json:"stripe_customer_id,omitempty"`
	// StripeSubscriptionID is the Stripe subscription ID.
	StripeSubscriptionID string `json:"stripe_subscription_id,omitempty"`
	// BillingStatus is the current billing status (e.g., active, past_due, canceled).
	BillingStatus string `json:"billing_status,omitempty"`
	// StripePriceID is the active Stripe price identifier.
	StripePriceID string `json:"stripe_price_id,omitempty"`
	// BillingCurrency is the 3-letter currency code (e.g. jpy, usd).
	BillingCurrency string `json:"billing_currency,omitempty"`
	// BillingAmountMinor is the recurring charge amount in minor currency units (e.g., cents/yen).
	BillingAmountMinor int64 `json:"billing_amount_minor,omitempty"`
	// BillingInterval is the billing cycle frequency ("month" or "year").
	BillingInterval string `json:"billing_interval,omitempty"`
	// CurrentPeriodEnd is the ISO timestamp when the current billing cycle ends.
	CurrentPeriodEnd string `json:"current_period_end,omitempty"`
	// CancelAtPeriodEnd indicates whether the subscription will be canceled at the end of the period.
	CancelAtPeriodEnd bool `json:"cancel_at_period_end,omitempty"`
	// BillingUpdatedAt is the ISO timestamp when billing info was last updated.
	BillingUpdatedAt string `json:"billing_updated_at,omitempty"`

	// Usage-Based Billing (Phase 2)

	// BudgetLimitMinor is the spending limit for usage-based billing in minor currency units.
	BudgetLimitMinor int64 `json:"budget_limit_minor,omitempty"`
	// CurrentPeriodUsageMinor is the usage amount consumed during the current period.
	CurrentPeriodUsageMinor int64 `json:"current_period_usage_minor,omitempty"`
	// CurrentPeriodStartedAt is the ISO timestamp when the current usage period started.
	CurrentPeriodStartedAt string `json:"current_period_started_at,omitempty"`
	// BudgetExceeded indicates whether current period usage exceeds the budget limit.
	BudgetExceeded bool `json:"budget_exceeded,omitempty"`

	// Credits

	// CreditBalanceMinor is the remaining prepaid credit balance in minor currency units.
	CreditBalanceMinor int64 `json:"credit_balance_minor,omitempty"`

	// CreatedAt is the ISO timestamp when the account was created.
	CreatedAt string `json:"created_at"`
}

// AccountUser represents membership of a user within an account organization.
type AccountUser struct {
	// AccountID is the ID of the parent account.
	AccountID string `json:"account_id"`
	// UserID is the ID of the member user.
	UserID string `json:"user_id"`
	// Role is the organization role within the account (e.g., "owner", "admin", "member").
	Role string `json:"role"`
	// JoinedAt is the ISO timestamp when the user joined the account.
	JoinedAt string `json:"joined_at"`
}

// WorkspaceRole represents access permissions within a workspace: owner > editor > viewer.
type WorkspaceRole string

const (
	// WorkspaceRoleOwner has full control over the workspace, including member management.
	WorkspaceRoleOwner WorkspaceRole = "owner"
	// WorkspaceRoleEditor can view and modify documents, trees, and resources.
	WorkspaceRoleEditor WorkspaceRole = "editor"
	// WorkspaceRoleViewer has read-only access to workspace resources.
	WorkspaceRoleViewer WorkspaceRole = "viewer"
)

// CanWrite reports whether the workspace role is permitted to perform write operations
// such as adding or processing documents.
func (r WorkspaceRole) CanWrite() bool {
	return r == WorkspaceRoleOwner || r == WorkspaceRoleEditor
}

// CanManageMembers reports whether the workspace role is permitted to invite, update,
// or remove workspace members.
func (r WorkspaceRole) CanManageMembers() bool {
	return r == WorkspaceRoleOwner
}

// WorkspaceMember represents an invited collaborator on a workspace.
// Direct account owners do not require explicit WorkspaceMember records and are treated as owners.
type WorkspaceMember struct {
	// WorkspaceID is the identifier of the workspace.
	WorkspaceID string `json:"workspace_id"`
	// UserID is the identifier of the member user.
	UserID string `json:"user_id"`
	// Email is the email address of the member.
	Email string `json:"email"`
	// Role is the assigned access role within the workspace.
	Role WorkspaceRole `json:"role"`
	// InvitedBy is the user ID of the inviter.
	InvitedBy string `json:"invited_by"`
	// InvitedAt is the ISO timestamp when the member was invited.
	InvitedAt string `json:"invited_at"`
}

// ShareLink represents a public or token-based shareable link for unauthenticated or external workspace access.
type ShareLink struct {
	// Token is the secret access token for the link.
	Token string `json:"token"`
	// WorkspaceID is the workspace granted access to.
	WorkspaceID string `json:"workspace_id"`
	// Role is the granted permission level (typically viewer).
	Role WorkspaceRole `json:"role"`
	// CreatedBy is the user ID who generated the link.
	CreatedBy string `json:"created_by"`
	// ExpiresAt is the optional ISO timestamp when the link expires.
	ExpiresAt string `json:"expires_at,omitempty"`
	// Revoked indicates whether the share link has been manually invalidated.
	Revoked bool `json:"revoked"`
	// CreatedAt is the ISO timestamp when the share link was created.
	CreatedAt string `json:"created_at"`
}

// Workspace represents an isolated environment for managing documents, trees, and knowledge items.
type Workspace struct {
	// WorkspaceID is the unique identifier of the workspace.
	WorkspaceID string `json:"workspace_id"`
	// AccountID is the parent account owning this workspace.
	AccountID string `json:"account_id"`
	// Name is the human-readable workspace name.
	Name string `json:"name"`
	// Plan is the account plan inherited by the workspace.
	Plan string `json:"plan,omitempty"`
	// StorageUsedBytes is the current storage used by documents in this workspace.
	StorageUsedBytes int64 `json:"storage_used_bytes,omitempty"`
	// StorageQuotaBytes is the effective storage quota for the workspace.
	StorageQuotaBytes int64 `json:"storage_quota_bytes,omitempty"`
	// MaxFileSizeBytes is the maximum allowed single-file size in bytes.
	MaxFileSizeBytes int64 `json:"max_file_size_bytes,omitempty"`
	// MaxUploadsPerFiveH is the rolling 5-hour upload rate limit.
	MaxUploadsPerFiveH int64 `json:"max_uploads_per_5h,omitempty"`
	// MaxUploadsPerWeek is the rolling 1-week upload rate limit.
	MaxUploadsPerWeek int64 `json:"max_uploads_per_1week,omitempty"`
	// CreatedAt is the ISO timestamp when the workspace was created.
	CreatedAt string `json:"created_at"`
}
