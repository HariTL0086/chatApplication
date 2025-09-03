package services

import (
	"Chat_App/internal/config"
	"Chat_App/internal/models"
	"Chat_App/internal/repository"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/gofrs/uuid"
	"github.com/golang-jwt/jwt/v5"
)

type AuthService struct {
	userRepo         *repository.UserRepo
	refreshTokenRepo *repository.RefreshTokenRepository
	config           *config.Config
}

func NewAuthService(userRepo *repository.UserRepo, refreshTokenRepo *repository.RefreshTokenRepository, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		config:           cfg,
	}
}

func (s *AuthService) Register(ctx context.Context, req *models.RegisterRequest) (*models.AuthResponse, error) {
	exists, err := s.userRepo.CheckEmailExists(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("email already exists")
	}

	// Check if Firebase ID already exists
	exists, err = s.userRepo.CheckFirebaseIDExists(ctx, req.FirebaseID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("firebase ID already exists")
	}

	id, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	user := &models.User{
		ID:         id,
		Username:   req.Username,
		Email:      req.Email,
		FirebaseID: req.FirebaseID,
		PhotoURL:   req.PhotoURL,
		CreatedAt:  time.Now(),
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

func (s *AuthService) createAndSaveRefreshToken(ctx context.Context, userID uuid.UUID) (string, *models.RefreshToken, error) {
	// Generate refresh token
	refreshTokenString, err := s.createRefreshToken(userID)
	if err != nil {
		return "", nil, err
	}

	// Hash the refresh token for storage
	hash := sha256.Sum256([]byte(refreshTokenString))
	tokenHash := hex.EncodeToString(hash[:])

	// Create refresh token record
	refreshTokenID, err := uuid.NewV4()
	if err != nil {
		return "", nil, err
	}

	refreshToken := &models.RefreshToken{
		ID:        refreshTokenID,
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(s.config.JWT.RefreshTokenTTL),
		CreatedAt: time.Now(),
	}

	// Save refresh token to database
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

// GetTokenExpiration extracts the expiration time from a JWT token
func (s *AuthService) GetTokenExpiration(tokenString string) (time.Time, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return time.Time{}, errors.New("unexpected signing method")
		}
		return time.Time{}, errors.New("unexpected signing method")
	})
	if err != nil {
		return time.Time{}, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		exp, ok := claims["exp"].(float64)
		if !ok {
			return time.Time{}, errors.New("invalid token expiration")
		}
		return time.Unix(int64(exp), 0), nil
	}
	return time.Time{}, errors.New("invalid token")
}

// RefreshAccessToken refreshes an access token using a valid refresh token
func (s *AuthService) RefreshAccessToken(ctx context.Context, refreshTokenString string) (string, error) {
	// Parse and validate the refresh token
	token, err := jwt.Parse(refreshTokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.config.JWT.Secret), nil
	})
	if err != nil {
		return "", errors.New("invalid refresh token")
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", errors.New("invalid refresh token")
	}

	// Check if it's a refresh token
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		return "", errors.New("invalid token type")
	}

	// Extract user ID
	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return "", errors.New("invalid token")
	}

	userID, err := uuid.FromString(userIDStr)
	if err != nil {
		return "", errors.New("invalid user ID in token")
	}

	// Hash the refresh token to check against database
	hash := sha256.Sum256([]byte(refreshTokenString))
	tokenHash := hex.EncodeToString(hash[:])

	// Check if refresh token exists and is valid in database
	refreshToken, err := s.refreshTokenRepo.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return "", errors.New("refresh token not found")
	}

	if refreshToken == nil {
		return "", errors.New("refresh token not found")
	}

	// Check if refresh token is expired
	if time.Now().After(refreshToken.ExpiresAt) {
		return "", errors.New("refresh token expired")
	}

	// Check if user ID matches
	if refreshToken.UserID != userID {
		return "", errors.New("token user mismatch")
	}

	// Generate new access token
	newAccessToken, err := s.createToken(userID)
	if err != nil {
		return "", fmt.Errorf("failed to create new access token: %w", err)
	}

	return newAccessToken, nil
}

// Login handles user login and returns new tokens
func (s *AuthService) Login(ctx context.Context, req *models.LoginRequest) (*models.AuthResponse, error) {
	// Check if user exists by Firebase ID
	user, err := s.userRepo.GetUserByFirebaseID(ctx, req.FirebaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	
	if user == nil {
		return nil, errors.New("user not found")
	}

	// Create new refresh token and save to database
	refreshToken, _, err := s.createAndSaveRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// Create new access token
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