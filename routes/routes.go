package routes

import (
	"Chat_App/handlers"
	"Chat_App/internal/middleware"
	"Chat_App/internal/repository"
	"Chat_App/internal/services"
	"Chat_App/internal/socket"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
)
func SetupRoutes(authService *services.AuthService, userRepo *repository.UserRepo, chatService *services.ChatService, groupService *services.GroupService, socketManager *socket.SocketManager) *gin.Engine {
    r := gin.Default()

    // Add CORS middleware
    r.Use(func(c *gin.Context) {
        c.Header("Access-Control-Allow-Origin", "*")
        c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
        
        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }
        
        c.Next()
    })

    authHandler := handlers.NewAuthHandler(authService)
    chatHandler := handlers.NewChatHandler(chatService, authService)
    groupHandler := handlers.NewGroupHandler(groupService)


    auth := r.Group("/auth")
    {
        auth.POST("/register", authHandler.Register)
        auth.POST("/login", authHandler.Login)
        auth.POST("/refresh", authHandler.RefreshToken)
        auth.GET("/user/:firebase_id", func(c *gin.Context) {
            firebaseID := c.Param("firebase_id")
            user, err := userRepo.GetUserByFirebaseID(c.Request.Context(), firebaseID)
            if err != nil {
                c.JSON(500, gin.H{"error": "Failed to get user"})
                return
            }
            if user == nil {
                c.JSON(404, gin.H{"error": "User not found"})
                return
            }
            c.JSON(200, user)
        })
       
    }

    chat := r.Group("/chat")
    chat.Use(middleware.AuthMiddleware(authService))
    {
        chat.GET("/conversations", chatHandler.GetUserConversations)
        chat.GET("/conversations/:conversation_id/messages", chatHandler.GetConversationMessages)
        chat.POST("/start", chatHandler.StartChat)
        chat.GET("/history/:conversation_id", chatHandler.GetChatHistory)
        chat.GET("/history/with/:user_id", chatHandler.GetChatHistoryWithUser)
        chat.GET("/group/:group_id/history", chatHandler.GetGroupChatHistory)
    }

    // User search API
    users := r.Group("/users")
    users.Use(middleware.AuthMiddleware(authService))
    {
        users.GET("/search", func(c *gin.Context) {
            username := c.Query("username")
            if username == "" {
                c.JSON(400, gin.H{"error": "Username parameter is required"})
                return
            }
            
            limit := 10 // Default limit
            if limitStr := c.Query("limit"); limitStr != "" {
                if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 50 {
                    limit = parsedLimit
                }
            }
            
            users, err := userRepo.SearchUsersByUsername(c.Request.Context(), username, limit)
            if err != nil {
                c.JSON(500, gin.H{"error": "Failed to search users"})
                return
            }
            
            // Return only necessary user information for search results
            var searchResults []gin.H
            for _, user := range users {
                searchResults = append(searchResults, gin.H{
                    "id":       user.ID,
                    "username": user.Username,
                    "email":    user.Email,
                })
            }
            
            c.JSON(200, gin.H{"users": searchResults})
        })
    }

   
    status := r.Group("/status")
    status.Use(middleware.AuthMiddleware(authService))
    {
        status.GET("/online", func(c *gin.Context) {
            onlineUsers, err := socketManager.GetRedisService().GetOnlineUsers()
            if err != nil {
                c.JSON(500, gin.H{"error": "Failed to get online users"})
                return
            }
            c.JSON(200, gin.H{"online_users": onlineUsers})
        })
        
        status.GET("/user/:user_id", func(c *gin.Context) {
            userID := c.Param("user_id")
            userStatus, err := socketManager.GetRedisService().GetUserStatus(userID)
            if err != nil {
                c.JSON(500, gin.H{"error": "Failed to get user status"})
                return
            }
            c.JSON(200, gin.H{"user_status": userStatus})
        })

        status.POST("/users", func(c *gin.Context) {
            var request struct {
                UserIDs []string `json:"user_ids" binding:"required"`
            }
            
            if err := c.ShouldBindJSON(&request); err != nil {
                c.JSON(400, gin.H{"error": "Invalid request format"})
                return
            }
            
            usersStatus, err := socketManager.GetRedisService().GetUsersStatus(request.UserIDs)
            if err != nil {
                c.JSON(500, gin.H{"error": "Failed to get users status"})
                return
            }
            
            c.JSON(200, gin.H{"users_status": usersStatus})
        })
    }

    
    groups := r.Group("/groups")
    groups.Use(middleware.AuthMiddleware(authService))
    {
        groups.POST("/", groupHandler.CreateGroup)
        groups.GET("/", groupHandler.GetUserGroups)
        groups.GET("/:id", groupHandler.GetGroup)
        groups.PUT("/:id", groupHandler.UpdateGroup)
        groups.DELETE("/:id", groupHandler.DeleteGroup)
        groups.GET("/:id/members", groupHandler.GetGroupMembers)
        groups.POST("/:id/members", groupHandler.AddGroupMember)
        groups.DELETE("/:id/members", groupHandler.RemoveGroupMember)
        groups.PUT("/:id/members/role", groupHandler.ChangeMemberRole)
        groups.POST("/:id/leave", groupHandler.LeaveGroup)
    }

 
    
    r.Any("/socket.io/*any", func(c *gin.Context) {
        log.Printf("Socket.IO route hit: %s %s", c.Request.Method, c.Request.URL.String())
        socketManager.ServeHTTP(c)
    })

    return r
}