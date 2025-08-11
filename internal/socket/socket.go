package socket

import (
	"Chat_App/internal/services"
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/zishang520/engine.io/v2/types"
	"github.com/zishang520/socket.io/v2/socket"
)

type AuthenticatedClient struct {
    UserID   uuid.UUID
    Username string
    SocketID socket.SocketId
}

var authenticatedClients sync.Map // map[socket.SocketId]*AuthenticatedClient

type SocketManager struct {
    io          *socket.Server
    authService *services.AuthService
    chatService *services.ChatService
}

func NewSocketManager(authService *services.AuthService, chatService *services.ChatService) *SocketManager {
    httpServer := types.CreateServer(nil)
    options := socket.DefaultServerOptions()

    options.SetCors(&types.Cors{
        Origin:  "*",
        Methods: []string{"GET", "POST", "OPTIONS"},
        Headers: []string{"Content-Type", "Authorization"},
        Credentials: true,
    })

    options.SetPingInterval(25 * time.Second)
    options.SetPingTimeout(20 * time.Second)

    // Configure transports - both websocket and polling
    transports := types.NewSet[string]()
    transports.Add("websocket")
    transports.Add("polling")
    options.SetTransports(transports)

    io := socket.NewServer(httpServer, options)

    sm := &SocketManager{
        io:          io,
        authService: authService,
        chatService: chatService,
    }

    sm.setupEventHandlers()
    return sm
}

func (sm *SocketManager) setupEventHandlers() {
    sm.io.On("connection", func(clients ...any) {
        client := clients[0].(*socket.Socket)
        log.Printf("Client connected: %s", client.Id())

        // Handle authentication
        client.On("authenticate", sm.handleAuthentication(client))
        
        // Handle chat events (only after authentication)
        client.On("join_conversation", sm.handleJoinConversation(client))
        client.On("send_message", sm.handleSendMessage(client))
        client.On("start_chat", sm.handleStartChat(client))

        client.On("disconnect", func(...any) {
            log.Printf("Client disconnected: %s", client.Id())
            authenticatedClients.Delete(client.Id())
        })



        client.Emit("auth_required", map[string]interface{}{
            "message": "Please authenticate to use chat features",
        })
    })
}

