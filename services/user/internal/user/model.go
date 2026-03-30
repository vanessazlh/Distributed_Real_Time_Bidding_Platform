package user

// User represents a registered user in the system.
type User struct {
	UserID       string `dynamodbav:"user_id" json:"user_id"`
	Email        string `dynamodbav:"email" json:"email"`
	PasswordHash string `dynamodbav:"password_hash" json:"-"`
	Username     string `dynamodbav:"username" json:"username"`
	Role         string `dynamodbav:"role" json:"role"`
	CreatedAt    string `dynamodbav:"created_at" json:"created_at"`
}

// RegisterRequest is the payload for POST /users.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Username string `json:"username" binding:"required,min=2"`
	Role     string `json:"role"`
}

// LoginRequest is the payload for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}
