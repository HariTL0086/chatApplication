package middleware

import (
	"Chat_App/internal/services"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware(authService *services.AuthService) gin.HandlerFunc {

	return func(c *gin.Context){
		authHeader := c.GetHeader("Authorization")
		if authHeader ==""{
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.ABORT()
			return 
		}
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer"{
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
		token := tokenParts[1]
		userID, err := authService.ValidateToken(token)
		if err != nil{
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.ABORT()
			return 
		}
		c.Set("userID", userID)
		c.Next()
	}
}
