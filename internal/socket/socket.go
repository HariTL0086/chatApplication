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


type ChatMessage struct {
	ID               string `json:"id"`
	SenderID         string `json:"sender_id"`
	SenderUsername   string `json:"sender_username"`
	Content          string `json:"content"`
	MessageType      string `json:"message_type"`
	Timestamp        int64  `json:"timestamp"`
	Room             string `json:"room"`
	ConversationID   string `json:"conversation_id"`
}


type GroupMessage struct {
	ID               string `json:"id"`
	SenderID         string `json:"sender_id"`
	SenderUsername   string `json:"sender_username"`
	Content          string `json:"content"`
	MessageType      string `json:"message_type"`
	Timestamp        int64  `json:"timestamp"`
	Room             string `json:"room"`
	GroupID          string `json:"group_id"`
	ConversationID   string `json:"conversation_id"`
}


type GroupMember struct {
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
	JoinedAt int64  `json:"joined_at"`
}


type GroupInfo struct {
	GroupID    string         `json:"group_id"`
	Name       string         `json:"name"`
	Description string        `json:"description"`
	CreatedBy  string         `json:"created_by"`
	CreatedAt  int64          `json:"created_at"`
	UpdatedAt  int64          `json:"updated_at"`
	Members    []GroupMember  `json:"members"`
}

var authenticatedClients sync.Map 


type SocketManager struct {
	io              *socket.Server
	authService     *services.AuthService
	redisService    *services.RedisService
	chatHandler     *ChatSocketHandler
	groupHandler    *GroupSocketHandler
}


func NewSocketManager(authService *services.AuthService, chatService *services.ChatService, groupService *services.GroupService, redisService *services.RedisService) *SocketManager {
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

    
    transports := types.NewSet[string]()
    transports.Add("websocket")
    transports.Add("polling")
    options.SetTransports(transports)

    io := socket.NewServer(httpServer, options)

	
	chatHandler := NewChatSocketHandler(chatService, io, redisService)
	groupHandler := NewGroupSocketHandler(groupService, chatService, io, redisService)

    sm := &SocketManager{
		io:           io,
		authService:  authService,
		redisService: redisService,
		chatHandler:  chatHandler,
		groupHandler: groupHandler,
    }

    sm.setupEventHandlers()
    return sm
}


