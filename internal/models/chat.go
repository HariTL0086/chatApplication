package models

import (
	"time"

	"github.com/gofrs/uuid"
)

// Conversation represents a chat conversation
type Conversation struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Type           string     `json:"type" gorm:"not null"` // "private", "group", etc.
	CreatedAt      time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"not null"`
	LastMessageAt  *time.Time `json:"last_message_at"`
	
	// Relationships
	Messages       []Message  `json:"messages,omitempty" gorm:"foreignKey:ConversationID"`
	Participants   []User     `json:"participants,omitempty" gorm:"many2many:conversation_participants;"`
}

// Message represents a chat message
type Message struct {
	ID               uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ConversationID   uuid.UUID `json:"conversation_id" gorm:"type:uuid;not null"`
	SenderID         uuid.UUID `json:"sender_id" gorm:"type:uuid;not null"`
	EncryptedContent string    `json:"encrypted_content" gorm:"not null"`
	MessageType      string    `json:"message_type" gorm:"not null;default:'text'"`
	MessageStatus    string    `json:"message_status" gorm:"not null;default:'sent'"`
	CreatedAt        time.Time `json:"created_at" gorm:"not null"`
	
	// Relationships
	Sender         User         `json:"sender,omitempty" gorm:"foreignKey:SenderID"`
	Conversation   Conversation `json:"conversation,omitempty" gorm:"foreignKey:ConversationID"`
}

type UserKey struct {
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;primary_key"`
	PublicKey string    `json:"public_key" gorm:"not null"`
	CreatedAt time.Time `json:"created_at" gorm:"not null"`
}

// Request/Response models
type StartChatRequest struct {
	RecipientID uuid.UUID `json:"recipient_id"`
}

type SendMessageRequest struct {
	ConversationID   uuid.UUID `json:"conversation_id"`
	EncryptedContent string    `json:"encrypted_content"`
	MessageType      string    `json:"message_type"`
}

type JoinRoomRequest struct {
	ConversationID uuid.UUID `json:"conversation_id"`
}

func (Conversation) TableName() string {
	return "conversations"
}

func (Message) TableName() string {
	return "messages"
}

func (UserKey) TableName() string {
	return "user_keys"
}