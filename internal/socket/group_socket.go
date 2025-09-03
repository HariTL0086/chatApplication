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
	redisService *services.RedisService
}


func NewGroupSocketHandler(groupService *services.GroupService, chatService *services.ChatService, io *socket.Server, redisService *services.RedisService) *GroupSocketHandler {
	return &GroupSocketHandler{
		groupService: groupService,
		chatService:  chatService,
		io:           io,
		redisService: redisService,
	}
}


func (gsh *GroupSocketHandler) SetupGroupHandlers(client *socket.Socket) {
	// Only real-time group chat events (Socket.IO)
	client.On("join_group", gsh.handleJoinGroup(client))
	client.On("leave_group", gsh.handleLeaveGroup(client))
	client.On("send_group_message", gsh.handleSendGroupMessage(client))
	client.On("start_group_typing", gsh.handleStartGroupTyping(client))
	client.On("stop_group_typing", gsh.handleStopGroupTyping(client))
	client.On("get_group_typing_users", gsh.handleGetGroupTypingUsers(client))
}


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

		
		roomName := createGroupRoomName(groupID)
		client.Join(socket.Room(roomName))

		
		group, err := gsh.groupService.GetGroup(context.Background(), groupID)
		if err != nil {
			log.Printf("Failed to get group: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to get group information",
			})
			return
		}

		
		conversation, err := gsh.chatService.GetConversationByGroupID(context.Background(), groupID)
		if err != nil {
			log.Printf("Failed to get group conversation: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to get group conversation",
			})
			return
		}

	
		messages, err := gsh.chatService.GetConversationMessages(context.Background(), conversation.ID, 50, 0)
		if err != nil {
			log.Printf("Failed to load group messages: %v", err)
			// Don't fail the join, just log the error
		}


		var messageList []map[string]interface{}
		for _, msg := range messages {
			messageList = append(messageList, map[string]interface{}{
				"id":               msg.ID.String(),
				"sender_id":        msg.SenderID.String(),
				"content":          msg.Content,
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

		
		roomName := createGroupRoomName(groupID)
		client.Leave(socket.Room(roomName))

		log.Printf("User %s left group room: %s (room: %s)", authClient.UserID.String(), groupID.String(), roomName)

		
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
		content, contentExists := messageData["content"].(string)
		messageType, typeExists := messageData["message_type"].(string)

		if !groupExists || !contentExists {
			client.Emit("error", map[string]interface{}{
				"error": "Group ID and content are required",
			})
			return
		}

		if !typeExists {
			messageType = "text" 
		}

		groupID, err := uuid.FromString(groupIDStr)
		if err != nil {
			client.Emit("error", map[string]interface{}{
				"error": "Invalid group ID",
			})
			return
		}

		
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


		conversation, err := gsh.chatService.GetConversationByGroupID(context.Background(), groupID)
		if err != nil {
			log.Printf("Failed to get group conversation: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to get group conversation",
			})
			return
		}

	
		dbMessage, err := gsh.chatService.SendMessage(context.Background(), authClient.UserID, conversation.ID, content, messageType)
		if err != nil {
			log.Printf("Failed to save group message to database: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to save message",
			})
			return
		}

		// Get group members
		groupMembers, err := gsh.groupService.GetGroupMembers(context.Background(), groupID)
		if err != nil {
			log.Printf("Failed to get group members: %v", err)
			// Continue with sending to online users only
		}

	
		var offlineMembers []string
		for _, member := range groupMembers {
			if member.UserID != authClient.UserID {
				isOnline, err := gsh.redisService.IsUserOnline(member.UserID.String())
				if err != nil {
					log.Printf("Failed to check online status for group member %s: %v", member.UserID.String(), err)
					// Assume offline if we can't check
					offlineMembers = append(offlineMembers, member.UserID.String())
					continue
				}

				if !isOnline {
					offlineMembers = append(offlineMembers, member.UserID.String())
				}
			}
		}

		if len(offlineMembers) > 0 {
			offlineMessage := &services.OfflineMessage{
				ID:               dbMessage.ID.String(),
				SenderID:         authClient.UserID.String(),
				SenderUsername:   authClient.Username,
				GroupID:          groupID.String(),
				Content:          content,
				MessageType:      messageType,
				Timestamp:        dbMessage.CreatedAt,
				Room:             createGroupRoomName(groupID),
				ConversationID:   conversation.ID.String(),
				IsGroupMessage:   true,
			}

			if err := gsh.redisService.StoreOfflineMessage(offlineMessage); err != nil {
				log.Printf("Failed to store offline group message: %v", err)
			} else {
				log.Printf("Stored offline group message for %d offline members", len(offlineMembers))
			}
		}

		roomName := createGroupRoomName(groupID)
		message := map[string]interface{}{
			"id":               dbMessage.ID.String(),
			"sender_id":        authClient.UserID.String(),
			"sender_username":  authClient.Username,
			"content": content,
			"message_type":     messageType,
			"timestamp":        dbMessage.CreatedAt.Unix(),
			"room":            roomName,
			"group_id":        groupID.String(),
			"conversation_id":  conversation.ID.String(),
		}

		gsh.io.To(socket.Room(roomName)).Emit("new_group_message", message)


		if err := gsh.redisService.StopUserTyping(authClient.UserID.String(), roomName); err != nil {
			log.Printf("Failed to stop typing status for user %s: %v", authClient.UserID.String(), err)
		}

		typingEvent := map[string]interface{}{
			"user_id":   authClient.UserID.String(),
			"username":  authClient.Username,
			"group_id":  groupID.String(),
			"room":      roomName,
			"is_typing": false,
		}
		client.To(socket.Room(roomName)).Emit("group_user_typing", typingEvent)

		log.Printf("Group message sent by %s to group %s (saved to DB, %d offline members)", 
			authClient.Username, groupID.String(), len(offlineMembers))
	}
}


