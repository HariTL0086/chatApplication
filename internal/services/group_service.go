package services

import (
	"Chat_App/internal/models"
	"Chat_App/internal/repository"
	"context"
	"errors"
	"time"

	"github.com/gofrs/uuid"
)

var (
	ErrGroupNotFound     = errors.New("group not found")
	ErrMemberNotFound    = errors.New("member not found")
	ErrAlreadyMember     = errors.New("user is already a member of this group")
	ErrNotGroupAdmin     = errors.New("user is not an admin of this group")
	ErrCannotRemoveAdmin = errors.New("cannot remove the last admin from the group")
)

type GroupService struct {
	groupRepo *repository.GroupRepository
	userRepo  *repository.UserRepo
	chatRepo  *repository.ChatRepository
}

func NewGroupService(groupRepo *repository.GroupRepository, userRepo *repository.UserRepo, chatRepo *repository.ChatRepository) *GroupService {
	return &GroupService{
		groupRepo: groupRepo,
		userRepo:  userRepo,
		chatRepo:  chatRepo,
	}
}


func (s *GroupService) CreateGroup(ctx context.Context, creatorID uuid.UUID, req *models.CreateGroupRequest) (*models.Group, error) {

	creator, err := s.userRepo.GetUserByID(ctx, creatorID)
	if err != nil {
		return nil, err
	}
	if creator == nil {
		return nil, ErrUserNotFound
	}

	
	group := &models.Group{
		ID:          uuid.Must(uuid.NewV4()),
		Name:        req.Name,
		Description: req.Description,
		CreatedBy:   creatorID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	
	if err := s.groupRepo.CreateGroup(ctx, group); err != nil {
		return nil, err
	}


	if err := s.groupRepo.AddGroupMember(ctx, group.ID, creatorID, "admin"); err != nil {
		return nil, err
	}

	for _, memberIDStr := range req.MemberIDs {
		memberID, err := uuid.FromString(memberIDStr)
		if err != nil {
			continue 
		}

		
		user, err := s.userRepo.GetUserByID(ctx, memberID)
		if err != nil || user == nil {
			continue // Skip non-existent users
		}

		
		s.groupRepo.AddGroupMember(ctx, group.ID, memberID, "member")
	}
	conversation := &models.Conversation{
		ID:        uuid.Must(uuid.NewV4()),
		Type:      "group",
		GroupID:   &group.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.chatRepo.CreateConversation(ctx, conversation); err != nil {
		return nil, err
	}

	return group, nil
}

// GetGroup retrieves a group by ID
func (s *GroupService) GetGroup(ctx context.Context, groupID uuid.UUID) (*models.Group, error) {
	return s.groupRepo.GetGroupByID(ctx, groupID)
}

// GetUserGroups retrieves all groups a user is a member of
func (s *GroupService) GetUserGroups(ctx context.Context, userID uuid.UUID) ([]*models.Group, error) {
	return s.groupRepo.GetUserGroups(ctx, userID)
}

// AddGroupMember adds a user to a group (admin only)
func (s *GroupService) AddGroupMember(ctx context.Context, adminID, groupID uuid.UUID, req *models.AddGroupMemberRequest) error {
	// Verify admin is actually an admin of the group
	isAdmin, err := s.groupRepo.IsGroupAdmin(ctx, groupID, adminID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return ErrNotGroupAdmin
	}

	// Parse user ID
	userID, err := uuid.FromString(req.UserID)
	if err != nil {
		return ErrUserNotFound
	}

	// Verify user exists
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return ErrUserNotFound
	}

	// Add member
	return s.groupRepo.AddGroupMember(ctx, groupID, userID, req.Role)
}

// RemoveGroupMember removes a user from a group (admin only)
func (s *GroupService) RemoveGroupMember(ctx context.Context, adminID, groupID uuid.UUID, req *models.RemoveGroupMemberRequest) error {
	// Verify admin is actually an admin of the group
	isAdmin, err := s.groupRepo.IsGroupAdmin(ctx, groupID, adminID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return ErrNotGroupAdmin
	}

	// Parse user ID
	userID, err := uuid.FromString(req.UserID)
	if err != nil {
		return ErrUserNotFound
	}

	// Remove member
	return s.groupRepo.RemoveGroupMember(ctx, groupID, userID)
}

// ChangeMemberRole changes a member's role (admin only)
func (s *GroupService) ChangeMemberRole(ctx context.Context, adminID, groupID uuid.UUID, req *models.ChangeMemberRoleRequest) error {
	// Verify admin is actually an admin of the group
	isAdmin, err := s.groupRepo.IsGroupAdmin(ctx, groupID, adminID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return ErrNotGroupAdmin
	}

	// Parse user ID
	userID, err := uuid.FromString(req.UserID)
	if err != nil {
		return ErrUserNotFound
	}

	// Change role
	return s.groupRepo.UpdateMemberRole(ctx, groupID, userID, req.Role)
}

// UpdateGroup updates group information (admin only)
func (s *GroupService) UpdateGroup(ctx context.Context, adminID, groupID uuid.UUID, req *models.UpdateGroupRequest) (*models.Group, error) {
	// Verify admin is actually an admin of the group
	isAdmin, err := s.groupRepo.IsGroupAdmin(ctx, groupID, adminID)
	if err != nil {
		return nil, err
	}
	if !isAdmin {
		return nil, ErrNotGroupAdmin
	}

	// Get current group
	group, err := s.groupRepo.GetGroupByID(ctx, groupID)
	if err != nil {
		return nil, err
	}

	// Update fields
	if req.Name != "" {
		group.Name = req.Name
	}
	if req.Description != "" {
		group.Description = req.Description
	}
	group.UpdatedAt = time.Now()

	// Save changes
	if err := s.groupRepo.UpdateGroup(ctx, group); err != nil {
		return nil, err
	}

	return group, nil
}

// GetGroupMembers retrieves all members of a group
func (s *GroupService) GetGroupMembers(ctx context.Context, groupID uuid.UUID) ([]*models.GroupMember, error) {
	return s.groupRepo.GetGroupMembers(ctx, groupID)
}

// IsGroupMember checks if a user is a member of a group
func (s *GroupService) IsGroupMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	return s.groupRepo.IsGroupMember(ctx, groupID, userID)
}

