package repository

import (
	"Chat_App/internal/models"
	"context"
	"time"

	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RefreshTokenRepo struct {
	db *pgxpool.Pool
}

func NewRefreshTokenRepo(db *pgxpool.Pool) *RefreshTokenRepo {
	return &RefreshTokenRepo{db: db}
}

// SaveRefreshToken - saves a new refresh token to database
func (r *RefreshTokenRepo) SaveRefreshToken(ctx context.Context, refreshToken *models.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	
	_, err := r.db.Exec(ctx, query, 
		refreshToken.ID, refreshToken.UserID, refreshToken.TokenHash, 
		refreshToken.ExpiresAt, refreshToken.CreatedAt)
	return err
}

// GetRefreshTokenByHash - finds refresh token by hash
func (r *RefreshTokenRepo) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, created_at 
		FROM refresh_tokens 
		WHERE token_hash = $1
	`
	
	var refreshToken models.RefreshToken
	err := r.db.QueryRow(ctx, query, tokenHash).Scan(
		&refreshToken.ID, &refreshToken.UserID, &refreshToken.TokenHash, 
		&refreshToken.ExpiresAt, &refreshToken.CreatedAt)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Token not found
		}
		return nil, err
	}
	
	return &refreshToken, nil
}

// DeleteRefreshToken - deletes a refresh token by hash
func (r *RefreshTokenRepo) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	query := `DELETE FROM refresh_tokens WHERE token_hash = $1`
	
	_, err := r.db.Exec(ctx, query, tokenHash)
	return err
}

// DeleteExpiredTokens - deletes all expired refresh tokens
func (r *RefreshTokenRepo) DeleteExpiredTokens(ctx context.Context) error {
	query := `DELETE FROM refresh_tokens WHERE expires_at < $1`
	
	_, err := r.db.Exec(ctx, query, time.Now())
	return err
}

// DeleteUserTokens - deletes all refresh tokens for a specific user
func (r *RefreshTokenRepo) DeleteUserTokens(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM refresh_tokens WHERE user_id = $1`
	
	_, err := r.db.Exec(ctx, query, userID)
	return err
}

// IsTokenExpired - checks if a refresh token is expired
func (r *RefreshTokenRepo) IsTokenExpired(ctx context.Context, tokenHash string) (bool, error) {
	query := `SELECT expires_at FROM refresh_tokens WHERE token_hash = $1`
	
	var expiresAt time.Time
	err := r.db.QueryRow(ctx, query, tokenHash).Scan(&expiresAt)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return true, nil // Token not found, consider it expired
		}
		return true, err
	}
	
	return time.Now().After(expiresAt), nil
} 

// GetRefreshTokenByEmail - finds refresh token by user email
func (r *RefreshTokenRepo) GetRefreshTokenByEmail(ctx context.Context, email string) (*models.RefreshToken, error) {
	query := `
		SELECT rt.id, rt.user_id, rt.token_hash, rt.expires_at, rt.created_at 
		FROM refresh_tokens rt
		JOIN users u ON rt.user_id = u.id
		WHERE u.email = $1
		ORDER BY rt.created_at DESC
		LIMIT 1
	`
	
	var refreshToken models.RefreshToken
	err := r.db.QueryRow(ctx, query, email).Scan(
		&refreshToken.ID, &refreshToken.UserID, &refreshToken.TokenHash, 
		&refreshToken.ExpiresAt, &refreshToken.CreatedAt)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Token not found
		}
		return nil, err
	}
	
	return &refreshToken, nil
}

// DeleteRefreshTokenByEmail - deletes refresh tokens by user email
func (r *RefreshTokenRepo) DeleteRefreshTokenByEmail(ctx context.Context, email string) error {
	query := `
		DELETE FROM refresh_tokens 
		WHERE user_id IN (SELECT id FROM users WHERE email = $1)
	`
	
	_, err := r.db.Exec(ctx, query, email)
	return err
} 