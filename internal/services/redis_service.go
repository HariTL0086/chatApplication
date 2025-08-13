package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/redis/go-redis/v9"
)

type RedisService struct {
	client *redis.Client
}

func (rs *RedisService) Set(cacheKey string, s string, i int) {
	panic("unimplemented")
}

func (rs *RedisService) Get(cacheKey string) (any, any) {
	panic("unimplemented")
}

// UserStatus represents user online/offline status
type UserStatus struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	Status   string    `json:"status"` // "online" or "offline"
	LastSeen time.Time `json:"last_seen"`
	SocketID string    `json:"socket_id,omitempty"`
}

func NewRedisService(redisAddr, password string, db int) (*RedisService, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	log.Println("Redis connected successfully")
	return &RedisService{client: client}, nil
}

func (rs *RedisService) SetUserOnline(userID uuid.UUID, username, socketID string) error {
	ctx := context.Background()
	key := fmt.Sprintf("user:status:%s", userID.String())

	status := UserStatus{
		UserID:   userID.String(),
		Username: username,
		Status:   "online",
		LastSeen: time.Now(),
		SocketID: socketID,
	}

	statusJSON, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal user status: %v", err)
	}

	// Set user status with expiration (24 hours)
	err = rs.client.Set(ctx, key, statusJSON, 24*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to set user status: %v", err)
	}

	// Add to online users set
	err = rs.client.SAdd(ctx, "online_users", userID.String()).Err()
	if err != nil {
		return fmt.Errorf("failed to add user to online set: %v", err)
	}

	log.Printf("User %s (%s) is now online", username, userID.String())
	return nil
}

func (rs *RedisService) SetUserOffline(userID uuid.UUID) error {
	ctx := context.Background()
	key := fmt.Sprintf("user:status:%s", userID.String())

	// Get current status to preserve username
	var currentStatus UserStatus
	statusJSON, err := rs.client.Get(ctx, key).Result()
	if err == nil {
		json.Unmarshal([]byte(statusJSON), &currentStatus)
	}

	status := UserStatus{
		UserID:   userID.String(),
		Username: currentStatus.Username,
		Status:   "offline",
		LastSeen: time.Now(),
		SocketID: "",
	}

	newStatusJSON, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal user status: %v", err)
	}

	// Set user status with expiration (24 hours)
	err = rs.client.Set(ctx, key, newStatusJSON, 24*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to set user status: %v", err)
	}

	// Remove from online users set
	err = rs.client.SRem(ctx, "online_users", userID.String()).Err()
	if err != nil {
		return fmt.Errorf("failed to remove user from online set: %v", err)
	}

	log.Printf("User %s (%s) is now offline, last seen: %s", currentStatus.Username, userID.String(), status.LastSeen.Format(time.RFC3339))
	return nil
}

func (rs *RedisService) GetUserStatus(userID string) (*UserStatus, error) {
	ctx := context.Background()
	key := fmt.Sprintf("user:status:%s", userID)

	statusJSON, err := rs.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {

			return &UserStatus{
				UserID:   userID,
				Status:   "offline",
				LastSeen: time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("failed to get user status: %v", err)
	}

	var status UserStatus
	if err := json.Unmarshal([]byte(statusJSON), &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user status: %v", err)
	}

	return &status, nil
}

func (rs *RedisService) GetOnlineUsers() ([]*UserStatus, error) {
	ctx := context.Background()

	userIDs, err := rs.client.SMembers(ctx, "online_users").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get online users: %v", err)
	}

	var onlineUsers []*UserStatus
	for _, userID := range userIDs {
		status, err := rs.GetUserStatus(userID)
		if err != nil {
			log.Printf("Failed to get status for user %s: %v", userID, err)
			continue
		}
		if status.Status == "online" {
			onlineUsers = append(onlineUsers, status)
		}
	}

	return onlineUsers, nil
}

