package repository

import (
	"context"
	"errors"
	"time"

	"Chat_App/internal/models"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type ChatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) CreateConversation(ctx context.Context, conversation *models.Conversation) error {
	result := r.db.WithContext(ctx).Create(conversation)
	return result.Error
}

func (r *ChatRepository) GetConversationByID(ctx context.Context, id uuid.UUID) (*models.Conversation, error) {
	var conversation models.Conversation
	result := r.db.WithContext(ctx).
		Preload("Messages").
		Preload("Messages.Sender").
		Preload("Participants").
		Where("id = ?", id).
		First(&conversation)

	if result.Error != nil {
		return nil, result.Error
	}
	return &conversation, nil
}

func (r *ChatRepository) GetConversationByParticipants(ctx context.Context, userID1, userID2 uuid.UUID) (*models.Conversation, error) {
	var conversation models.Conversation

	// Try to find existing conversation
	result := r.db.WithContext(ctx).
		Joins("JOIN conversation_participants cp1 ON conversations.id = cp1.conversation_id").
		Joins("JOIN conversation_participants cp2 ON conversations.id = cp2.conversation_id").
		Where("cp1.user_id = ? AND cp2.user_id = ? AND conversations.type = ?", userID1, userID2, "private").
		First(&conversation)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Create new conversation
			conversation = models.Conversation{
				ID:        uuid.Must(uuid.NewV4()),
				Type:      "private",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			if err := r.CreateConversation(ctx, &conversation); err != nil {
				return nil, err
			}

			// Add participants using GORM associations
			var users []models.User
			r.db.WithContext(ctx).Where("id IN ?", []uuid.UUID{userID1, userID2}).Find(&users)
			r.db.WithContext(ctx).Model(&conversation).Association("Participants").Append(users)

			return &conversation, nil
		}
		return nil, result.Error
	}

	return &conversation, nil
}

func (r *ChatRepository) SaveMessage(ctx context.Context, message *models.Message) error {
	result := r.db.WithContext(ctx).Create(message)
	if result.Error != nil {
		return result.Error
	}

	// Update conversation's last_message_at
	r.db.WithContext(ctx).
		Model(&models.Conversation{}).
		Where("id = ?", message.ConversationID).
		Updates(map[string]interface{}{
			"last_message_at": time.Now(),
			"updated_at":      time.Now(),
		})

	return nil
}

func (r *ChatRepository) GetMessagesByConversation(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	var messages []models.Message
	result := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Preload("Sender").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&messages)

	if result.Error != nil {
		return nil, result.Error
	}

	// Convert to pointers
	var messagePtrs []*models.Message
	for i := range messages {
		messagePtrs = append(messagePtrs, &messages[i])
	}

	return messagePtrs, nil
}

func (r *ChatRepository) GetMessagesByConversationID(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]models.Message, error) {
	var messages []models.Message
	result := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Preload("Sender").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&messages)

	return messages, result.Error
}

func (r *ChatRepository) GetUserConversations(ctx context.Context, userID uuid.UUID) ([]*models.Conversation, error) {
	var conversations []models.Conversation
	result := r.db.WithContext(ctx).
		Joins("JOIN conversation_participants ON conversations.id = conversation_participants.conversation_id").
		Where("conversation_participants.user_id = ?", userID).
		Preload("Messages").
		Preload("Participants").
		Order("conversations.last_message_at DESC NULLS LAST, conversations.updated_at DESC").
		Find(&conversations)

	if result.Error != nil {
		return nil, result.Error
	}

	// Convert to pointers
	var conversationPtrs []*models.Conversation
	for i := range conversations {
		conversationPtrs = append(conversationPtrs, &conversations[i])
	}

	return conversationPtrs, nil
}

func (r *ChatRepository) GetConversationParticipants(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error) {
	var participants []models.User
	result := r.db.WithContext(ctx).
		Joins("JOIN conversation_participants ON users.id = conversation_participants.user_id").
		Where("conversation_participants.conversation_id = ?", conversationID).
		Find(&participants)

	if result.Error != nil {
		return nil, result.Error
	}

	var participantIDs []uuid.UUID
	for _, participant := range participants {
		participantIDs = append(participantIDs, participant.ID)
	}

	return participantIDs, nil
}

func (r *ChatRepository) UpdateMessageStatus(ctx context.Context, messageID uuid.UUID, status string) error {
	result := r.db.WithContext(ctx).
		Model(&models.Message{}).
		Where("id = ?", messageID).
		Update("message_status", status)

	return result.Error
}

// GetConversationByGroupID retrieves a conversation for a group
func (r *ChatRepository) GetConversationByGroupID(ctx context.Context, groupID uuid.UUID) (*models.Conversation, error) {
	var conversation models.Conversation
	result := r.db.WithContext(ctx).
		Where("group_id = ? AND type = ?", groupID, "group").
		First(&conversation)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("group conversation not found")
		}
		return nil, result.Error
	}

	return &conversation, nil
}

// DeleteConversationByGroupID deletes a conversation associated with a group
func (r *ChatRepository) DeleteConversationByGroupID(ctx context.Context, groupID uuid.UUID) error {
	// First delete all messages in the conversation
	var conversation models.Conversation
	if err := r.db.WithContext(ctx).Where("group_id = ? AND type = ?", groupID, "group").First(&conversation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // No conversation to delete
		}
		return err
	}

	// Delete messages first (due to foreign key constraints)
	if err := r.db.WithContext(ctx).Where("conversation_id = ?", conversation.ID).Delete(&models.Message{}).Error; err != nil {
		return err
	}

	// Delete conversation participants from the junction table
	if err := r.db.WithContext(ctx).Exec("DELETE FROM conversation_participants WHERE conversation_id = ?", conversation.ID).Error; err != nil {
		return err
	}

	// Finally delete the conversation
	if err := r.db.WithContext(ctx).Delete(&conversation).Error; err != nil {
		return err
	}

	return nil
}
