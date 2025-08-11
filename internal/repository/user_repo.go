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

// CreateUser - saves a new user to database
func (r *UserRepo) CreateUser(ctx context.Context, user *models.User) error {
	result := r.db.WithContext(ctx).Create(user)
	return result.Error
}

// GetUserByEmail - finds user by email
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

// CheckEmailExists - checks if email already exists
func (r *UserRepo) CheckEmailExists(ctx context.Context, email string) (bool, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&models.User{}).Where("email = ?", email).Count(&count)
	return count > 0, result.Error
}

// GetUserByID - finds user by ID
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