func (rs *RedisService) GetUsersStatus(userIDs []string) (map[string]*UserStatus, error) {
	ctx := context.Background()
	statusMap := make(map[string]*UserStatus)

	// Use pipeline for better performance
	pipe := rs.client.Pipeline()
	cmds := make(map[string]*redis.StringCmd)

	for _, userID := range userIDs {
		key := fmt.Sprintf("user:status:%s", userID)
		cmds[userID] = pipe.Get(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to execute pipeline: %v", err)
	}

	for userID, cmd := range cmds {
		if cmd.Err() == redis.Nil {
			// User has no status, create default offline status
			statusMap[userID] = &UserStatus{
				UserID:   userID,
				Status:   "offline",
				LastSeen: time.Now(),
			}
		} else if cmd.Err() != nil {
			log.Printf("Failed to get status for user %s: %v", userID, cmd.Err())
			continue
		} else {
			var status UserStatus
			if err := json.Unmarshal([]byte(cmd.Val()), &status); err != nil {
				log.Printf("Failed to unmarshal status for user %s: %v", userID, err)
				continue
			}
			statusMap[userID] = &status
		}
	}

	return statusMap, nil
}

type OfflineMessage struct {
	ID               string    `json:"id"`
	SenderID         string    `json:"sender_id"`
	SenderUsername   string    `json:"sender_username"`
	RecipientID      string    `json:"recipient_id"`
	GroupID          string    `json:"group_id"`
	EncryptedContent string    `json:"encrypted_content"`
	MessageType      string    `json:"message_type"`
	Timestamp        time.Time `json:"timestamp"`
	Room             string    `json:"room"`
	ConversationID   string    `json:"conversation_id"`
	IsGroupMessage   bool      `json:"is_group_message"`
}

func (rs *RedisService) StoreOfflineMessage(message *OfflineMessage) error {
	ctx := context.Background()

	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal offline message: %v", err)
	}

	if message.IsGroupMessage {

		key := fmt.Sprintf("offline_group_messages:%s", message.GroupID)
		err = rs.client.LPush(ctx, key, messageJSON).Err()
		if err != nil {
			return fmt.Errorf("failed to store group offline message: %v", err)
		}

		rs.client.Expire(ctx, key, 7*24*time.Hour)
	} else {

		key := fmt.Sprintf("offline_private_messages:%s", message.RecipientID)
		err = rs.client.LPush(ctx, key, messageJSON).Err()
		if err != nil {
			return fmt.Errorf("failed to store private offline message: %v", err)
		}

		rs.client.Expire(ctx, key, 7*24*time.Hour)
	}

	log.Printf("Stored offline message for %s (type: %s)",
		message.RecipientID, message.MessageType)
	return nil
}

func (rs *RedisService) GetOfflineMessages(userID string) ([]*OfflineMessage, error) {
	ctx := context.Background()
	var messages []*OfflineMessage

	privateKey := fmt.Sprintf("offline_private_messages:%s", userID)
	privateMessages, err := rs.client.LRange(ctx, privateKey, 0, -1).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get private offline messages: %v", err)
	}
	for _, msgJSON := range privateMessages {
		var message OfflineMessage
		if err := json.Unmarshal([]byte(msgJSON), &message); err != nil {
			log.Printf("Failed to unmarshal private offline message: %v", err)
			continue
		}
		messages = append(messages, &message)
	}

	return messages, nil
}

func (rs *RedisService) GetGroupOfflineMessages(groupID string) ([]*OfflineMessage, error) {
	ctx := context.Background()
	var messages []*OfflineMessage

	groupKey := fmt.Sprintf("offline_group_messages:%s", groupID)
	groupMessages, err := rs.client.LRange(ctx, groupKey, 0, -1).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get group offline messages: %v", err)
	}

	for _, msgJSON := range groupMessages {
		var message OfflineMessage
		if err := json.Unmarshal([]byte(msgJSON), &message); err != nil {
			log.Printf("Failed to unmarshal group offline message: %v", err)
			continue
		}
		messages = append(messages, &message)
	}

	return messages, nil
}

func (rs *RedisService) ClearOfflineMessages(userID string) error {
	ctx := context.Background()

	privateKey := fmt.Sprintf("offline_private_messages:%s", userID)
	err := rs.client.Del(ctx, privateKey).Err()
	if err != nil {
		return fmt.Errorf("failed to clear private offline messages: %v", err)
	}

	return nil
}

func (rs *RedisService) ClearGroupOfflineMessages(groupID string) error {
	ctx := context.Background()

	groupKey := fmt.Sprintf("offline_group_messages:%s", groupID)
	err := rs.client.Del(ctx, groupKey).Err()
	if err != nil {
		return fmt.Errorf("failed to clear group offline messages: %v", err)
	}

	return nil
}

func (rs *RedisService) IsUserOnline(userID string) (bool, error) {
	ctx := context.Background()

	isMember, err := rs.client.SIsMember(ctx, "online_users", userID).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check if user is online: %v", err)
	}

	return isMember, nil
}

func (rs *RedisService) GetUserSocketID(userID string) (string, error) {
	ctx := context.Background()
	key := fmt.Sprintf("user:status:%s", userID)

	statusJSON, err := rs.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("failed to get user status: %v", err)
	}

	var status UserStatus
	if err := json.Unmarshal([]byte(statusJSON), &status); err != nil {
		return "", fmt.Errorf("failed to unmarshal user status: %v", err)
	}

	return status.SocketID, nil
}