func (sm *SocketManager) handleAuthentication(client *socket.Socket) func(...any) {
    return func(data ...any) {
        log.Printf("Authentication attempt from client %s with %d data items", client.Id(), len(data))
        
        var ackCallback func(...any)
        var authDataInput any
        
        if len(data) > 0 {
            if callback, ok := data[len(data)-1].(func(...any)); ok {
                ackCallback = callback
                authDataInput = data[0] 
            } else {
                authDataInput = data[0] 
            }
        }
        
        if authDataInput == nil {
            log.Printf("No authentication data provided for client %s", client.Id())
            errorResponse := map[string]interface{}{
                "error": "No authentication data provided",
            }
            
            if ackCallback != nil {
                ackCallback(errorResponse)
            } else {
                client.Emit("auth_error", errorResponse)
            }
            return
        }

        // Parse authentication data
        log.Printf("Authentication data type: %T, value: %+v", authDataInput, authDataInput)
        
        var authData map[string]interface{}
        
        switch v := authDataInput.(type) {
        case string:
            log.Printf("Received string data: %s", v)
            
            // Handle Socket.IO format
            if len(v) > 0 && v[0] == '4' && v[1] == '2' {
                startIdx := strings.Index(v, `["authenticate",`)
                if startIdx != -1 {
                    jsonStart := strings.Index(v[startIdx:], `[`) + startIdx
                    jsonEnd := strings.LastIndex(v, `]`)
                    if jsonEnd > jsonStart {
                        jsonStr := v[jsonStart+1 : jsonEnd] 
                        if err := json.Unmarshal([]byte(jsonStr), &authData); err != nil {
                            log.Printf("Failed to parse JSON from string: %v", err)
                            errorResponse := map[string]interface{}{
                                "error": "Invalid JSON format in authentication data",
                            }
                            if ackCallback != nil {
                                ackCallback(errorResponse)
                            } else {
                                client.Emit("auth_error", errorResponse)
                            }
                            return
                        }
                    }
                }
            }
        case map[string]interface{}:
            // Handle the testing tool format
            if bodyStr, exists := v["body"].(string); exists {
                log.Printf("Found body string: %s", bodyStr)
                // Parse the body string as JSON
                if err := json.Unmarshal([]byte(bodyStr), &authData); err != nil {
                    log.Printf("Failed to parse body JSON: %v", err)
                    errorResponse := map[string]interface{}{
                        "error": "Invalid JSON format in body",
                    }
                    if ackCallback != nil {
                        ackCallback(errorResponse)
                    } else {
                        client.Emit("auth_error", errorResponse)
                    }
                    return
                }
            } else {
                // Direct format
                authData = v
            }
        default:
            log.Printf("Invalid authentication data format for client %s", client.Id())
            errorResponse := map[string]interface{}{
                "error": "Invalid authentication data format",
            }
            if ackCallback != nil {
                ackCallback(errorResponse)
            } else {
                client.Emit("auth_error", errorResponse)
            }
            return
        }
        
        if authData == nil {
            log.Printf("Failed to parse authentication data for client %s", client.Id())
            errorResponse := map[string]interface{}{
                "error": "Failed to parse authentication data",
            }
            if ackCallback != nil {
                ackCallback(errorResponse)
            } else {
                client.Emit("auth_error", errorResponse)
            }
            return
        }

        // Clean up token (remove extra quotes if present)
        token, exists := authData["token"].(string)
        if !exists {
            errorResponse := map[string]interface{}{
                "error": "Token is required",
            }
            if ackCallback != nil {
                ackCallback(errorResponse)
            } else {
                client.Emit("auth_error", errorResponse)
            }
            return
        }

        // Clean up token - remove extra quotes
        token = strings.Trim(token, `"`)
        log.Printf("Cleaned token: %s", token)

        // Validate token
        log.Printf("Validating token for client %s", client.Id())
        userID, err := sm.authService.ValidateToken(token)
        if err != nil {
            log.Printf("Token validation failed for client %s: %v", client.Id(), err)
            errorResponse := map[string]interface{}{
                "error": "Invalid token",
            }
            if ackCallback != nil {
                ackCallback(errorResponse)
            } else {
                client.Emit("auth_error", errorResponse)
            }
            return
        }

        // Get username
        username, exists := authData["username"].(string)
        if !exists {
            username = "Unknown User"
        }

        // Store authenticated client
        authClient := &AuthenticatedClient{
            UserID:   userID,
            Username: username,
            SocketID: client.Id(),
        }
        authenticatedClients.Store(client.Id(), authClient)

        // Auto-join user to their existing conversations
        sm.autoJoinUserToConversations(client, userID)

        log.Printf("User authenticated: %s (%s)", username, userID.String())
        log.Printf("Sending auth_success to client %s", client.Id())
        
        successResponse := map[string]interface{}{
            "message": "Successfully authenticated",
            "user_id": userID.String(),
        }
        
        if ackCallback != nil {
            ackCallback(successResponse)
        } else {
            client.Emit("auth_success", successResponse)
        }
    }
}

func (sm *SocketManager) handleStartChat(client *socket.Socket) func(...any) {
    return func(data ...any) {
    
        authClient, authenticated := sm.getAuthenticatedClient(client.Id())
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
        conversation, err := sm.chatService.StartChat(context.Background(), authClient.UserID, recipientID)
        if err != nil {
            log.Printf("Failed to create/get conversation: %v", err)
            client.Emit("error", map[string]interface{}{
                "error": "Failed to create conversation",
            })
            return
        }

        // Create conversation room name (consistent for both users)
        roomName := sm.createPrivateRoomName(authClient.UserID, recipientID)
        
        // Join the room
        client.Join(socket.Room(roomName))

        // Note: Recipient will be auto-joined when they authenticate

        log.Printf("User %s started chat with %s (room: %s, conversation: %s)", 
            authClient.UserID.String(), recipientID.String(), roomName, conversation.ID.String())

        client.Emit("chat_started", map[string]interface{}{
            "room":            roomName,
            "conversation_id": conversation.ID.String(),
            "recipient_id":    recipientID.String(),
        })
    }
}

