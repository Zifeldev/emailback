package models

import "time"

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`    
	Role         string    `json:"role"` 
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RefreshToken struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	Revoked   bool      `json:"revoked"`
}

// TokenBlacklist represents an invalidated JWT token
type TokenBlacklist struct {
	ID            string    `json:"id"`
	TokenJTI      string    `json:"token_jti"` 
	UserID        int64     `json:"user_id"`
	BlacklistedAt time.Time `json:"blacklisted_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	Reason        string    `json:"reason"` 
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` 
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type AccessTokenClaims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"` 
	JTI    string `json:"jti"` 
}

type RefreshTokenClaims struct {
	UserID  int64 `json:"user_id"`
	TokenID int64 `json:"token_id"`
}

// Admin request models
type CreateUserRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Role     string `json:"role" binding:"required"`
}

type UpdateRoleRequest struct {
	Role string `json:"role" binding:"required"` 
}
