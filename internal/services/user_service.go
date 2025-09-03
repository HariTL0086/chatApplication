package services

import (
	"Chat_App/internal/models"
	"Chat_App/internal/repository"
	"context"

	"github.com/gofrs/uuid"
)

type UserService struct {
	userRepo *repository.UserRepo
}

func NewUserService(userRepo *repository.UserRepo) *UserService {
	return &UserService{
		userRepo: userRepo,
	}
}

// GetUserByID gets a user by ID
func (s *UserService) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	return s.userRepo.GetUserByID(ctx, userID)
}

// GetUserByFirebaseID gets a user by Firebase ID
func (s *UserService) GetUserByFirebaseID(ctx context.Context, firebaseID string) (*models.User, error) {
	return s.userRepo.GetUserByFirebaseID(ctx, firebaseID)
}

// SearchUsersByUsername searches for users by username
func (s *UserService) SearchUsersByUsername(ctx context.Context, username string, limit int) ([]models.User, error) {
	return s.userRepo.SearchUsersByUsername(ctx, username, limit)
}

// GetAllUsers gets all users with pagination
func (s *UserService) GetAllUsers(ctx context.Context, limit, offset int) ([]models.User, error) {
	return s.userRepo.GetAllUsers(ctx, limit, offset)
}

// GetAllUsersCount gets total count of all users
func (s *UserService) GetAllUsersCount(ctx context.Context) (int64, error) {
	return s.userRepo.GetAllUsersCount(ctx)
}

// GetContactUsers gets all users except the current user (for contact list)
func (s *UserService) GetContactUsers(ctx context.Context, currentUserID uuid.UUID, limit, offset int) ([]models.User, error) {
	return s.userRepo.GetUsersExcludingCurrent(ctx, currentUserID, limit, offset)
}

// GetContactUsersCount gets total count of contact users
func (s *UserService) GetContactUsersCount(ctx context.Context, currentUserID uuid.UUID) (int64, error) {
	return s.userRepo.GetUsersExcludingCurrentCount(ctx, currentUserID)
}

// UpdateUser updates an existing user
func (s *UserService) UpdateUser(ctx context.Context, user *models.User) error {
	return s.userRepo.UpdateUser(ctx, user)
} 