func (sm *SocketManager) handleJoinConversation(client *socket.Socket) func(...any) {
    return func(data ...any) {
        authClient, authenticated := sm.getAuthenticatedClient(client.Id())
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
        conversation, err := sm.chatService.GetConversationByRoomName(context.Background(), roomName)
        if err != nil {
            log.Printf("Failed to get conversation for room %s: %v", roomName, err)
            client.Emit("error", map[string]interface{}{
                "error": "Failed to get conversation",
            })
            return
        }

        // Load recent messages (last 50 messages)
        messages, err := sm.chatService.GetConversationMessages(context.Background(), conversation.ID, 50, 0)
        if err != nil {
            log.Printf("Failed to load messages for conversation %s: %v", conversation.ID.String(), err)
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
        
        client.Emit("joined_conversation", map[string]interface{}{
            "room":            roomName,
            "conversation_id": conversation.ID.String(),
            "messages":        messageList,
        })
    }
}

func (sm *SocketManager) handleSendMessage(client *socket.Socket) func(...any) {
    return func(data ...any) {
        authClient, authenticated := sm.getAuthenticatedClient(client.Id())
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
        conversation, err := sm.chatService.GetConversationByRoomName(context.Background(), roomName)
        if err != nil {
            log.Printf("Failed to get conversation for room %s: %v", roomName, err)
            client.Emit("error", map[string]interface{}{
                "error": "Failed to get conversation",
            })
            return
        }

        // Save message to database
        dbMessage, err := sm.chatService.SendMessage(context.Background(), authClient.UserID, conversation.ID, encryptedContent, messageType)
        if err != nil {
            log.Printf("Failed to save message to database: %v", err)
            client.Emit("error", map[string]interface{}{
                "error": "Failed to save message",
            })
            return
        }

        // Create message object for socket emission
        message := map[string]interface{}{
            "id":               dbMessage.ID.String(),
            "sender_id":        authClient.UserID.String(),
            "sender_username":  authClient.Username,
            "encrypted_content": encryptedContent,
            "message_type":     messageType,
            "timestamp":        dbMessage.CreatedAt.Unix(),
            "room":            roomName,
            "conversation_id":  conversation.ID.String(),
        }

        // Send message to all users in the room
        sm.io.To(socket.Room(roomName)).Emit("new_message", message)
        
        // Debug: Log who should receive the message
        log.Printf("Message sent by %s to room %s (saved to DB)", authClient.Username, roomName)
        log.Printf("Broadcasting to room: %s", roomName)
        
        // Debug: List all authenticated clients in this room
        sm.debugRoomParticipants(roomName)
    }
}

func (sm *SocketManager) getAuthenticatedClient(socketID socket.SocketId) (*AuthenticatedClient, bool) {
    if client, exists := authenticatedClients.Load(socketID); exists {
        return client.(*AuthenticatedClient), true
    }
    return nil, false
}

func (sm *SocketManager) autoJoinUserToConversations(client *socket.Socket, userID uuid.UUID) {
    
    conversations, err := sm.chatService.GetUserConversations(context.Background(), userID)
    if err != nil {
        log.Printf("Failed to get conversations for user %s: %v", userID.String(), err)
        return
    }
    
    // Join user to all their conversation rooms
    for _, conversation := range conversations {
        // For each conversation, find the other participant to create room name
        participants, err := sm.chatService.GetConversationParticipants(context.Background(), conversation.ID)
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
            roomName := sm.createPrivateRoomName(userID, otherUserID)
            client.Join(socket.Room(roomName))
            log.Printf("Auto-joined user %s to room %s for conversation %s", 
                userID.String(), roomName, conversation.ID.String())
        }
    }
}

func (sm *SocketManager) debugRoomParticipants(roomName string) {
    log.Printf("=== Debug: Room %s participants ===", roomName)
    
    // List all authenticated clients and their rooms
    authenticatedClients.Range(func(key, value interface{}) bool {
        socketID := key.(socket.SocketId)
        authClient := value.(*AuthenticatedClient)
        log.Printf("User %s (%s) - Socket: %s", authClient.Username, authClient.UserID.String(), socketID)
        return true
    })
    
 
}

func (sm *SocketManager) createPrivateRoomName(userID1, userID2 uuid.UUID) string {
    // Create consistent room name regardless of user order
    if userID1.String() < userID2.String() {
        return "private_" + userID1.String() + "_" + userID2.String()
    }
    return "private_" + userID2.String() + "_" + userID1.String()
}

func (sm *SocketManager) ServeHTTP(c *gin.Context) {
    log.Printf("Socket.IO request: %s %s", c.Request.Method, c.Request.URL.Path)
    log.Printf("Request headers: %v", c.Request.Header)
    
    handler := sm.io.ServeHandler(nil)
    handler.ServeHTTP(c.Writer, c.Request)
}

func (sm *SocketManager) GetIO() *socket.Server {
    return sm.io
}