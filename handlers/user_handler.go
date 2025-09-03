package handlers

import (
	"Chat_App/internal/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
)

type UserHandler struct {
	userService *services.UserService
	authService *services.AuthService
}

func NewUserHandler(userService *services.UserService, authService *services.AuthService) *UserHandler {
	return &UserHandler{
		userService: userService,
		authService: authService,
	}
}

// GetContactUsers gets all users except the current user (for contact list)
func (h *UserHandler) GetContactUsers(c *gin.Context) {
	userID, err := h.getUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	// Parse query parameters for pagination
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Get contact users and total count
	users, err := h.userService.GetContactUsers(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get contact users"})
		return
	}

	totalCount, err := h.userService.GetContactUsersCount(c.Request.Context(), userID)
	if err != nil {
		// If we can't get the count, just return the users without it
		totalCount = int64(len(users))
	}

	c.JSON(http.StatusOK, gin.H{
		"users":    users,
		"limit":    limit,
		"offset":   offset,
		"total":    totalCount,
		"has_more": offset+limit < int(totalCount),
	})
}

// GetAllUsers gets all users with pagination (admin function)
func (h *UserHandler) GetAllUsers(c *gin.Context) {
	// Parse query parameters for pagination
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Get all users and total count
	users, err := h.userService.GetAllUsers(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get users"})
		return
	}

	totalCount, err := h.userService.GetAllUsersCount(c.Request.Context())
	if err != nil {
		// If we can't get the count, just return the users without it
		totalCount = int64(len(users))
	}

	c.JSON(http.StatusOK, gin.H{
		"users":    users,
		"limit":    limit,
		"offset":   offset,
		"total":    totalCount,
		"has_more": offset+limit < int(totalCount),
	})
}

// SearchUsers searches for users by username
func (h *UserHandler) SearchUsers(c *gin.Context) {
	username := c.Query("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username parameter is required"})
		return
	}

	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 50 {
		limit = 10
	}

	users, err := h.userService.SearchUsersByUsername(c.Request.Context(), username, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"query": username,
		"limit": limit,
	})
}

// GetUserByID gets a specific user by ID
func (h *UserHandler) GetUserByID(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.FromString(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.userService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

// GetUserByFirebaseID gets a user by Firebase ID
func (h *UserHandler) GetUserByFirebaseID(c *gin.Context) {
	firebaseID := c.Param("firebase_id")
	if firebaseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Firebase ID is required"})
		return
	}

	user, err := h.userService.GetUserByFirebaseID(c.Request.Context(), firebaseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

// getUserIDFromToken extracts user ID from JWT token
func (h *UserHandler) getUserIDFromToken(c *gin.Context) (uuid.UUID, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return uuid.Nil, gin.Error{}
	}

	// Extract token from "Bearer <token>"
	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		return uuid.Nil, gin.Error{}
	}

	token := authHeader[7:]
	return h.authService.ValidateToken(token)
} 