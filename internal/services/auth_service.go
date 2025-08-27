package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
	refreshTokenRepo *repository.RefreshTokenRepository
	config          *config.Config
	cryptoService   *CryptoService
}

func NewAuthService(userRepo *repository.UserRepo, refreshTokenRepo *repository.RefreshTokenRepository, cfg *config.Config) *AuthService {
	cryptoService, _ := NewCryptoService("keys")
	return &AuthService{
		userRepo:        userRepo,
		refreshTokenRepo: refreshTokenRepo,
		config:          cfg,
		cryptoService:   cryptoService,
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

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	id, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	privateKey, publicKey, err := s.cryptoService.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	publicKeyString, err := s.cryptoService.PublicKeyToString(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to convert public key to string: %w", err)
	}

	user := &models.User{
		ID:        id,
		Username:  req.Username,
		Email:     req.Email,
		Password:  string(hashedPassword),
		PublicKey: publicKeyString,
		CreatedAt: time.Now(),
	}

	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	if err := s.cryptoService.SavePrivateKey(user.ID, privateKey); err != nil {
		log.Printf("Failed to save private key for user %s: %v", user.ID, err)
		return nil, fmt.Errorf("failed to save private key: %w", err)
	}

	
	refreshToken, _, err := s.createAndSaveRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	
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

	
	hash := sha256.Sum256([]byte(refreshTokenString))
	tokenHash := hex.EncodeToString(hash[:])

	
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
		return []byte(s.config.JWT.Secret), nil
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

func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenString string) (*models.AuthResponse, error) {
	// Hash the provided refresh token
	hash := sha256.Sum256([]byte(refreshTokenString))
	tokenHash := hex.EncodeToString(hash[:])


	refreshToken, err := s.refreshTokenRepo.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if refreshToken == nil {
		return nil, errors.New("invalid refresh token")
	}

	
	expired, err := s.refreshTokenRepo.IsTokenExpired(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if expired {
	
		s.refreshTokenRepo.DeleteRefreshToken(ctx, tokenHash)
		return nil, errors.New("refresh token expired")
	}


	user, err := s.userRepo.GetUserByID(ctx, refreshToken.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	
	if err := s.refreshTokenRepo.DeleteRefreshToken(ctx, tokenHash); err != nil {
		return nil, err
	}

	
	accessToken, err := s.createToken(user.ID)
	if err != nil {
		return nil, err
	}

	
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


func (s *AuthService) RefreshTokenByEmail(ctx context.Context, email string) (*models.AuthResponse, error) {
	
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	
	refreshToken, err := s.refreshTokenRepo.GetRefreshTokenByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if refreshToken == nil {
		return nil, errors.New("no refresh token found for this user")
	}


	expired, err := s.refreshTokenRepo.IsTokenExpired(ctx, refreshToken.TokenHash)
	if err != nil {
		return nil, err
	}
	if expired {
	
		s.refreshTokenRepo.DeleteRefreshToken(ctx, refreshToken.TokenHash)
		return nil, errors.New("refresh token expired")
	}

	
	if err := s.refreshTokenRepo.DeleteRefreshToken(ctx, refreshToken.TokenHash); err != nil {
		return nil, err
	}

	accessToken, err := s.createToken(user.ID)
	if err != nil {
		return nil, err
	}


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


func (s *AuthService) Logout(ctx context.Context, tokenHash string) error {
	return s.refreshTokenRepo.DeleteRefreshToken(ctx, tokenHash)
}


func (s *AuthService) LogoutByEmail(ctx context.Context, email string) error {
	return s.refreshTokenRepo.DeleteRefreshTokenByEmail(ctx, email)
}