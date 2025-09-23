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

	userID, err := h.getUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	
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

func (h *ChatHandler) GetChatHistory(c *gin.Context) {
	userID, err := h.getUserIDFromToken(c)
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

	// Get pagination parameters
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

	// Get decrypted messages for the user
	messages, err := h.chatService.GetDecryptedMessages(c.Request.Context(), conversationID, userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get chat history"})
		return
	}

	// Format messages for response
	var messageList []gin.H
	for _, msg := range messages {
		messageList = append(messageList, gin.H{
			"id":               msg.ID.String(),
			"sender_id":        msg.SenderID.String(),
			"sender_name":      msg.Sender.Username, // Access sender name through the Sender relationship
			"content":          msg.Content, // This now contains the actual message content
			"message_type":     msg.MessageType,
			"message_status":   msg.MessageStatus,
			"timestamp":        msg.CreatedAt,
			"conversation_id":  conversationID.String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"conversation_id": conversationID.String(),
		"messages":        messageList,
		"limit":           limit,
		"offset":          offset,
		"total":           len(messageList),
	})
}

func (h *ChatHandler) GetChatHistoryWithUser(c *gin.Context) {
	userID, err := h.getUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	otherUserIDStr := c.Param("user_id")
	otherUserID, err := uuid.FromString(otherUserIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get or create conversation between these two users
	conversation, err := h.chatService.StartChat(c.Request.Context(), userID, otherUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get conversation"})
		return
	}

	// Get pagination parameters
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

	// Get decrypted messages for the user
	messages, err := h.chatService.GetDecryptedMessages(c.Request.Context(), conversation.ID, userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get chat history"})
		return
	}

	// Format messages for response
	var messageList []gin.H
	for _, msg := range messages {
		messageList = append(messageList, gin.H{
			"id":               msg.ID.String(),
			"sender_id":        msg.SenderID.String(),
			"content":          msg.Content, // This now contains the actual message content
			"message_type":     msg.MessageType,
			"message_status":   msg.MessageStatus,
			"timestamp":        msg.CreatedAt,
			"conversation_id":  conversation.ID.String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"conversation_id": conversation.ID.String(),
		"conversation_type": "private",
		"other_user_id":   otherUserID.String(),
		"messages":        messageList,
		"limit":           limit,
		"offset":          offset,
		"total":           len(messageList),
	})
}

func (h *ChatHandler) GetGroupChatHistory(c *gin.Context) {
	_, err := h.getUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	groupIDStr := c.Param("group_id")
	groupID, err := uuid.FromString(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	// Get conversation for the group
	conversation, err := h.chatService.GetConversationByGroupID(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get group conversation"})
		return
	}

	// Get pagination parameters
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

	// Get messages for the group conversation
	messages, err := h.chatService.GetConversationMessages(c.Request.Context(), conversation.ID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get group chat history"})
		return
	}

	// Format messages for response
	var messageList []gin.H
	for _, msg := range messages {
		messageList = append(messageList, gin.H{
			"id":               msg.ID.String(),
			"sender_id":        msg.SenderID.String(),
			"sender_name":      msg.Sender.Username, // Access sender name through the Sender relationship
			"content":          msg.Content, // This contains the message content
			"message_type":     msg.MessageType,
			"message_status":   msg.MessageStatus,
			"timestamp":        msg.CreatedAt, // Convert to Unix timestamp for consistency
			"conversation_id":  conversation.ID.String(),
			"group_id":         groupID.String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"conversation_id": conversation.ID.String(),
		"conversation_type": "group",
		"group_id":        groupID.String(),
		"messages":        messageList,
		"limit":           limit,
		"offset":          offset,
		"total":           len(messageList),
	})
}


func (h *ChatHandler) UpdateMessageStatus(c *gin.Context) {
	_, err := h.getUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	var req struct {
		MessageID string `json:"message_id" binding:"required"`
		Status    string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate status values
	validStatuses := map[string]bool{
		"sent":      true,
		"delivered": true,
		"seen":      true,
	}
	if !validStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status. Must be 'sent', 'delivered', or 'seen'"})
		return
	}

	messageID, err := uuid.FromString(req.MessageID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}

	// Update message status
	err = h.chatService.UpdateMessageStatus(c.Request.Context(), messageID, req.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update message status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message_id": messageID.String(),
		"status":     req.Status,
		"success":    true,
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