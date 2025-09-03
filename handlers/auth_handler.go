package handlers

import (
	models "Chat_App/internal/models"
	"Chat_App/internal/services"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *services.AuthService
}

func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	response, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		// Check for specific error types and return appropriate status codes
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "email already exists") || strings.Contains(errorMsg, "firebase ID already exists") {
			c.JSON(http.StatusConflict, gin.H{"error": errorMsg})
			return
		}
		// For other errors, return 500
		c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
		return
	}
	c.JSON(http.StatusOK, response)
}

// RefreshToken handles token refresh requests
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	// Get refresh token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No authorization header"})
		return
	}

	// Extract token from "Bearer <token>" format
	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
		return
	}

	refreshToken := tokenParts[1]
	
	// Call auth service to refresh the token
	newAccessToken, err := h.authService.RefreshAccessToken(c.Request.Context(), refreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": newAccessToken,
		"token_type":   "Bearer",
	})
}

// Login handles user login requests
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	response, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "user not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": errorMsg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
		return
	}
	
	c.JSON(http.StatusOK, response)
}