package repository

import (
	"Chat_App/internal/models"
	"context"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}


func (r *UserRepo) CreateUser(ctx context.Context, user *models.User) error {
	result := r.db.WithContext(ctx).Create(user)
	return result.Error
}


func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	result := r.db.WithContext(ctx).Where("email = ?", email).First(&user)
	
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil // User not found
		}
		return nil, result.Error
	}
	
	return &user, nil
}


func (r *UserRepo) CheckEmailExists(ctx context.Context, email string) (bool, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&models.User{}).Where("email = ?", email).Count(&count)
	return count > 0, result.Error
}

func (r *UserRepo) CheckFirebaseIDExists(ctx context.Context, firebaseID string) (bool, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&models.User{}).Where("firebase_id = ?", firebaseID).Count(&count)
	return count > 0, result.Error
}

func (r *UserRepo) GetUserByFirebaseID(ctx context.Context, firebaseID string) (*models.User, error) {
	var user models.User
	result := r.db.WithContext(ctx).Where("firebase_id = ?", firebaseID).First(&user)
	
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil // User not found
		}
		return nil, result.Error
	}
	
	return &user, nil
}


func (r *UserRepo) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	var user models.User
	result := r.db.WithContext(ctx).Where("id = ?", userID).First(&user)
	
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil // User not found
		}
		return nil, result.Error
	}
	
	return &user, nil
}

// UpdateUser updates an existing user
func (r *UserRepo) UpdateUser(ctx context.Context, user *models.User) error {
	result := r.db.WithContext(ctx).Save(user)
	return result.Error
}

// SearchUsersByUsername searches for users by username with partial matching
func (r *UserRepo) SearchUsersByUsername(ctx context.Context, username string, limit int) ([]models.User, error) {
	var users []models.User
	result := r.db.WithContext(ctx).
		Where("username ILIKE ?", "%"+username+"%").
		Limit(limit).
		Find(&users)
	
	if result.Error != nil {
		return nil, result.Error
	}
	
	return users, nil
}

// GetAllUsers gets all users with pagination
func (r *UserRepo) GetAllUsers(ctx context.Context, limit, offset int) ([]models.User, error) {
	var users []models.User
	result := r.db.WithContext(ctx).
		Limit(limit).
		Offset(offset).
		Order("username ASC").
		Find(&users)
	
	if result.Error != nil {
		return nil, result.Error
	}
	
	return users, nil
}

// GetAllUsersCount gets total count of all users
func (r *UserRepo) GetAllUsersCount(ctx context.Context) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&models.User{}).Count(&count)
	return count, result.Error
}

// GetUsersExcludingCurrent gets all users except the current user
func (r *UserRepo) GetUsersExcludingCurrent(ctx context.Context, currentUserID uuid.UUID, limit, offset int) ([]models.User, error) {
	var users []models.User
	result := r.db.WithContext(ctx).
		Where("id != ?", currentUserID).
		Limit(limit).
		Offset(offset).
		Order("username ASC").
		Find(&users)
	
	if result.Error != nil {
		return nil, result.Error
	}
	
	return users, nil
}

// GetUsersExcludingCurrentCount gets total count of users excluding current user
func (r *UserRepo) GetUsersExcludingCurrentCount(ctx context.Context, currentUserID uuid.UUID) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&models.User{}).Where("id != ?", currentUserID).Count(&count)
	return count, result.Error
}