package socket

import (
	"Chat_App/internal/services"
	"context"
	"log"
	"time"

	"Chat_App/internal/models"

	"github.com/gofrs/uuid"
	"github.com/zishang520/socket.io/v2/socket"
)


type ChatSocketHandler struct {
	chatService  *services.ChatService
	io           *socket.Server
	redisService *services.RedisService
}


func NewChatSocketHandler(chatService *services.ChatService, io *socket.Server, redisService *services.RedisService) *ChatSocketHandler {
	return &ChatSocketHandler{
		chatService:  chatService,
		io:           io,
		redisService: redisService,
	}
}


func (csh *ChatSocketHandler) SetupChatHandlers(client *socket.Socket) {
	client.On("start_chat", csh.handleStartChat(client))
	client.On("join_conversation", csh.handleJoinConversation(client))
	client.On("send_message", csh.handleSendMessage(client))
	client.On("get_decrypted_messages", csh.handleGetDecryptedMessages(client))
	client.On("start_typing", csh.handleStartTyping(client))
	client.On("stop_typing", csh.handleStopTyping(client))
	client.On("get_typing_users", csh.handleGetTypingUsers(client))
}



func (csh *ChatSocketHandler) handleStartChat(client *socket.Socket) func(...any) {
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

	
		conversation, err := csh.chatService.StartChat(context.Background(), authClient.UserID, recipientID)
		if err != nil {
			log.Printf("Failed to create/get conversation: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to create conversation",
			})
			return
		}

		roomName := createPrivateRoomName(authClient.UserID, recipientID)

	
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

	
		conversation, err := csh.chatService.GetConversationByRoomName(context.Background(), roomName)
		if err != nil {
			log.Printf("Failed to get conversation for room %s: %v", roomName, err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to get conversation",
			})
			return
		}


		messages, err := csh.chatService.GetDecryptedMessages(context.Background(), conversation.ID, authClient.UserID, 50, 0)
		if err != nil {
			log.Printf("Failed to load messages for conversation %s: %v", conversation.ID.String(), err)
			
		}


		var messageList []map[string]interface{}
		for _, msg := range messages {
			messageList = append(messageList, map[string]interface{}{
				"id":                msg.ID.String(),
				"sender_id":         msg.SenderID.String(),
				"decrypted_content": msg.EncryptedContent, // This now contains decrypted content
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


func (csh *ChatSocketHandler) handleGetDecryptedMessages(client *socket.Socket) func(...any) {
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
				"error": "No conversation data provided",
			})
			return
		}

		conversationData, ok := data[0].(map[string]interface{})
		if !ok {
			client.Emit("error", map[string]interface{}{
				"error": "Invalid conversation data format",
			})
			return
		}

		conversationIDStr, exists := conversationData["conversation_id"].(string)
		if !exists {
			client.Emit("error", map[string]interface{}{
				"error": "Conversation ID is required",
			})
			return
		}

		conversationID, err := uuid.FromString(conversationIDStr)
		if err != nil {
			client.Emit("error", map[string]interface{}{
				"error": "Invalid conversation ID",
			})
			return
		}

		limit := 50
		offset := 0
		if limitVal, ok := conversationData["limit"].(float64); ok {
			limit = int(limitVal)
		}
		if offsetVal, ok := conversationData["offset"].(float64); ok {
			offset = int(offsetVal)
		}

		messages, err := csh.chatService.GetDecryptedMessages(context.Background(), conversationID, authClient.UserID, limit, offset)
		if err != nil {
			log.Printf("Failed to get decrypted messages: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to get messages",
			})
			return
		}

		var messageList []map[string]interface{}
		for _, msg := range messages {
			messageList = append(messageList, map[string]interface{}{
				"id":               msg.ID.String(),
				"sender_id":        msg.SenderID.String(),
				"decrypted_content": msg.EncryptedContent,
				"message_type":     msg.MessageType,
				"timestamp":        msg.CreatedAt.Unix(),
				"conversation_id":  conversationID.String(),
			})
		}

		client.Emit("decrypted_messages", map[string]interface{}{
			"conversation_id": conversationID.String(),
			"messages":        messageList,
		})
	}
}

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
		content, contentExists := messageData["content"].(string)
		messageType, typeExists := messageData["message_type"].(string)

		if !roomExists || !contentExists {
			client.Emit("error", map[string]interface{}{
				"error": "Room and content are required",
			})
			return
		}

		if !typeExists {
			messageType = "text"
		}

		
		conversation, err := csh.chatService.GetConversationByRoomName(context.Background(), roomName)
		if err != nil {
			log.Printf("Failed to get conversation for room %s: %v", roomName, err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to get conversation",
			})
			return
		}

	
		dbMessage, err := csh.chatService.SendEncryptedMessage(context.Background(), authClient.UserID, conversation.ID, content, messageType)
		if err != nil {
			log.Printf("Failed to save message to database: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to save message",
			})
			return
		}

		
		// Get participants and handle offline recipients
		participants, err := csh.chatService.GetConversationParticipants(context.Background(), conversation.ID)
		if err != nil {
			log.Printf("Failed to get conversation participants: %v", err)
			return
		}

		// Handle offline recipients and send decrypted messages to online users
		var offlineRecipients []string
		for _, participantID := range participants {
			if participantID != authClient.UserID {
				isOnline, err := csh.redisService.IsUserOnline(participantID.String())
				if err != nil {
					log.Printf("Failed to check online status for user %s: %v", participantID.String(), err)
					offlineRecipients = append(offlineRecipients, participantID.String())
					continue
				}

				if !isOnline {
					offlineRecipients = append(offlineRecipients, participantID.String())
				} else {
					// Send decrypted message to online user
					decryptedContent, err := csh.chatService.DecryptMessage(context.Background(), dbMessage, participantID)
					if err != nil {
						log.Printf("Failed to decrypt message for user %s: %v", participantID.String(), err)
						// Send encrypted message if decryption fails
						decryptedContent = dbMessage.EncryptedContent
					}

					decryptedMessage := map[string]interface{}{
						"id":                dbMessage.ID.String(),
						"sender_id":         authClient.UserID.String(),
						"sender_username":   authClient.Username,
						"decrypted_content": decryptedContent,
						"message_type":      messageType,
						"timestamp":         dbMessage.CreatedAt.Unix(),
						"room":              roomName,
						"conversation_id":   conversation.ID.String(),
						"message_status":    "sent",
					}

					// Send to specific user
					csh.io.To(socket.Room("user_"+participantID.String())).Emit("new_message", decryptedMessage)

					// Auto-mark as seen for online users after a short delay (simulating they saw it)
					go func(msgID uuid.UUID, senderID uuid.UUID) {
						time.Sleep(1 * time.Second) // Small delay to simulate reading
						if err := csh.chatService.MarkMessageAsSeen(context.Background(), msgID); err != nil {
							log.Printf("Failed to auto-mark message as seen: %v", err)
							return
						}

						// Send status update back to sender
						statusUpdate := map[string]interface{}{
							"message_id":      msgID.String(),
							"status":          "seen",
							"seen_by":         participantID.String(),
							"conversation_id": conversation.ID.String(),
						}
						csh.io.To(socket.Room("user_"+senderID.String())).Emit("message_status_updated", statusUpdate)
						log.Printf("Auto-marked message %s as seen by online user %s", msgID.String(), participantID.String())
					}(dbMessage.ID, authClient.UserID)
				}
			}
		}

		// Store offline messages
		for _, recipientID := range offlineRecipients {
			offlineMessage := &services.OfflineMessage{
				ID:               dbMessage.ID.String(),
				SenderID:         authClient.UserID.String(),
				SenderUsername:   authClient.Username,
				RecipientID:      recipientID,
				EncryptedContent: dbMessage.EncryptedContent,
				MessageType:      messageType,
				Timestamp:        dbMessage.CreatedAt,
				Room:             roomName,
				ConversationID:   conversation.ID.String(),
				IsGroupMessage:   false,
			}

			if err := csh.redisService.StoreOfflineMessage(offlineMessage); err != nil {
				log.Printf("Failed to store offline message for user %s: %v", recipientID, err)
			} else {
				log.Printf("Stored offline message for user %s", recipientID)
			}
		}

		// Send encrypted message to sender (for their own record)
		senderMessage := map[string]interface{}{
			"id":                dbMessage.ID.String(),
			"sender_id":         authClient.UserID.String(),
			"sender_username":   authClient.Username,
			"encrypted_content": dbMessage.EncryptedContent,
			"message_type":      messageType,
			"timestamp":         dbMessage.CreatedAt.Unix(),
			"room":              roomName,
			"conversation_id":   conversation.ID.String(),
		}

		// Send to sender
		csh.io.To(socket.Room("user_"+authClient.UserID.String())).Emit("new_message", senderMessage)


		if err := csh.redisService.StopUserTyping(authClient.UserID.String(), roomName); err != nil {
			log.Printf("Failed to stop typing status for user %s: %v", authClient.UserID.String(), err)
		}

	
		typingEvent := map[string]interface{}{
			"user_id":   authClient.UserID.String(),
			"username":  authClient.Username,
			"room":      roomName,
			"is_typing": false,
		}
		client.To(socket.Room(roomName)).Emit("user_typing", typingEvent)

		log.Printf("Message sent by %s to room %s (saved to DB, %d offline recipients)", 
			authClient.Username, roomName, len(offlineRecipients))
	}
}

