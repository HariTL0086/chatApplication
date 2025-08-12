package socket

import (
	"Chat_App/internal/services"
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

// ===== TYPES AND STRUCTURES =====

// AuthenticatedClient represents an authenticated socket client
type AuthenticatedClient struct {
    UserID   uuid.UUID
    Username string
    SocketID socket.SocketId
}

// ChatMessage represents a chat message for socket emission
type ChatMessage struct {
	ID               string `json:"id"`
	SenderID         string `json:"sender_id"`
	SenderUsername   string `json:"sender_username"`
	EncryptedContent string `json:"encrypted_content"`
	MessageType      string `json:"message_type"`
	Timestamp        int64  `json:"timestamp"`
	Room             string `json:"room"`
	ConversationID   string `json:"conversation_id"`
}

// GroupMessage represents a group message for socket emission
type GroupMessage struct {
	ID               string `json:"id"`
	SenderID         string `json:"sender_id"`
	SenderUsername   string `json:"sender_username"`
	EncryptedContent string `json:"encrypted_content"`
	MessageType      string `json:"message_type"`
	Timestamp        int64  `json:"timestamp"`
	Room             string `json:"room"`
	GroupID          string `json:"group_id"`
	ConversationID   string `json:"conversation_id"`
}

// GroupMember represents a group member for socket emission
type GroupMember struct {
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
	JoinedAt int64  `json:"joined_at"`
}

// GroupInfo represents group information for socket emission
type GroupInfo struct {
	GroupID    string         `json:"group_id"`
	Name       string         `json:"name"`
	Description string        `json:"description"`
	CreatedBy  string         `json:"created_by"`
	CreatedAt  int64          `json:"created_at"`
	UpdatedAt  int64          `json:"updated_at"`
	Members    []GroupMember  `json:"members"`
}

// ===== GLOBAL VARIABLES =====

// Global variable for authenticated clients
var authenticatedClients sync.Map // map[socket.SocketId]*AuthenticatedClient

// SocketManager manages the main socket connection and coordinates handlers
type SocketManager struct {
	io              *socket.Server
	authService     *services.AuthService
	chatHandler     *ChatSocketHandler
	groupHandler    *GroupSocketHandler
}

// NewSocketManager creates a new socket manager with separate handlers
func NewSocketManager(authService *services.AuthService, chatService *services.ChatService, groupService *services.GroupService) *SocketManager {
    httpServer := types.CreateServer(nil)
    options := socket.DefaultServerOptions()

    options.SetCors(&types.Cors{
		Origin:      "*",
		Methods:     []string{"GET", "POST", "OPTIONS"},
		Headers:     []string{"Content-Type", "Authorization"},
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

	// Create separate handlers
	chatHandler := NewChatSocketHandler(chatService, io)
	groupHandler := NewGroupSocketHandler(groupService, chatService, io)

    sm := &SocketManager{
		io:           io,
		authService:  authService,
		chatHandler:  chatHandler,
		groupHandler: groupHandler,
    }

    sm.setupEventHandlers()
    return sm
}

// setupEventHandlers sets up the main connection handler and delegates to specific handlers
func (sm *SocketManager) setupEventHandlers() {
    sm.io.On("connection", func(clients ...any) {
        client := clients[0].(*socket.Socket)
        log.Printf("Client connected: %s", client.Id())

        // Handle authentication
        client.On("authenticate", sm.handleAuthentication(client))
        
		// Setup chat handlers
		sm.chatHandler.SetupChatHandlers(client)
		
		// Setup group handlers
		sm.groupHandler.SetupGroupHandlers(client)

        client.On("disconnect", func(...any) {
            log.Printf("Client disconnected: %s", client.Id())
            authenticatedClients.Delete(client.Id())
        })

        client.Emit("auth_required", map[string]interface{}{
            "message": "Please authenticate to use chat features",
        })
    })
}

// handleAuthentication handles socket authentication
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
		sm.chatHandler.autoJoinUserToConversations(client, userID)

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

// getAuthenticatedClient retrieves an authenticated client by socket ID
func getAuthenticatedClient(socketID socket.SocketId) (*AuthenticatedClient, bool) {
    if client, exists := authenticatedClients.Load(socketID); exists {
        return client.(*AuthenticatedClient), true
    }
    return nil, false
}

// ServeHTTP handles HTTP requests for the socket server
func (sm *SocketManager) ServeHTTP(c *gin.Context) {
    log.Printf("Socket.IO request: %s %s", c.Request.Method, c.Request.URL.Path)
    log.Printf("Request headers: %v", c.Request.Header)
    
    handler := sm.io.ServeHandler(nil)
    handler.ServeHTTP(c.Writer, c.Request)
}

// GetIO returns the socket.io server instance
func (sm *SocketManager) GetIO() *socket.Server {
    return sm.io
}