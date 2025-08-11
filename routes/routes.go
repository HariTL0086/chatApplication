package routes

import (
	"Chat_App/handlers"
	"Chat_App/internal/repository"
	"Chat_App/internal/services"
	"Chat_App/internal/socket"
	"log"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(authService *services.AuthService, userRepo *repository.UserRepo, chatService *services.ChatService) *gin.Engine {
    r := gin.Default()

    r.GET("/test-chat", func(c *gin.Context) {
        log.Printf("Test-chat route accessed")
        // Use absolute path to ensure file is found
        filePath := filepath.Join("/home/tl0086/Project/ChatApplication", "test.html")
        log.Printf("Attempting to serve file: %s", filePath)
        c.File(filePath)
    })

    socketManager := socket.NewSocketManager(authService, chatService)

    authHandler := handlers.NewAuthHandler(authService)
    chatHandler := handlers.NewChatHandler(chatService, authService)

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
    
    r.Any("/socket.io/*any", func(c *gin.Context) {
        log.Printf("Socket.IO route hit: %s %s", c.Request.Method, c.Request.URL.String())
        socketManager.ServeHTTP(c)
    })

    return r
}