// IsGroupAdmin checks if a user is an admin of a group
func (s *GroupService) IsGroupAdmin(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	return s.groupRepo.IsGroupAdmin(ctx, groupID, userID)
}

// DeleteGroup deletes a group (admin only)
func (s *GroupService) DeleteGroup(ctx context.Context, adminID, groupID uuid.UUID) error {
	// Verify admin is actually an admin of the group
	isAdmin, err := s.groupRepo.IsGroupAdmin(ctx, groupID, adminID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return ErrNotGroupAdmin
	}

	// Start a database transaction to ensure data consistency
	tx := s.groupRepo.GetDB().WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete the group conversation first (this will cascade to messages and participants)
	if err := s.chatRepo.DeleteConversationByGroupID(ctx, groupID); err != nil {
		tx.Rollback()
		return err
	}

	// Delete all group members
	if err := s.groupRepo.DeleteAllGroupMembers(ctx, groupID); err != nil {
		tx.Rollback()
		return err
	}

	// Finally delete the group
	if err := s.groupRepo.DeleteGroup(ctx, groupID); err != nil {
		tx.Rollback()
		return err
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return err
	}

	return nil
}

// LeaveGroup allows a user to leave a group
func (s *GroupService) LeaveGroup(ctx context.Context, userID, groupID uuid.UUID) error {
	// Check if user is a member of the group
	isMember, err := s.groupRepo.IsGroupMember(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrMemberNotFound
	}

	// Remove the user from the group (the repository method handles admin checks)
	err = s.groupRepo.RemoveGroupMember(ctx, groupID, userID)
	if err != nil {
		return err
	}

	return nil
} 