func (csh *ChatSocketHandler) DeliverOfflineMessages(client *socket.Socket, userID uuid.UUID) {
	
	offlineMessages, err := csh.redisService.GetOfflineMessages(userID.String())
	if err != nil {
		log.Printf("Failed to get offline messages for user %s: %v", userID.String(), err)
		return
	}

	if len(offlineMessages) == 0 {
		return
	}

	var seenUpdates []map[string]interface{}
	
	// Deliver offline messages
	for _, msg := range offlineMessages {
		if !msg.IsGroupMessage {
			// Decrypt offline message for the user
			decryptedContent, err := csh.chatService.DecryptMessage(context.Background(), &models.Message{
				EncryptedContent: msg.EncryptedContent,
			}, userID)
			if err != nil {
				log.Printf("Failed to decrypt offline message %s for user %s: %v", msg.ID, userID.String(), err)
				// Send encrypted message if decryption fails
				decryptedContent = msg.EncryptedContent
			}

			// Send decrypted offline message
			message := map[string]interface{}{
				"id":                msg.ID,
				"sender_id":         msg.SenderID,
				"sender_username":   msg.SenderUsername,
				"decrypted_content": decryptedContent,
				"message_type":      msg.MessageType,
				"timestamp":         msg.Timestamp.Unix(),
				"room":              msg.Room,
				"conversation_id":   msg.ConversationID,
				"is_offline_message": true,
				"message_status":    "sent",
			}

			client.Emit("offline_message", message)
			log.Printf("Delivered decrypted offline message %s to user %s", msg.ID, userID.String())

			// Mark message as seen since user is receiving it now
			messageID, err := uuid.FromString(msg.ID)
			if err == nil {
				if err := csh.chatService.MarkMessageAsSeen(context.Background(), messageID); err != nil {
					log.Printf("Failed to mark offline message as seen: %v", err)
				} else {
					// Collect status updates to send to senders
					seenUpdates = append(seenUpdates, map[string]interface{}{
						"message_id":      msg.ID,
						"sender_id":       msg.SenderID,
						"status":          "seen",
						"seen_by":         userID.String(),
						"conversation_id": msg.ConversationID,
					})
				}
			}
		}
	}

	// Send status updates to senders who are online
	for _, update := range seenUpdates {
		senderID := update["sender_id"].(string)
		isOnline, err := csh.redisService.IsUserOnline(senderID)
		if err != nil {
			log.Printf("Failed to check if sender %s is online: %v", senderID, err)
			continue
		}

		if isOnline {
			csh.io.To(socket.Room("user_"+senderID)).Emit("message_status_updated", update)
			log.Printf("Notified sender %s that offline message was seen by %s", senderID, userID.String())
		}
	}

	if err := csh.redisService.ClearOfflineMessages(userID.String()); err != nil {
		log.Printf("Failed to clear offline messages for user %s: %v", userID.String(), err)
	} else {
		log.Printf("Cleared %d offline messages for user %s", len(offlineMessages), userID.String())
	}
}


