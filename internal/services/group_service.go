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
func (s *GroupService) GetGroup(ctx context.Context, groupID uuid.UUID) (*models.Group, error) {
	return s.groupRepo.GetGroupByID(ctx, groupID)
}


func (s *GroupService) GetUserGroups(ctx context.Context, userID uuid.UUID) ([]*models.Group, error) {
	return s.groupRepo.GetUserGroups(ctx, userID)
}


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


func (s *GroupService) RemoveGroupMember(ctx context.Context, adminID, groupID uuid.UUID, req *models.RemoveGroupMemberRequest) error {
	
	isAdmin, err := s.groupRepo.IsGroupAdmin(ctx, groupID, adminID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return ErrNotGroupAdmin
	}


	userID, err := uuid.FromString(req.UserID)
	if err != nil {
		return ErrUserNotFound
	}

	
	return s.groupRepo.RemoveGroupMember(ctx, groupID, userID)
}


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


func (s *GroupService) UpdateGroup(ctx context.Context, adminID, groupID uuid.UUID, req *models.UpdateGroupRequest) (*models.Group, error) {
	
	isAdmin, err := s.groupRepo.IsGroupAdmin(ctx, groupID, adminID)
	if err != nil {
		return nil, err
	}
	if !isAdmin {
		return nil, ErrNotGroupAdmin
	}


	group, err := s.groupRepo.GetGroupByID(ctx, groupID)
	if err != nil {
		return nil, err
	}

	
	if req.Name != "" {
		group.Name = req.Name
	}
	if req.Description != "" {
		group.Description = req.Description
	}
	group.UpdatedAt = time.Now()

	
	if err := s.groupRepo.UpdateGroup(ctx, group); err != nil {
		return nil, err
	}

	return group, nil
}


func (s *GroupService) GetGroupMembers(ctx context.Context, groupID uuid.UUID) ([]*models.GroupMember, error) {
	return s.groupRepo.GetGroupMembers(ctx, groupID)
}


func (s *GroupService) IsGroupMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	return s.groupRepo.IsGroupMember(ctx, groupID, userID)
}

func (s *GroupService) IsGroupAdmin(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	return s.groupRepo.IsGroupAdmin(ctx, groupID, userID)
}


func (s *GroupService) DeleteGroup(ctx context.Context, adminID, groupID uuid.UUID) error {
	
	isAdmin, err := s.groupRepo.IsGroupAdmin(ctx, groupID, adminID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return ErrNotGroupAdmin
	}

	
	tx := s.groupRepo.GetDB().WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()


	if err := s.chatRepo.DeleteConversationByGroupID(ctx, groupID); err != nil {
		tx.Rollback()
		return err
	}


	if err := s.groupRepo.DeleteAllGroupMembers(ctx, groupID); err != nil {
		tx.Rollback()
		return err
	}

	if err := s.groupRepo.DeleteGroup(ctx, groupID); err != nil {
		tx.Rollback()
		return err
	}


	if err := tx.Commit().Error; err != nil {
		return err
	}

	return nil
}


func (s *GroupService) LeaveGroup(ctx context.Context, userID, groupID uuid.UUID) error {

	isMember, err := s.groupRepo.IsGroupMember(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrMemberNotFound
	}


	err = s.groupRepo.RemoveGroupMember(ctx, groupID, userID)
	if err != nil {
		return err
	}

	return nil
} 