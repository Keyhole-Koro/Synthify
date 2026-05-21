package domain

type User struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
	LastLoginAt string `json:"last_login_at"`
	UpdatedAt   string `json:"updated_at"`
}
