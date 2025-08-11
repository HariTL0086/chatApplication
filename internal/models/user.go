package models

import (
	"time"

	"github.com/gofrs/uuid"
)

// User
type User struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Username     string    `json:"username" gorm:"not null;unique"`
	Email        string    `json:"email" gorm:"not null;unique"`
	Password     string    `json:"-" gorm:"not null"`
	RefreshToken *string   `json:"-"`
	CreatedAt    time.Time `json:"created_at" gorm:"not null"`
	
	// Relationships
	Messages       []Message       `json:"messages,omitempty" gorm:"foreignKey:SenderID"`
	Conversations  []Conversation  `json:"conversations,omitempty" gorm:"many2many:conversation_participants;"`
	RefreshTokens  []RefreshToken  `json:"-" gorm:"foreignKey:UserID"`
}

func (User) TableName() string {
	return "users"
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
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;not null"`
	TokenHash string    `json:"-" gorm:"not null;unique"`
	ExpiresAt time.Time `json:"expires_at" gorm:"not null"`
	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	
	// Relationships
	User User `json:"-" gorm:"foreignKey:UserID"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

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
