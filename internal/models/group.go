package models

import (
	"time"

	"github.com/gofrs/uuid"
)

// Group represents a group chat
type Group struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name        string     `json:"name" gorm:"not null"`
	Description string     `json:"description"`
	CreatedBy   uuid.UUID  `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt   time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"not null"`
	
	// Relationships
	Members     []GroupMember `json:"members,omitempty" gorm:"foreignKey:GroupID"`
	Conversation *Conversation `json:"conversation,omitempty" gorm:"foreignKey:GroupID"`
}

// GroupMember represents a member in a group
type GroupMember struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	GroupID   uuid.UUID `json:"group_id" gorm:"type:uuid;not null"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;not null"`
	Role      string    `json:"role" gorm:"not null;default:'member'"` // "admin", "member"
	JoinedAt  time.Time `json:"joined_at" gorm:"not null"`
	
	// Relationships
	Group      Group      `json:"group,omitempty" gorm:"foreignKey:GroupID"`
	User       User       `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// Request/Response models for group operations
type CreateGroupRequest struct {
	Name        string   `json:"name" binding:"required,min=1,max=100"`
	Description string   `json:"description" binding:"max=500"`
	MemberIDs   []string `json:"member_ids"` // Array of user IDs to add as members
}

type UpdateGroupRequest struct {
	
	Name        string `json:"name" binding:"min=1,max=100"`
	Description string `json:"description" binding:"max=500"`
}

type AddGroupMemberRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"oneof=admin member"`
}

type RemoveGroupMemberRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

type ChangeMemberRoleRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required,oneof=admin member"`
}

type GroupResponse struct {
	Group   *Group         `json:"group"`
	Members []GroupMember  `json:"members"`
}

type GroupListResponse struct {
	Groups []Group `json:"groups"`
}

func (Group) TableName() string {
	return "groups"
}

func (GroupMember) TableName() string {
	return "group_members"
} 