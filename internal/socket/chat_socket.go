package socket

import (
	"Chat_App/internal/services"
	"context"
	"log"

	"github.com/gofrs/uuid"
	"github.com/zishang520/socket.io/v2/socket"
)

// ChatSocketHandler handles chat-related socket events
type ChatSocketHandler struct {
	chatService  *services.ChatService
	io           *socket.Server

}

// NewChatSocketHandler creates a new chat socket handler
func NewChatSocketHandler(chatService *services.ChatService, io *socket.Server) *ChatSocketHandler {
	return &ChatSocketHandler{
		chatService:  chatService,
		io:           io,
		
	}
}

// SetupChatHandlers sets up all chat-related socket event handlers
func (csh *ChatSocketHandler) SetupChatHandlers(client *socket.Socket) {
	client.On("start_chat", csh.handleStartChat(client))
	client.On("join_conversation", csh.handleJoinConversation(client))
	client.On("send_message", csh.handleSendMessage(client))

}


// handleStartChat handles starting a new chat conversation
func (csh *ChatSocketHandler) handleStartChat(client *socket.Socket) func(...any) {
	return func(data ...any) {
		authClient, authenticated := getAuthenticatedClient(client.Id())
		if !authenticated {
			client.Emit("error", map[string]interface{}{
				"error": "Please authenticate first",
			})
			return
		}

		// Parse start chat data
		if len(data) == 0 {
			client.Emit("error", map[string]interface{}{
				"error": "No chat data provided",
			})
			return
		}

		chatData, ok := data[0].(map[string]interface{})
		if !ok {
			client.Emit("error", map[string]interface{}{
				"error": "Invalid chat data format",
			})
			return
		}

		recipientIDStr, exists := chatData["recipient_id"].(string)
		if !exists {
			client.Emit("error", map[string]interface{}{
				"error": "Recipient ID is required",
			})
			return
		}

		recipientID, err := uuid.FromString(recipientIDStr)
		if err != nil {
			client.Emit("error", map[string]interface{}{
				"error": "Invalid recipient ID",
			})
			return
		}

		// Create or get conversation in database
		conversation, err := csh.chatService.StartChat(context.Background(), authClient.UserID, recipientID)
		if err != nil {
			log.Printf("Failed to create/get conversation: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to create conversation",
			})
			return
		}

		// Create conversation room name (consistent for both users)
		roomName := createPrivateRoomName(authClient.UserID, recipientID)

		// Join the room
		client.Join(socket.Room(roomName))

		log.Printf("User %s started chat with %s (room: %s, conversation: %s)",
			authClient.UserID.String(), recipientID.String(), roomName, conversation.ID.String())

		client.Emit("chat_started", map[string]interface{}{
			"room":            roomName,
			"conversation_id": conversation.ID.String(),
			"recipient_id":    recipientID.String(),
		})
	}
}

// handleJoinConversation handles joining a conversation room
func (csh *ChatSocketHandler) handleJoinConversation(client *socket.Socket) func(...any) {
	return func(data ...any) {
		authClient, authenticated := getAuthenticatedClient(client.Id())
		if !authenticated {
			client.Emit("error", map[string]interface{}{
				"error": "Please authenticate first",
			})
			return
		}

		if len(data) == 0 {
			client.Emit("error", map[string]interface{}{
				"error": "No room data provided",
			})
			return
		}

		roomData, ok := data[0].(map[string]interface{})
		if !ok {
			client.Emit("error", map[string]interface{}{
				"error": "Invalid room data format",
			})
			return
		}

		roomName, exists := roomData["room"].(string)
		if !exists {
			client.Emit("error", map[string]interface{}{
				"error": "Room name is required",
			})
			return
		}

		client.Join(socket.Room(roomName))
		log.Printf("User %s joined room: %s", authClient.UserID.String(), roomName)

		// Get conversation from room name
		conversation, err := csh.chatService.GetConversationByRoomName(context.Background(), roomName)
		if err != nil {
			log.Printf("Failed to get conversation for room %s: %v", roomName, err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to get conversation",
			})
			return
		}

		// Load recent messages (last 50 messages)
		messages, err := csh.chatService.GetConversationMessages(context.Background(), conversation.ID, 50, 0)
		if err != nil {
			log.Printf("Failed to load messages for conversation %s: %v", conversation.ID.String(), err)
			// Don't fail the join, just log the error
		}

		// Convert messages to map format for socket emission
		var messageList []map[string]interface{}
		for _, msg := range messages {
			messageList = append(messageList, map[string]interface{}{
				"id":                msg.ID.String(),
				"sender_id":         msg.SenderID.String(),
				"encrypted_content": msg.EncryptedContent,
				"message_type":      msg.MessageType,
				"timestamp":         msg.CreatedAt.Unix(),
				"room":              roomName,
				"conversation_id":   conversation.ID.String(),
			})
		}

		client.Emit("joined_conversation", map[string]interface{}{
			"room":            roomName,
			"conversation_id": conversation.ID.String(),
			"messages":        messageList,
		})
	}
}

