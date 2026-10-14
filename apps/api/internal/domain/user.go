package domain

// User represents an authenticated user in the system.
type User struct {
	// UserID is the unique identifier for the user.
	UserID string `json:"user_id"`
	// AccountID is the account associated with the user, if any.
	AccountID string `json:"account_id,omitempty"`
	// Email is the email address of the user.
	Email string `json:"email"`
	// DisplayName is the human-readable display name.
	DisplayName string `json:"display_name"`
	// CreatedAt is the ISO timestamp when the user was created.
	CreatedAt string `json:"created_at"`
	// LastLoginAt is the ISO timestamp of the user's latest login.
	LastLoginAt string `json:"last_login_at"`
	// UpdatedAt is the ISO timestamp when the user was last modified.
	UpdatedAt string `json:"updated_at"`
}
