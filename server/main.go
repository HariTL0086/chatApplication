package main

import (
	"Chat_App/internal/config"
	"Chat_App/internal/database"
	"Chat_App/internal/repository"
	"Chat_App/internal/services"
	"Chat_App/routes"
	"log"
)

func main() {
	log.Println("🚀 Starting server..")
	
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	// Connect to database using config
	db, err := database.NewDB(cfg.GetDatabaseURL())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	
	userRepo := repository.NewUserRepo(db.Pool)
	refreshTokenRepo := repository.NewRefreshTokenRepo(db.Pool)
	
	// Pass config instead of just JWT secret
	authService := services.NewAuthService(userRepo,refreshTokenRepo,cfg)
	r := routes.SetupRoutes(authService)

	log.Printf("Server is running on %s", cfg.GetServerAddress())

	
	if err := r.Run(cfg.GetServerAddress()); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
