package domain

type Account struct {
	AccountID            string `json:"account_id"`
	Name                 string `json:"name"`
	Plan                 string `json:"plan"` // "free" | "pro"
	StorageQuotaBytes    int64  `json:"storage_quota_bytes"`
	StorageUsedBytes     int64  `json:"storage_used_bytes"`
	MaxFileSizeBytes     int64  `json:"max_file_size_bytes"`
	MaxUploadsPerFiveH   int64  `json:"max_uploads_per_5h"`
	MaxUploadsPerWeek    int64  `json:"max_uploads_per_1week"`
	StripeCustomerID     string `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID string `json:"stripe_subscription_id,omitempty"`
	BillingStatus        string `json:"billing_status,omitempty"`
	StripePriceID        string `json:"stripe_price_id,omitempty"`
	BillingCurrency      string `json:"billing_currency,omitempty"`
	BillingAmountMinor   int64  `json:"billing_amount_minor,omitempty"`
	BillingInterval      string `json:"billing_interval,omitempty"`
	CurrentPeriodEnd     string `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd    bool   `json:"cancel_at_period_end,omitempty"`
	BillingUpdatedAt     string `json:"billing_updated_at,omitempty"`

	// Usage-Based Billing (Phase 2)
	BudgetLimitMinor          int64  `json:"budget_limit_minor,omitempty"`
	CurrentPeriodUsageMinor   int64  `json:"current_period_usage_minor,omitempty"`
	CurrentPeriodStartedAt    string `json:"current_period_started_at,omitempty"`
	BudgetExceeded            bool   `json:"budget_exceeded,omitempty"`

	// Credits
	CreditBalanceMinor int64 `json:"credit_balance_minor,omitempty"`

	CreatedAt            string `json:"created_at"`
}

type AccountUser struct {
	AccountID string `json:"account_id"`
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	JoinedAt  string `json:"joined_at"`
}

// WorkspaceRole は workspace 単位の権限。owner > editor > viewer。
type WorkspaceRole string

const (
	WorkspaceRoleOwner  WorkspaceRole = "owner"
	WorkspaceRoleEditor WorkspaceRole = "editor"
	WorkspaceRoleViewer WorkspaceRole = "viewer"
)

// CanWrite はドキュメント追加・処理などの書き込み操作が許可される role か。
func (r WorkspaceRole) CanWrite() bool {
	return r == WorkspaceRoleOwner || r == WorkspaceRoleEditor
}

// CanManageMembers はメンバー招待・role 変更・削除が許可される role か。
func (r WorkspaceRole) CanManageMembers() bool {
	return r == WorkspaceRoleOwner
}

// WorkspaceMember は workspace 単位で共有された招待メンバー。
// account 経由のアクセス (所有者) はこのテーブルには載らず owner 相当として扱う。
type WorkspaceMember struct {
	WorkspaceID string        `json:"workspace_id"`
	UserID      string        `json:"user_id"`
	Email       string        `json:"email"`
	Role        WorkspaceRole `json:"role"`
	InvitedBy   string        `json:"invited_by"`
	InvitedAt   string        `json:"invited_at"`
}

// ShareLink は公開リンク共有。token を知っていれば無認証で workspace を閲覧できる。
// role は閲覧専用に限定する想定 (課金操作は招待メンバーのみ)。
type ShareLink struct {
	Token       string        `json:"token"`
	WorkspaceID string        `json:"workspace_id"`
	Role        WorkspaceRole `json:"role"`
	CreatedBy   string        `json:"created_by"`
	ExpiresAt   string        `json:"expires_at,omitempty"`
	Revoked     bool          `json:"revoked"`
	CreatedAt   string        `json:"created_at"`
}

type Workspace struct {
	WorkspaceID        string `json:"workspace_id"`
	AccountID          string `json:"account_id"`
	Name               string `json:"name"`
	Plan               string `json:"plan,omitempty"`
	StorageUsedBytes   int64  `json:"storage_used_bytes,omitempty"`
	StorageQuotaBytes  int64  `json:"storage_quota_bytes,omitempty"`
	MaxFileSizeBytes   int64  `json:"max_file_size_bytes,omitempty"`
	MaxUploadsPerFiveH int64  `json:"max_uploads_per_5h,omitempty"`
	MaxUploadsPerWeek  int64  `json:"max_uploads_per_1week,omitempty"`
	CreatedAt          string `json:"created_at"`
}
