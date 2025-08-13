package routes

import (
	"Chat_App/handlers"
	"Chat_App/internal/middleware"
	"Chat_App/internal/repository"
	"Chat_App/internal/services"
	"Chat_App/internal/socket"
	"log"

	"github.com/gin-gonic/gin"
)
func SetupRoutes(authService *services.AuthService, userRepo *repository.UserRepo, chatService *services.ChatService, groupService *services.GroupService, socketManager *socket.SocketManager) *gin.Engine {
    r := gin.Default()

    authHandler := handlers.NewAuthHandler(authService)
    chatHandler := handlers.NewChatHandler(chatService, authService)
    groupHandler := handlers.NewGroupHandler(groupService)


    auth := r.Group("/auth")
    {
        auth.POST("/register", authHandler.Register)
        auth.POST("/login", authHandler.Login)
        auth.POST("/refresh", authHandler.RefreshToken)
        auth.POST("/refresh-by-email", authHandler.RefreshTokenByEmail)
        auth.POST("/logout", authHandler.Logout)
        auth.POST("/logout-by-email", authHandler.LogoutByEmail)
    }

    chat := r.Group("/chat")
    {
        chat.GET("/conversations", chatHandler.GetUserConversations)
        chat.GET("/conversations/:conversation_id/messages", chatHandler.GetConversationMessages)
        chat.POST("/start", chatHandler.StartChat)
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