// handleSendMessage handles sending a message in a conversation
func (csh *ChatSocketHandler) handleSendMessage(client *socket.Socket) func(...any) {
	return func(data ...any) {
		authClient, authenticated := getAuthenticatedClient(client.Id())
		if !authenticated {
			client.Emit("error", map[string]interface{}{
				"error": "Please authenticate first",
			})
			return
		}

		if len(data) == 0 {
			client.Emit("error", map[string]interface{}{
				"error": "No message data provided",
			})
			return
		}

		messageData, ok := data[0].(map[string]interface{})
		if !ok {
			client.Emit("error", map[string]interface{}{
				"error": "Invalid message data format",
			})
			return
		}

		roomName, roomExists := messageData["room"].(string)
		encryptedContent, contentExists := messageData["encrypted_content"].(string)
		messageType, typeExists := messageData["message_type"].(string)

		if !roomExists || !contentExists {
			client.Emit("error", map[string]interface{}{
				"error": "Room and encrypted_content are required",
			})
			return
		}

		if !typeExists {
			messageType = "text" // default message type
		}

		// Get conversation from room name
		conversation, err := csh.chatService.GetConversationByRoomName(context.Background(), roomName)
		if err != nil {
			log.Printf("Failed to get conversation for room %s: %v", roomName, err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to get conversation",
			})
			return
		}

		// Save message to database
		dbMessage, err := csh.chatService.SendMessage(context.Background(), authClient.UserID, conversation.ID, encryptedContent, messageType)
		if err != nil {
			log.Printf("Failed to save message to database: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to save message",
			})
			return
		}

		// Create message object for socket emission
		message := map[string]interface{}{
			"id":                dbMessage.ID.String(),
			"sender_id":         authClient.UserID.String(),
			"sender_username":   authClient.Username,
			"encrypted_content": encryptedContent,
			"message_type":      messageType,
			"timestamp":         dbMessage.CreatedAt.Unix(),
			"room":              roomName,
			"conversation_id":   conversation.ID.String(),
		}

		// Send message to all users in the room
		csh.io.To(socket.Room(roomName)).Emit("new_message", message)

		log.Printf("Message sent by %s to room %s (saved to DB)", authClient.Username, roomName)
	}
}

// autoJoinUserToConversations automatically joins a user to their existing conversations
func (csh *ChatSocketHandler) autoJoinUserToConversations(client *socket.Socket, userID uuid.UUID) {
	conversations, err := csh.chatService.GetUserConversations(context.Background(), userID)
	if err != nil {
		log.Printf("Failed to get conversations for user %s: %v", userID.String(), err)
		return
	}

	// Join user to all their conversation rooms
	for _, conversation := range conversations {
		// For each conversation, find the other participant to create room name
		participants, err := csh.chatService.GetConversationParticipants(context.Background(), conversation.ID)
		if err != nil {
			log.Printf("Failed to get participants for conversation %s: %v", conversation.ID.String(), err)
			continue
		}

		// Find the other participant (not the current user)
		var otherUserID uuid.UUID
		for _, participantID := range participants {
			if participantID != userID {
				otherUserID = participantID
				break
			}
		}

		if otherUserID != uuid.Nil {
			roomName := createPrivateRoomName(userID, otherUserID)
			client.Join(socket.Room(roomName))
			log.Printf("Auto-joined user %s to room %s for conversation %s",
				userID.String(), roomName, conversation.ID.String())
		}
	}
}

// createPrivateRoomName creates a consistent room name for private conversations
func createPrivateRoomName(userID1, userID2 uuid.UUID) string {
	// Create consistent room name regardless of user order
	if userID1.String() < userID2.String() {
		return "private_" + userID1.String() + "_" + userID2.String()
	}
	return "private_" + userID2.String() + "_" + userID1.String()
}



