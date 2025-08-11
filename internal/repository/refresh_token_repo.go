package repository

import (
	"context"
	"errors"
	"time"

	"Chat_App/internal/models"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type RefreshTokenRepository struct {
	db *gorm.DB
}

func NewRefreshTokenRepository(db *gorm.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	result := r.db.WithContext(ctx).Create(token)
	return result.Error
}

func (r *RefreshTokenRepository) SaveRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	result := r.db.WithContext(ctx).Create(token)
	return result.Error
}

func (r *RefreshTokenRepository) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	var token models.RefreshToken
	result := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&token)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &token, nil
}

func (r *RefreshTokenRepository) GetRefreshTokenByEmail(ctx context.Context, email string) (*models.RefreshToken, error) {
	var token models.RefreshToken
	result := r.db.WithContext(ctx).
		Joins("JOIN users ON refresh_tokens.user_id = users.id").
		Where("users.email = ?", email).
		First(&token)
	
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &token, nil
}

func (r *RefreshTokenRepository) IsTokenExpired(ctx context.Context, tokenHash string) (bool, error) {
	var token models.RefreshToken
	result := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&token)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return true, nil // Consider non-existent tokens as expired
		}
		return false, result.Error
	}
	return time.Now().After(token.ExpiresAt), nil
}

func (r *RefreshTokenRepository) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	result := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).Delete(&models.RefreshToken{})
	return result.Error
}

func (r *RefreshTokenRepository) DeleteRefreshTokenByEmail(ctx context.Context, email string) error {
	result := r.db.WithContext(ctx).
		Joins("JOIN users ON refresh_tokens.user_id = users.id").
		Where("users.email = ?", email).
		Delete(&models.RefreshToken{})
	return result.Error
}

func (r *RefreshTokenRepository) DeleteExpiredTokens(ctx context.Context) error {
	result := r.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&models.RefreshToken{})
	return result.Error
}

func (r *RefreshTokenRepository) DeleteTokensByUserID(ctx context.Context, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&models.RefreshToken{})
	return result.Error
}
