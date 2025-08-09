package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"time"

	"Chat_App/internal/config"
	models "Chat_App/internal/models"
	"Chat_App/internal/repository"

	"github.com/gofrs/uuid"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo        *repository.UserRepo
	refreshTokenRepo *repository.RefreshTokenRepo
	config          *config.Config
}

func NewAuthService(userRepo *repository.UserRepo, refreshTokenRepo *repository.RefreshTokenRepo, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo:        userRepo,
		refreshTokenRepo: refreshTokenRepo,
		config:          cfg,
	}
}

// Register - creates a new user account
func (s *AuthService) Register(ctx context.Context, req *models.RegisterRequest) (*models.AuthResponse, error) {
	exists, err := s.userRepo.CheckEmailExists(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("email already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	id, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	user := &models.User{
		ID:        id,
		Username:  req.Username,
		Email:     req.Email,
		Password:  string(hashedPassword),
		CreatedAt: time.Now(),
	}

	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	// Create refresh token and save to database
	refreshToken, _, err := s.createAndSaveRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// Create access token
	accessToken, err := s.createToken(user.ID)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         *user,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, req *models.LoginRequest) (*models.AuthResponse, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	accessToken, err := s.createToken(user.ID)
	if err != nil {
		return nil, err
	}

	// Create refresh token and save to database
	refreshToken, _, err := s.createAndSaveRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         *user,
	}, nil
}

// createToken - creates access token using config TTL
func (s *AuthService) createToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"exp":     time.Now().Add(s.config.JWT.AccessTokenTTL).Unix(),
		"iat":     time.Now().Unix(),
		"type":    "access",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWT.Secret))
}

// createRefreshToken - creates refresh token using config TTL
func (s *AuthService) createRefreshToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"exp":     time.Now().Add(s.config.JWT.RefreshTokenTTL).Unix(),
		"iat":     time.Now().Unix(),
		"type":    "refresh",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWT.Secret))
}

// createAndSaveRefreshToken - creates a refresh token and saves it to database
func (s *AuthService) createAndSaveRefreshToken(ctx context.Context, userID uuid.UUID) (string, *models.RefreshToken, error) {
	// Generate refresh token
	refreshTokenString, err := s.createRefreshToken(userID)
	if err != nil {
		return "", nil, err
	}

	// Hash the token for storage
	hash := sha256.Sum256([]byte(refreshTokenString))
	tokenHash := hex.EncodeToString(hash[:])

	// Create refresh token model
	refreshTokenID, err := uuid.NewV4()
	if err != nil {
		return "", nil, err
	}

	refreshToken := &models.RefreshToken{
		ID:        refreshTokenID,
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(s.config.JWT.RefreshTokenTTL), // Use config TTL
		CreatedAt: time.Now(),
	}

	// Save to database
	if err := s.refreshTokenRepo.SaveRefreshToken(ctx, refreshToken); err != nil {
		log.Printf("Error saving refresh token: %v", err)
		return "", nil, err
	}

	log.Printf("Successfully saved refresh token for user: %s (expires: %s)", 
		userID.String(), refreshToken.ExpiresAt.Format(time.RFC3339))
	return refreshTokenString, refreshToken, nil
}

func (s *AuthService) ValidateToken(tokenString string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.config.JWT.Secret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userIDStr, ok := claims["user_id"].(string)
		if !ok {
			return uuid.Nil, errors.New("invalid token")
		}
		return uuid.FromStringOrNil(userIDStr), nil
	}
	return uuid.Nil, errors.New("invalid token")
}

// RefreshToken - refreshes access token using refresh token
func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenString string) (*models.AuthResponse, error) {
	// Hash the provided refresh token
	hash := sha256.Sum256([]byte(refreshTokenString))
	tokenHash := hex.EncodeToString(hash[:])

	// Get refresh token from database
	refreshToken, err := s.refreshTokenRepo.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if refreshToken == nil {
		return nil, errors.New("invalid refresh token")
	}

	// Check if token is expired
	expired, err := s.refreshTokenRepo.IsTokenExpired(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if expired {
		// Delete expired token
		s.refreshTokenRepo.DeleteRefreshToken(ctx, tokenHash)
		return nil, errors.New("refresh token expired")
	}

	// Get user from database
	user, err := s.userRepo.GetUserByID(ctx, refreshToken.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	// Delete old refresh token
	if err := s.refreshTokenRepo.DeleteRefreshToken(ctx, tokenHash); err != nil {
		return nil, err
	}

	// Generate new access token
	accessToken, err := s.createToken(user.ID)
	if err != nil {
		return nil, err
	}

	// Generate new refresh token
	newRefreshToken, _, err := s.createAndSaveRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		User:         *user,
	}, nil
}

// RefreshTokenByEmail - refreshes access token using email
func (s *AuthService) RefreshTokenByEmail(ctx context.Context, email string) (*models.AuthResponse, error) {
	// Get user by email first
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	// Get refresh token by email
	refreshToken, err := s.refreshTokenRepo.GetRefreshTokenByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if refreshToken == nil {
		return nil, errors.New("no refresh token found for this user")
	}

	// Check if token is expired
	expired, err := s.refreshTokenRepo.IsTokenExpired(ctx, refreshToken.TokenHash)
	if err != nil {
		return nil, err
	}
	if expired {
		// Delete expired token
		s.refreshTokenRepo.DeleteRefreshToken(ctx, refreshToken.TokenHash)
		return nil, errors.New("refresh token expired")
	}

	// Delete old refresh token
	if err := s.refreshTokenRepo.DeleteRefreshToken(ctx, refreshToken.TokenHash); err != nil {
		return nil, err
	}

	// Generate new access token
	accessToken, err := s.createToken(user.ID)
	if err != nil {
		return nil, err
	}

	// Generate new refresh token
	newRefreshToken, _, err := s.createAndSaveRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		User:         *user,
	}, nil
}

// Logout - invalidates a refresh token
func (s *AuthService) Logout(ctx context.Context, tokenHash string) error {
	return s.refreshTokenRepo.DeleteRefreshToken(ctx, tokenHash)
}

// LogoutByEmail - invalidates refresh tokens by email
func (s *AuthService) LogoutByEmail(ctx context.Context, email string) error {
	return s.refreshTokenRepo.DeleteRefreshTokenByEmail(ctx, email)
}