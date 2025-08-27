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
	ErrUserNotFound     = errors.New("user not found")
	ErrInvalidRoomName  = errors.New("invalid room name")
)

type ChatService struct {
	chatRepo      *repository.ChatRepository
	userRepo      *repository.UserRepo
	cryptoService *CryptoService
}

func NewChatService(chatRepo *repository.ChatRepository, userRepo *repository.UserRepo) *ChatService {
	cryptoService, _ := NewCryptoService("keys")
	return &ChatService{
		chatRepo:      chatRepo,
		userRepo:      userRepo,
		cryptoService: cryptoService,
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


func (s *ChatService) SendMessage(ctx context.Context, senderID, conversationID uuid.UUID, encryptedContent, messageType string) (*models.Message, error) {
	
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
		EncryptedContent: encryptedContent,
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


func (s *ChatService) GetConversationByGroupID(ctx context.Context, groupID uuid.UUID) (*models.Conversation, error) {
	return s.chatRepo.GetConversationByGroupID(ctx, groupID)
}


func (s *ChatService) DeleteConversationByGroupID(ctx context.Context, groupID uuid.UUID) error {
	return s.chatRepo.DeleteConversationByGroupID(ctx, groupID)
}

func (s *ChatService) SendEncryptedMessage(ctx context.Context, senderID, conversationID uuid.UUID, plainMessage, messageType string) (*models.Message, error) {
	sender, err := s.userRepo.GetUserByID(ctx, senderID)
	if err != nil {
		return nil, err
	}
	if sender == nil {
		return nil, ErrUserNotFound
	}

	participants, err := s.GetConversationParticipants(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	var recipientID uuid.UUID
	for _, participantID := range participants {
		if participantID != senderID {
			recipientID = participantID
			break
		}
	}

	if recipientID == uuid.Nil {
		return nil, errors.New("no recipient found in conversation")
	}

	recipient, err := s.userRepo.GetUserByID(ctx, recipientID)
	if err != nil {
		return nil, err
	}
	if recipient == nil {
		return nil, errors.New("recipient not found")
	}

	encryptedContent, err := s.cryptoService.EncryptWithPublicKeyString(plainMessage, recipient.PublicKey)
	if err != nil {
		return nil, errors.New("failed to encrypt message: " + err.Error())
	}

	message := &models.Message{
		ID:               uuid.Must(uuid.NewV4()),
		ConversationID:   conversationID,
		SenderID:         senderID,
		EncryptedContent: encryptedContent,
		MessageType:      messageType,
		CreatedAt:        time.Now(),
		MessageStatus:    "sent",
	}

	if err := s.chatRepo.SaveMessage(ctx, message); err != nil {
		return nil, err
	}

	return message, nil
}

func (s *ChatService) DecryptMessage(ctx context.Context, message *models.Message, userID uuid.UUID) (string, error) {
	decryptedContent, err := s.cryptoService.DecryptWithUserPrivateKey(message.EncryptedContent, userID)
	if err != nil {
		return "", errors.New("failed to decrypt message: " + err.Error())
	}

	return decryptedContent, nil
}

func (s *ChatService) GetDecryptedMessages(ctx context.Context, conversationID, userID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	messages, err := s.chatRepo.GetMessagesByConversation(ctx, conversationID, limit, offset)
	if err != nil {
		return nil, err
	}

	for _, message := range messages {
		if message.SenderID != userID {
			decryptedContent, err := s.cryptoService.DecryptWithUserPrivateKey(message.EncryptedContent, userID)
			if err == nil {
				message.EncryptedContent = decryptedContent
			}
		}
	}

	return messages, nil
}

func (s *ChatService) MarkMessageAsSeen(ctx context.Context, messageID uuid.UUID) error {
	return s.chatRepo.UpdateMessageStatus(ctx, messageID, "seen")
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