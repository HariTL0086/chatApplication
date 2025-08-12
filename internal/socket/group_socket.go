package socket

import (
	"Chat_App/internal/services"
	"context"
	"log"

	"github.com/gofrs/uuid"
	"github.com/zishang520/socket.io/v2/socket"
)


type GroupSocketHandler struct {
	groupService *services.GroupService
	chatService  *services.ChatService
	io           *socket.Server
	
}

// NewGroupSocketHandler creates a new group socket handler
func NewGroupSocketHandler(groupService *services.GroupService, chatService *services.ChatService, io *socket.Server) *GroupSocketHandler {
	return &GroupSocketHandler{
		groupService: groupService,
		chatService:  chatService,
		io:           io,
	
	}
}

// SetupGroupHandlers sets up only the essential real-time group socket event handlers
func (gsh *GroupSocketHandler) SetupGroupHandlers(client *socket.Socket) {
	// Only real-time group chat events (Socket.IO)
	client.On("join_group", gsh.handleJoinGroup(client))
	client.On("leave_group", gsh.handleLeaveGroup(client))
	client.On("send_group_message", gsh.handleSendGroupMessage(client))
	
}

// handleJoinGroup handles joining a group chat room
func (gsh *GroupSocketHandler) handleJoinGroup(client *socket.Socket) func(...any) {
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
				"error": "No group data provided",
			})
			return
		}

		groupData, ok := data[0].(map[string]interface{})
		if !ok {
			client.Emit("error", map[string]interface{}{
				"error": "Invalid group data format",
			})
			return
		}

		groupIDStr, exists := groupData["group_id"].(string)
		if !exists {
			client.Emit("error", map[string]interface{}{
				"error": "Group ID is required",
			})
			return
		}

		groupID, err := uuid.FromString(groupIDStr)
		if err != nil {
			client.Emit("error", map[string]interface{}{
				"error": "Invalid group ID",
			})
			return
		}

		// Check if user is a member of the group
		isMember, err := gsh.groupService.IsGroupMember(context.Background(), groupID, authClient.UserID)
		if err != nil {
			log.Printf("Failed to check group membership: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to verify group membership",
			})
			return
		}

		if !isMember {
			client.Emit("error", map[string]interface{}{
				"error": "You are not a member of this group",
			})
			return
		}

		// Join the group room
		roomName := createGroupRoomName(groupID)
		client.Join(socket.Room(roomName))

		// Get group information
		group, err := gsh.groupService.GetGroup(context.Background(), groupID)
		if err != nil {
			log.Printf("Failed to get group: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to get group information",
			})
			return
		}

		// Get group conversation
		conversation, err := gsh.chatService.GetConversationByGroupID(context.Background(), groupID)
		if err != nil {
			log.Printf("Failed to get group conversation: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to get group conversation",
			})
			return
		}

		// Load recent messages (last 50 messages)
		messages, err := gsh.chatService.GetConversationMessages(context.Background(), conversation.ID, 50, 0)
		if err != nil {
			log.Printf("Failed to load group messages: %v", err)
			// Don't fail the join, just log the error
		}

		// Convert messages to map format for socket emission
		var messageList []map[string]interface{}
		for _, msg := range messages {
			messageList = append(messageList, map[string]interface{}{
				"id":               msg.ID.String(),
				"sender_id":        msg.SenderID.String(),
				"encrypted_content": msg.EncryptedContent,
				"message_type":     msg.MessageType,
				"timestamp":        msg.CreatedAt.Unix(),
				"room":            roomName,
				"conversation_id":  conversation.ID.String(),
			})
		}

		log.Printf("User %s joined group: %s (room: %s)", authClient.UserID.String(), group.Name, roomName)

		client.Emit("joined_group", map[string]interface{}{
			"room":            roomName,
			"group_id":        groupID.String(),
			"group_name":      group.Name,
			"conversation_id": conversation.ID.String(),
			"messages":        messageList,
		})
	}
}

