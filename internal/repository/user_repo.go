package repository

import (
	"Chat_App/internal/models"
	"context"

	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

// CreateUser - saves a new user to database
func (r *UserRepo) CreateUser(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (id, username, email, password, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	
	_, err := r.db.Exec(ctx, query, 
		user.ID, user.Username, user.Email, user.Password, user.CreatedAt)
	return err
}

// GetUserByEmail - finds user by email
func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `SELECT id, username, email, password, created_at FROM users WHERE email = $1`
	
	var user models.User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Username, &user.Email, &user.Password, &user.CreatedAt)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // User not found
		}
		return nil, err
	}
	
	return &user, nil
}

// CheckEmailExists - checks if email already exists
func (r *UserRepo) CheckEmailExists(ctx context.Context, email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
	
	var exists bool
	err := r.db.QueryRow(ctx, query, email).Scan(&exists)
	return exists, err
}

// GetUserByID - finds user by ID
func (r *UserRepo) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	query := `SELECT id, username, email, password, created_at FROM users WHERE id = $1`
	
	var user models.User
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.Username, &user.Email, &user.Password, &user.CreatedAt)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // User not found
		}
		return nil, err
	}
	
	return &user, nil
}