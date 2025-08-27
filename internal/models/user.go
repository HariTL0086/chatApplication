package models

import (
	"time"

	"github.com/gofrs/uuid"
)


type User struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Username     string    `json:"username" gorm:"not null;unique"`
	Email        string    `json:"email" gorm:"not null;unique"`
	Password     string    `json:"-" gorm:"not null"`
	PublicKey    string    `json:"public_key" gorm:"type:text"`
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

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}


type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}


type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

type RefreshToken struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;not null"`
	TokenHash string    `json:"-" gorm:"not null;unique"`
	ExpiresAt time.Time `json:"expires_at" gorm:"not null"`
	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	
	User User `json:"-" gorm:"foreignKey:UserID"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}


type RefreshTokenByEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}


type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}


type LogoutByEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}
