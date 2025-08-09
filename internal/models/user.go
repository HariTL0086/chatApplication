package models

import (
	"time"

	"github.com/gofrs/uuid"
)

// User
type User struct {
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	Password string    `json:"-"` 
	RefreshToken *string `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// RegisterRequest 
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// LoginRequest 
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse 
type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

// RefreshToken 
type RefreshToken struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	TokenHash string    `json:"-"` 
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// RefreshTokenRequest - request model for refresh token operations
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshTokenByEmailRequest - request model for email-based refresh token operations
type RefreshTokenByEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// LogoutRequest - request model for logout operations
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutByEmailRequest - request model for email-based logout operations
type LogoutByEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}