// handleLeaveGroup handles leaving a group chat room
func (gsh *GroupSocketHandler) handleLeaveGroup(client *socket.Socket) func(...any) {
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
				"error": "No group data provided",
			})
			return
		}

		groupData, ok := data[0].(map[string]interface{})
		if !ok {
			client.Emit("error", map[string]interface{}{
				"error": "Invalid group data format",
			})
			return
		}

		groupIDStr, exists := groupData["group_id"].(string)
		if !exists {
			client.Emit("error", map[string]interface{}{
				"error": "Group ID is required",
			})
			return
		}

		groupID, err := uuid.FromString(groupIDStr)
		if err != nil {
			client.Emit("error", map[string]interface{}{
				"error": "Invalid group ID",
			})
			return
		}

		// Leave the group room (Socket.IO only - no database changes)
		roomName := createGroupRoomName(groupID)
		client.Leave(socket.Room(roomName))

		log.Printf("User %s left group room: %s (room: %s)", authClient.UserID.String(), groupID.String(), roomName)

		// Notify other group members about the user leaving the room
		gsh.io.To(socket.Room(roomName)).Emit("member_left_group_room", map[string]interface{}{
			"group_id": groupID.String(),
			"user_id":  authClient.UserID.String(),
			"username": authClient.Username,
		})

		client.Emit("left_group_room", map[string]interface{}{
			"room":     roomName,
			"group_id": groupID.String(),
		})
	}
}

// handleSendGroupMessage handles sending a message in a group
func (gsh *GroupSocketHandler) handleSendGroupMessage(client *socket.Socket) func(...any) {
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

		groupIDStr, groupExists := messageData["group_id"].(string)
		encryptedContent, contentExists := messageData["encrypted_content"].(string)
		messageType, typeExists := messageData["message_type"].(string)

		if !groupExists || !contentExists {
			client.Emit("error", map[string]interface{}{
				"error": "Group ID and encrypted_content are required",
			})
			return
		}

		if !typeExists {
			messageType = "text" // default message type
		}

		groupID, err := uuid.FromString(groupIDStr)
		if err != nil {
			client.Emit("error", map[string]interface{}{
				"error": "Invalid group ID",
			})
			return
		}

		// Check if user is a member of the group
		isMember, err := gsh.groupService.IsGroupMember(context.Background(), groupID, authClient.UserID)
		if err != nil {
			log.Printf("Failed to check group membership: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to verify group membership",
			})
			return
		}

		if !isMember {
			client.Emit("error", map[string]interface{}{
				"error": "You are not a member of this group",
			})
			return
		}

		// Get group conversation
		conversation, err := gsh.chatService.GetConversationByGroupID(context.Background(), groupID)
		if err != nil {
			log.Printf("Failed to get group conversation: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to get group conversation",
			})
			return
		}

		// Save message to database
		dbMessage, err := gsh.chatService.SendMessage(context.Background(), authClient.UserID, conversation.ID, encryptedContent, messageType)
		if err != nil {
			log.Printf("Failed to save group message to database: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to save message",
			})
			return
		}

		// Create message object for socket emission
		roomName := createGroupRoomName(groupID)
		message := map[string]interface{}{
			"id":               dbMessage.ID.String(),
			"sender_id":        authClient.UserID.String(),
			"sender_username":  authClient.Username,
			"encrypted_content": encryptedContent,
			"message_type":     messageType,
			"timestamp":        dbMessage.CreatedAt.Unix(),
			"room":            roomName,
			"group_id":        groupID.String(),
			"conversation_id":  conversation.ID.String(),
		}

		// Send message to all users in the group room
		gsh.io.To(socket.Room(roomName)).Emit("new_group_message", message)

		log.Printf("Group message sent by %s to group %s (saved to DB)", authClient.Username, groupID.String())
	}
}

// createGroupRoomName creates a room name for group conversations
func createGroupRoomName(groupID uuid.UUID) string {
	return "group_" + groupID.String()
} 

