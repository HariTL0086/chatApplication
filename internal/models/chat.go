package models

import (
	"time"

	"github.com/gofrs/uuid"
)


type Conversation struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Type           string     `json:"type" gorm:"not null"` // "private", "group", etc.
	GroupID        *uuid.UUID `json:"group_id,omitempty" gorm:"type:uuid"` // For group conversations
	CreatedAt      time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"not null"`
	LastMessageAt  *time.Time `json:"last_message_at"`
	
	// Relationships
	Messages       []Message  `json:"messages,omitempty" gorm:"foreignKey:ConversationID"`
	Participants   []User     `json:"participants,omitempty" gorm:"many2many:conversation_participants;"`
	Group          *Group     `json:"group,omitempty" gorm:"foreignKey:GroupID"`
}


type Message struct {
	ID               uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ConversationID   uuid.UUID `json:"conversation_id" gorm:"type:uuid;not null"`
	SenderID         uuid.UUID `json:"sender_id" gorm:"type:uuid;not null"`
	Content          string    `json:"content" gorm:"not null"`
	MessageType      string    `json:"message_type" gorm:"not null;default:'text'"`
	MessageStatus    string    `json:"message_status" gorm:"not null;default:'sent'"`
	CreatedAt        time.Time `json:"created_at" gorm:"not null"`
	
	// Relationships
	Sender         User         `json:"sender,omitempty" gorm:"foreignKey:SenderID"`
	Conversation   Conversation `json:"conversation,omitempty" gorm:"foreignKey:ConversationID"`
}



type StartChatRequest struct {
	RecipientID uuid.UUID `json:"recipient_id"`
}

type SendMessageRequest struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	Content        string    `json:"content"`
	MessageType    string    `json:"message_type"`
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

