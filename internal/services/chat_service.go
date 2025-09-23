package services

import (
	"Chat_App/internal/models"
	"Chat_App/internal/repository"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofrs/uuid"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidRoomName = errors.New("invalid room name")
)

type ChatService struct {
	chatRepo *repository.ChatRepository
	userRepo *repository.UserRepo
}

// GetUserByID gets a user by ID
func (s *ChatService) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	return s.userRepo.GetUserByID(ctx, userID)
}

func NewChatService(chatRepo *repository.ChatRepository, userRepo *repository.UserRepo) *ChatService {
	return &ChatService{
		chatRepo: chatRepo,
		userRepo: userRepo,
	}
}

func (s *ChatService) StartChat(ctx context.Context, userID1, userID2 uuid.UUID) (*models.Conversation, error) {

	user1, err := s.userRepo.GetUserByID(ctx, userID1)
	if err != nil {
		return nil, err
	}
	if user1 == nil {
		return nil, ErrUserNotFound
	}

	user2, err := s.userRepo.GetUserByID(ctx, userID2)
	if err != nil {
		return nil, err
	}
	if user2 == nil {
		return nil, ErrUserNotFound
	}

	conversation, err := s.chatRepo.GetConversationByParticipants(ctx, userID1, userID2)
	if err != nil {
		return nil, err
	}

	return conversation, nil
}


func (s *ChatService) SendMessage(ctx context.Context, senderID, conversationID uuid.UUID, content, messageType string) (*models.Message, error) {

	sender, err := s.userRepo.GetUserByID(ctx, senderID)
	if err != nil {
		return nil, err
	}
	if sender == nil {
		return nil, ErrUserNotFound
	}

	message := &models.Message{
		ID:               uuid.Must(uuid.NewV4()),
		ConversationID:   conversationID,
		SenderID:         senderID,
		Content:          content, // Store as plain text now
		MessageType:      messageType,
		CreatedAt:        time.Now(),
		MessageStatus:    "sent",
	}

	if err := s.chatRepo.SaveMessage(ctx, message); err != nil {
		return nil, err
	}

	return message, nil
}

func (s *ChatService) GetConversationMessages(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	return s.chatRepo.GetMessagesByConversation(ctx, conversationID, limit, offset)
}

func (s *ChatService) GetUserConversations(ctx context.Context, userID uuid.UUID) ([]*models.Conversation, error) {

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	return s.chatRepo.GetUserConversations(ctx, userID)
}

func (s *ChatService) GetConversationParticipants(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error) {
	return s.chatRepo.GetConversationParticipants(ctx, conversationID)
}

func (s *ChatService) GetConversationByRoomName(ctx context.Context, roomName string) (*models.Conversation, error) {

	if len(roomName) < 8 || roomName[:8] != "private_" {
		return nil, ErrInvalidRoomName
	}

	parts := strings.Split(roomName[8:], "_")
	if len(parts) != 2 {
		return nil, ErrInvalidRoomName
	}

	userID1, err := uuid.FromString(parts[0])
	if err != nil {
		return nil, ErrInvalidRoomName
	}

	userID2, err := uuid.FromString(parts[1])
	if err != nil {
		return nil, ErrInvalidRoomName
	}

	return s.chatRepo.GetConversationByParticipants(ctx, userID1, userID2)
}

func (s *ChatService) GetRoomNameForConversation(ctx context.Context, conversationID uuid.UUID) (string, error) {
	// Get participants for this conversation
	participants, err := s.GetConversationParticipants(ctx, conversationID)
	if err != nil {
		return "", err
	}

	if len(participants) != 2 {
		return "", errors.New("conversation must have exactly 2 participants")
	}

	// Create room name using the same logic as createPrivateRoomName
	userID1 := participants[0]
	userID2 := participants[1]

	if userID1.String() < userID2.String() {
		return "private_" + userID1.String() + "_" + userID2.String(), nil
	}
	return "private_" + userID2.String() + "_" + userID1.String(), nil
}

func (s *ChatService) GetConversationByGroupID(ctx context.Context, groupID uuid.UUID) (*models.Conversation, error) {
	return s.chatRepo.GetConversationByGroupID(ctx, groupID)
}

func (s *ChatService) DeleteConversationByGroupID(ctx context.Context, groupID uuid.UUID) error {
	return s.chatRepo.DeleteConversationByGroupID(ctx, groupID)
}

func (s *ChatService) SendEncryptedMessage(ctx context.Context, senderID, conversationID uuid.UUID, plainMessage, messageType string) (*models.Message, error) {
	// Simplified version without encryption
	return s.SendMessage(ctx, senderID, conversationID, plainMessage, messageType)
}

func (s *ChatService) DecryptMessage(ctx context.Context, message *models.Message, userID uuid.UUID) (string, error) {
	// Return the content as-is since we're not encrypting
	return message.Content, nil
}

func (s *ChatService) GetDecryptedMessages(ctx context.Context, conversationID, userID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	messages, err := s.chatRepo.GetMessagesByConversation(ctx, conversationID, limit, offset)
	if err != nil {
		return nil, err
	}

	// No decryption needed, just return messages as-is
	return messages, nil
}

func (s *ChatService) MarkMessageAsSeen(ctx context.Context, messageID uuid.UUID) error {
	return s.chatRepo.UpdateMessageStatus(ctx, messageID, "seen")
}

func (s *ChatService) UpdateMessageStatus(ctx context.Context, messageID uuid.UUID, status string) error {
	return s.chatRepo.UpdateMessageStatus(ctx, messageID, status)
}

func (s *ChatService) GetConversationByMessageID(ctx context.Context, messageID uuid.UUID) (*models.Conversation, error) {
	return s.chatRepo.GetConversationByMessageID(ctx, messageID)
}

func (s *ChatService) GetUnseenMessages(ctx context.Context, recipientID uuid.UUID) ([]*models.Message, error) {
	// Get conversations where the user is a participant
	conversations, err := s.GetUserConversations(ctx, recipientID)
	if err != nil {
		return nil, err
	}

	var unseenMessages []*models.Message
	for _, conv := range conversations {
		// Get messages from this conversation that are sent but not seen
		messages, err := s.chatRepo.GetMessagesByConversation(ctx, conv.ID, 100, 0)
		if err != nil {
			continue
		}

		for _, message := range messages {
			// Only include messages sent by others that are not seen
			if message.SenderID != recipientID && message.MessageStatus == "sent" {
				unseenMessages = append(unseenMessages, message)
			}
		}
	}

	return unseenMessages, nil
}