func (csh *ChatSocketHandler) handleStartTyping(client *socket.Socket) func(...any) {
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
				"error": "No typing data provided",
			})
			return
		}

		typingData, ok := data[0].(map[string]interface{})
		if !ok {
			client.Emit("error", map[string]interface{}{
				"error": "Invalid typing data format",
			})
			return
		}

		roomName, exists := typingData["room"].(string)
		if !exists {
			client.Emit("error", map[string]interface{}{
				"error": "Room is required",
			})
			return
		}

		// Set user as typing in Redis
		err := csh.redisService.SetUserTyping(authClient.UserID.String(), authClient.Username, roomName)
		if err != nil {
			log.Printf("Failed to set user typing: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to set typing status",
			})
			return
		}

		// Notify other users in the room that this user is typing
		typingEvent := map[string]interface{}{
			"user_id":   authClient.UserID.String(),
			"username":  authClient.Username,
			"room":      roomName,
			"is_typing": true,
		}

		// Emit to all users in the room except the sender
		client.To(socket.Room(roomName)).Emit("user_typing", typingEvent)

		log.Printf("User %s started typing in room %s", authClient.Username, roomName)
	}
}


func (csh *ChatSocketHandler) handleStopTyping(client *socket.Socket) func(...any) {
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
				"error": "No typing data provided",
			})
			return
		}

		typingData, ok := data[0].(map[string]interface{})
		if !ok {
			client.Emit("error", map[string]interface{}{
				"error": "Invalid typing data format",
			})
			return
		}

		roomName, exists := typingData["room"].(string)
		if !exists {
			client.Emit("error", map[string]interface{}{
				"error": "Room is required",
			})
			return
		}

		// Stop user typing in Redis
		err := csh.redisService.StopUserTyping(authClient.UserID.String(), roomName)
		if err != nil {
			log.Printf("Failed to stop user typing: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to stop typing status",
			})
			return
		}

		// Notify other users in the room that this user stopped typing
		typingEvent := map[string]interface{}{
			"user_id":   authClient.UserID.String(),
			"username":  authClient.Username,
			"room":      roomName,
			"is_typing": false,
		}

		// Emit to all users in the room except the sender
		client.To(socket.Room(roomName)).Emit("user_typing", typingEvent)

		log.Printf("User %s stopped typing in room %s", authClient.Username, roomName)
	}
}


func (csh *ChatSocketHandler) handleGetTypingUsers(client *socket.Socket) func(...any) {
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
				"error": "Room is required",
			})
			return
		}

		// Get typing users from Redis
		typingUsers, err := csh.redisService.GetTypingUsers(roomName)
		if err != nil {
			log.Printf("Failed to get typing users: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to get typing users",
			})
			return
		}

		// Convert to map format for socket emission
		var typingUsersList []map[string]interface{}
		for _, user := range typingUsers {
			// Don't include the current user in the list
			if user.UserID != authClient.UserID.String() {
				typingUsersList = append(typingUsersList, map[string]interface{}{
					"user_id":   user.UserID,
					"username":  user.Username,
					"room":      user.Room,
					"is_typing": user.IsTyping,
				})
			}
		}

		client.Emit("typing_users", map[string]interface{}{
			"room":         roomName,
			"typing_users": typingUsersList,
		})

		log.Printf("Sent typing users for room %s to user %s", roomName, authClient.Username)
	}
}


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


func createPrivateRoomName(userID1, userID2 uuid.UUID) string {
	
	if userID1.String() < userID2.String() {
		return "private_" + userID1.String() + "_" + userID2.String()
	}
	return "private_" + userID2.String() + "_" + userID1.String()
}