type TypingStatus struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Room      string    `json:"room"`
	IsTyping  bool      `json:"is_typing"`
	StartedAt time.Time `json:"started_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (rs *RedisService) SetUserTyping(userID, username, room string) error {
	ctx := context.Background()

	// Create typing status
	typingStatus := TypingStatus{
		UserID:    userID,
		Username:  username,
		Room:      room,
		IsTyping:  true,
		StartedAt: time.Now(),
		ExpiresAt: time.Now().Add(10 * time.Second), // Typing status expires after 10 seconds
	}

	typingJSON, err := json.Marshal(typingStatus)
	if err != nil {
		return fmt.Errorf("failed to marshal typing status: %v", err)
	}

	// Store typing status with key: typing:{room}:{user_id}
	key := fmt.Sprintf("typing:%s:%s", room, userID)
	err = rs.client.Set(ctx, key, typingJSON, 10*time.Second).Err()
	if err != nil {
		return fmt.Errorf("failed to set typing status: %v", err)
	}

	// Add user to typing users set for this room
	setKey := fmt.Sprintf("typing_users:%s", room)
	err = rs.client.SAdd(ctx, setKey, userID).Err()
	if err != nil {
		return fmt.Errorf("failed to add user to typing set: %v", err)
	}

	// Set expiration for the typing set (cleanup after 30 seconds)
	rs.client.Expire(ctx, setKey, 30*time.Second)

	log.Printf("User %s is typing in room %s", username, room)
	return nil
}

func (rs *RedisService) StopUserTyping(userID, room string) error {
	ctx := context.Background()

	// Remove typing status
	key := fmt.Sprintf("typing:%s:%s", room, userID)
	err := rs.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to remove typing status: %v", err)
	}

	// Remove user from typing users set
	setKey := fmt.Sprintf("typing_users:%s", room)
	err = rs.client.SRem(ctx, setKey, userID).Err()
	if err != nil {
		return fmt.Errorf("failed to remove user from typing set: %v", err)
	}

	log.Printf("User %s stopped typing in room %s", userID, room)
	return nil
}

func (rs *RedisService) GetTypingUsers(room string) ([]*TypingStatus, error) {
	ctx := context.Background()
	var typingUsers []*TypingStatus

	// Get all typing user IDs for this room
	setKey := fmt.Sprintf("typing_users:%s", room)
	userIDs, err := rs.client.SMembers(ctx, setKey).Result()
	if err != nil {
		if err == redis.Nil {
			return typingUsers, nil // No typing users
		}
		return nil, fmt.Errorf("failed to get typing users: %v", err)
	}

	// Get typing status for each user
	for _, userID := range userIDs {
		key := fmt.Sprintf("typing:%s:%s", room, userID)
		typingJSON, err := rs.client.Get(ctx, key).Result()
		if err != nil {
			if err == redis.Nil {
				// User is no longer typing, remove from set
				rs.client.SRem(ctx, setKey, userID)
				continue
			}
			log.Printf("Failed to get typing status for user %s: %v", userID, err)
			continue
		}

		var typingStatus TypingStatus
		if err := json.Unmarshal([]byte(typingJSON), &typingStatus); err != nil {
			log.Printf("Failed to unmarshal typing status for user %s: %v", userID, err)
			continue
		}

		// Check if typing status has expired
		if time.Now().After(typingStatus.ExpiresAt) {
			// Remove expired typing status
			rs.client.Del(ctx, key)
			rs.client.SRem(ctx, setKey, userID)
			continue
		}

		typingUsers = append(typingUsers, &typingStatus)
	}

	return typingUsers, nil
}

func (rs *RedisService) CleanupExpiredTypingStatus() error {
	ctx := context.Background()

	// Get all typing set keys
	pattern := "typing_users:*"
	keys, err := rs.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get typing keys: %v", err)
	}

	for _, setKey := range keys {
		// Extract room name from key
		room := strings.TrimPrefix(setKey, "typing_users:")

		// Get typing users for this room
		userIDs, err := rs.client.SMembers(ctx, setKey).Result()
		if err != nil {
			continue
		}

		// Check each user's typing status
		for _, userID := range userIDs {
			key := fmt.Sprintf("typing:%s:%s", room, userID)
			typingJSON, err := rs.client.Get(ctx, key).Result()
			if err != nil {
				// Remove user from set if status doesn't exist
				rs.client.SRem(ctx, setKey, userID)
				continue
			}

			var typingStatus TypingStatus
			if err := json.Unmarshal([]byte(typingJSON), &typingStatus); err != nil {
				continue
			}

			// Remove if expired
			if time.Now().After(typingStatus.ExpiresAt) {
				rs.client.Del(ctx, key)
				rs.client.SRem(ctx, setKey, userID)
			}
		}
	}

	return nil
}

func (rs *RedisService) Close() error {
	return rs.client.Close()
}
