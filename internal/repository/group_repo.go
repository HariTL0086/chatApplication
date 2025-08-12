package repository

import (
	"Chat_App/internal/models"
	"context"
	"errors"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

var (
	ErrGroupNotFound     = errors.New("group not found")
	ErrMemberNotFound    = errors.New("member not found")
	ErrAlreadyMember     = errors.New("user is already a member of this group")
	ErrNotGroupAdmin     = errors.New("user is not an admin of this group")
	ErrCannotRemoveAdmin = errors.New("cannot remove the last admin from the group")
)

type GroupRepository struct {
	db *gorm.DB
}

func NewGroupRepository(db *gorm.DB) *GroupRepository {
	return &GroupRepository{db: db}
}

// CreateGroup creates a new group
func (r *GroupRepository) CreateGroup(ctx context.Context, group *models.Group) error {
	return r.db.WithContext(ctx).Create(group).Error
}

// GetGroupByID retrieves a group by ID
func (r *GroupRepository) GetGroupByID(ctx context.Context, groupID uuid.UUID) (*models.Group, error) {
	var group models.Group
	err := r.db.WithContext(ctx).Preload("Members.User").First(&group, "id = ?", groupID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}
	return &group, nil
}

// GetUserGroups retrieves all groups a user is a member of
func (r *GroupRepository) GetUserGroups(ctx context.Context, userID uuid.UUID) ([]*models.Group, error) {
	var groups []*models.Group
	err := r.db.WithContext(ctx).
		Joins("JOIN group_members ON groups.id = group_members.group_id").
		Where("group_members.user_id = ?", userID).
		Preload("Members.User").
		Find(&groups).Error
	return groups, err
}

// AddGroupMember adds a user to a group
func (r *GroupRepository) AddGroupMember(ctx context.Context, groupID, userID uuid.UUID, role string) error {
	// Check if user is already a member
	var existingMember models.GroupMember
	err := r.db.WithContext(ctx).Where("group_id = ? AND user_id = ?", groupID, userID).First(&existingMember).Error
	if err == nil {
		return ErrAlreadyMember
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	member := &models.GroupMember{
		GroupID:  groupID,
		UserID:   userID,
		Role:     role,
		JoinedAt: r.db.NowFunc(),
	}

	return r.db.WithContext(ctx).Create(member).Error
}

// RemoveGroupMember removes a user from a group
func (r *GroupRepository) RemoveGroupMember(ctx context.Context, groupID, userID uuid.UUID) error {
	// Check if this is the last admin
	var adminCount int64
	err := r.db.WithContext(ctx).Model(&models.GroupMember{}).
		Where("group_id = ? AND role = ?", groupID, "admin").Count(&adminCount).Error
	if err != nil {
		return err
	}

	var member models.GroupMember
	err = r.db.WithContext(ctx).Where("group_id = ? AND user_id = ?", groupID, userID).First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMemberNotFound
		}
		return err
	}

	// If this is the last admin and we're trying to remove them, prevent it
	if member.Role == "admin" && adminCount == 1 {
		return ErrCannotRemoveAdmin
	}

	return r.db.WithContext(ctx).Delete(&member).Error
}

// UpdateMemberRole updates a member's role in the group
func (r *GroupRepository) UpdateMemberRole(ctx context.Context, groupID, userID uuid.UUID, role string) error {
	result := r.db.WithContext(ctx).Model(&models.GroupMember{}).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Update("role", role)
	
	if result.Error != nil {
		return result.Error
	}
	
	if result.RowsAffected == 0 {
		return ErrMemberNotFound
	}
	
	return nil
}

// GetGroupMembers retrieves all members of a group
func (r *GroupRepository) GetGroupMembers(ctx context.Context, groupID uuid.UUID) ([]*models.GroupMember, error) {
	var members []*models.GroupMember
	err := r.db.WithContext(ctx).Where("group_id = ?", groupID).Preload("User").Find(&members).Error
	return members, err
}

// IsGroupMember checks if a user is a member of a group
func (r *GroupRepository) IsGroupMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.GroupMember{}).
		Where("group_id = ? AND user_id = ?", groupID, userID).Count(&count).Error
	return count > 0, err
}

// IsGroupAdmin checks if a user is an admin of a group
func (r *GroupRepository) IsGroupAdmin(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.GroupMember{}).
		Where("group_id = ? AND user_id = ? AND role = ?", groupID, userID, "admin").Count(&count).Error
	return count > 0, err
}

// UpdateGroup updates group information
func (r *GroupRepository) UpdateGroup(ctx context.Context, group *models.Group) error {
	return r.db.WithContext(ctx).Save(group).Error
}

// DeleteGroup deletes a group and all its members
func (r *GroupRepository) DeleteGroup(ctx context.Context, groupID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Group{}, "id = ?", groupID).Error
}

// GetDB returns the underlying database connection for transaction management
func (r *GroupRepository) GetDB() *gorm.DB {
	return r.db
}

// DeleteAllGroupMembers deletes all members of a group
func (r *GroupRepository) DeleteAllGroupMembers(ctx context.Context, groupID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("group_id = ?", groupID).Delete(&models.GroupMember{}).Error
} 