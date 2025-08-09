package routes

import (
	"Chat_App/handlers"

	"Chat_App/internal/services"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(authService *services.AuthService) *gin.Engine {
	r := gin.Default()


	authHandler := handlers.NewAuthHandler(authService)

	// Public routes (no login needed)
	auth := r.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/refresh", authHandler.RefreshToken)
		auth.POST("/refresh-by-email", authHandler.RefreshTokenByEmail)
		auth.POST("/logout", authHandler.Logout)
		auth.POST("/logout-by-email", authHandler.LogoutByEmail)
	}

	// Protected routes (need login)
	return r
}