func (gsh *GroupSocketHandler) DeliverOfflineGroupMessages(client *socket.Socket, userID uuid.UUID) {
	// Get user's groups
	groups, err := gsh.groupService.GetUserGroups(context.Background(), userID)
	if err != nil {
		log.Printf("Failed to get groups for user %s: %v", userID.String(), err)
		return
	}

	// Check for offline messages in each group
	for _, group := range groups {
		offlineMessages, err := gsh.redisService.GetGroupOfflineMessages(group.ID.String())
		if err != nil {
			log.Printf("Failed to get offline group messages for group %s: %v", group.ID.String(), err)
			continue
		}

		if len(offlineMessages) == 0 {
			continue // No offline messages for this group
		}

		// Deliver group messages
		for _, msg := range offlineMessages {
			// Send the offline group message to the user
			message := map[string]interface{}{
				"id":                msg.ID,
				"sender_id":         msg.SenderID,
				"sender_username":   msg.SenderUsername,
				"content": msg.Content,
				"message_type":      msg.MessageType,
				"timestamp":         msg.Timestamp.Unix(),
				"room":              msg.Room,
				"group_id":          msg.GroupID,
				"conversation_id":   msg.ConversationID,
				"is_offline_message": true,
			}

			client.Emit("offline_group_message", message)
			log.Printf("Delivered offline group message %s to user %s in group %s", 
				msg.ID, userID.String(), msg.GroupID)
		}

		// Clear offline messages for this group after delivery
		if err := gsh.redisService.ClearGroupOfflineMessages(group.ID.String()); err != nil {
			log.Printf("Failed to clear offline group messages for group %s: %v", group.ID.String(), err)
		} else {
			log.Printf("Cleared %d offline group messages for group %s", len(offlineMessages), group.ID.String())
		}
	}
}


func (gsh *GroupSocketHandler) handleStartGroupTyping(client *socket.Socket) func(...any) {
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

		groupIDStr, exists := typingData["group_id"].(string)
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

		roomName := createGroupRoomName(groupID)

		// Set user as typing in Redis
		err = gsh.redisService.SetUserTyping(authClient.UserID.String(), authClient.Username, roomName)
		if err != nil {
			log.Printf("Failed to set user typing: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to set typing status",
			})
			return
		}

		// Notify other users in the group that this user is typing
		typingEvent := map[string]interface{}{
			"user_id":   authClient.UserID.String(),
			"username":  authClient.Username,
			"group_id":  groupID.String(),
			"room":      roomName,
			"is_typing": true,
		}

		// Emit to all users in the group room except the sender
		client.To(socket.Room(roomName)).Emit("group_user_typing", typingEvent)

		log.Printf("User %s started typing in group %s", authClient.Username, groupID.String())
	}
}


func (gsh *GroupSocketHandler) handleStopGroupTyping(client *socket.Socket) func(...any) {
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

		groupIDStr, exists := typingData["group_id"].(string)
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

		roomName := createGroupRoomName(groupID)

		// Stop user typing in Redis
		err = gsh.redisService.StopUserTyping(authClient.UserID.String(), roomName)
		if err != nil {
			log.Printf("Failed to stop user typing: %v", err)
			client.Emit("error", map[string]interface{}{
				"error": "Failed to stop typing status",
			})
			return
		}

		// Notify other users in the group that this user stopped typing
		typingEvent := map[string]interface{}{
			"user_id":   authClient.UserID.String(),
			"username":  authClient.Username,
			"group_id":  groupID.String(),
			"room":      roomName,
			"is_typing": false,
		}

		// Emit to all users in the group room except the sender
		client.To(socket.Room(roomName)).Emit("group_user_typing", typingEvent)

		log.Printf("User %s stopped typing in group %s", authClient.Username, groupID.String())
	}
}


func (gsh *GroupSocketHandler) handleGetGroupTypingUsers(client *socket.Socket) func(...any) {
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

		roomName := createGroupRoomName(groupID)

		// Get typing users from Redis
		typingUsers, err := gsh.redisService.GetTypingUsers(roomName)
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
					"group_id":  groupID.String(),
					"room":      user.Room,
					"is_typing": user.IsTyping,
				})
			}
		}

		client.Emit("group_typing_users", map[string]interface{}{
			"group_id":     groupID.String(),
			"room":         roomName,
			"typing_users": typingUsersList,
		})

		log.Printf("Sent typing users for group %s to user %s", groupID.String(), authClient.Username)
	}
}


func createGroupRoomName(groupID uuid.UUID) string {
	return "group_" + groupID.String()
} 

