package domain

type User struct {
	UserID      string `json:"user_id"`
	AccountID   string `json:"account_id,omitempty"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
	LastLoginAt string `json:"last_login_at"`
	UpdatedAt   string `json:"updated_at"`
}
