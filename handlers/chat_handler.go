package handlers

import (
	"Chat_App/internal/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
)

type ChatHandler struct {
	chatService *services.ChatService
	authService *services.AuthService
}

func NewChatHandler(chatService *services.ChatService, authService *services.AuthService) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		authService: authService,
	}
}
func (h *ChatHandler) GetUserConversations(c *gin.Context) {
	userID, err := h.getUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	conversations, err := h.chatService.GetUserConversations(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get conversations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"conversations": conversations,
	})
}


func (h *ChatHandler) GetConversationMessages(c *gin.Context) {
	
	_, err := h.getUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	conversationIDStr := c.Param("conversation_id")
	conversationID, err := uuid.FromString(conversationIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid conversation ID"})
		return
	}


	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	messages, err := h.chatService.GetConversationMessages(c.Request.Context(), conversationID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"limit":    limit,
		"offset":   offset,
	})
}

func (h *ChatHandler) StartChat(c *gin.Context) {
	// Get user ID from token
	userID, err := h.getUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	// Get recipient ID from request body
	var req struct {
		RecipientID string `json:"recipient_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	recipientID, err := uuid.FromString(req.RecipientID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid recipient ID"})
		return
	}

	conversation, err := h.chatService.StartChat(c.Request.Context(), userID, recipientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create conversation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"conversation": conversation,
	})
}


func (h *ChatHandler) getUserIDFromToken(c *gin.Context) (uuid.UUID, error) {
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