func (sm *SocketManager) setupEventHandlers() {
    sm.io.On("connection", func(clients ...any) {
        client := clients[0].(*socket.Socket)
        log.Printf("Client connected: %s", client.Id())

       
        client.On("authenticate", sm.handleAuthentication(client))
        
	
		sm.chatHandler.SetupChatHandlers(client)
		
	
		sm.groupHandler.SetupGroupHandlers(client)

        client.On("disconnect", func(...any) {
            log.Printf("Client disconnected: %s", client.Id())
            
            
            if authClient, exists := authenticatedClients.Load(client.Id()); exists {
                if user, ok := authClient.(*AuthenticatedClient); ok {
                    if err := sm.redisService.SetUserOffline(user.UserID); err != nil {
                        log.Printf("Failed to set user offline: %v", err)
                    }
                    
                    
                    relevantUserIDs, err := sm.getRelevantUserIDs(user.UserID)
                    if err != nil {
                        log.Printf("Failed to get relevant user IDs for %s: %v", user.UserID.String(), err)
                        
                        sm.io.Emit("user_status_change", map[string]interface{}{
                            "user_id": user.UserID.String(),
                            "username": user.Username,
                            "status": "offline",
                            "last_seen": time.Now().Unix(),
                        })
                        return
                    }
                    
                
                    if len(relevantUserIDs) > 0 {
                        sm.emitStatusChangeToUsers(relevantUserIDs, map[string]interface{}{
                            "user_id": user.UserID.String(),
                            "username": user.Username,
                            "status": "offline",
                            "last_seen": time.Now().Unix(),
                        })
                    }
                }
            }
            
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

     
        if err := sm.redisService.SetUserOnline(userID, username, string(client.Id())); err != nil {
            log.Printf("Failed to set user online: %v", err)
        }

     
        relevantUserIDs, err := sm.getRelevantUserIDs(userID)
        if err != nil {
            log.Printf("Failed to get relevant user IDs for %s: %v", userID.String(), err)
         
            sm.io.Emit("user_status_change", map[string]interface{}{
                "user_id": userID.String(),
                "username": username,
                "status": "online",
                "last_seen": time.Now().Unix(),
            })
        } else {
         
            if len(relevantUserIDs) > 0 {
                sm.emitStatusChangeToUsers(relevantUserIDs, map[string]interface{}{
                    "user_id": userID.String(),
                    "username": username,
                    "status": "online",
                    "last_seen": time.Now().Unix(),
                })
            }
        }

        // Join user to their personal room for receiving decrypted messages
        client.Join(socket.Room("user_" + userID.String()))

     
		sm.chatHandler.autoJoinUserToConversations(client, userID)

     
		sm.chatHandler.DeliverOfflineMessages(client, userID)
		sm.groupHandler.DeliverOfflineGroupMessages(client, userID)

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


func getAuthenticatedClient(socketID socket.SocketId) (*AuthenticatedClient, bool) {
    if client, exists := authenticatedClients.Load(socketID); exists {
        return client.(*AuthenticatedClient), true
    }
    return nil, false
}


func (sm *SocketManager) getRelevantUserIDs(userID uuid.UUID) ([]string, error) {
    var relevantUserIDs []string
    ctx := context.Background()
    

    conversations, err := sm.chatHandler.chatService.GetUserConversations(ctx, userID)
    if err != nil {
        log.Printf("Failed to get conversations for user %s: %v", userID.String(), err)
        return nil, err
    }
    
   
    for _, conversation := range conversations {
        if conversation.Type == "private" {
            participants, err := sm.chatHandler.chatService.GetConversationParticipants(ctx, conversation.ID)
            if err != nil {
                log.Printf("Failed to get participants for conversation %s: %v", conversation.ID.String(), err)
                continue
            }
            
            for _, participantID := range participants {
                if participantID != userID {
                    relevantUserIDs = append(relevantUserIDs, participantID.String())
                }
            }
        }
    }
    

    groups, err := sm.groupHandler.groupService.GetUserGroups(ctx, userID)
    if err != nil {
        log.Printf("Failed to get groups for user %s: %v", userID.String(), err)
       
    } else {
        for _, group := range groups {
            members, err := sm.groupHandler.groupService.GetGroupMembers(ctx, group.ID)
            if err != nil {
                log.Printf("Failed to get members for group %s: %v", group.ID.String(), err)
                continue
            }
            
            for _, member := range members {
                if member.UserID != userID {
                    relevantUserIDs = append(relevantUserIDs, member.UserID.String())
                }
            }
        }
    }
    
    
    uniqueIDs := make(map[string]bool)
    var uniqueRelevantUserIDs []string
    for _, id := range relevantUserIDs {
        if !uniqueIDs[id] {
            uniqueIDs[id] = true
            uniqueRelevantUserIDs = append(uniqueRelevantUserIDs, id)
        }
    }
    
    return uniqueRelevantUserIDs, nil
}


func (sm *SocketManager) emitStatusChangeToUsers(userIDs []string, statusData map[string]interface{}) {

    var targetSocketIDs []string
    
    authenticatedClients.Range(func(key, value interface{}) bool {
        if authClient, ok := value.(*AuthenticatedClient); ok {
            for _, userID := range userIDs {
                if authClient.UserID.String() == userID {
                    targetSocketIDs = append(targetSocketIDs, string(authClient.SocketID))
                    break
                }
            }
        }
        return true
    })
    
 
    for _, socketID := range targetSocketIDs {
        sm.io.To(socket.Room(socketID)).Emit("user_status_change", statusData)
    }
    
    log.Printf("Emitted status change for user %s to %d relevant users", 
        statusData["user_id"], len(targetSocketIDs))
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


func (sm *SocketManager) GetRedisService() *services.RedisService {
    return sm.redisService
}