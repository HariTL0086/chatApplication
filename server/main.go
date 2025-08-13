package main

import (
	"Chat_App/internal/config"
	"Chat_App/internal/database"
	"Chat_App/internal/repository"
	"Chat_App/internal/services"
	"Chat_App/internal/socket"
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

	// Connect to Redis using config
	redisService, err := services.NewRedisService(
		cfg.GetRedisAddress(),
		cfg.Redis.Password,
		cfg.Redis.DB,
	)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisService.Close()
	
	// Run migrations
	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	
	userRepo := repository.NewUserRepo(db.GetDB())
	refreshTokenRepo := repository.NewRefreshTokenRepository(db.GetDB())
	chatRepo := repository.NewChatRepository(db.GetDB())
	groupRepo := repository.NewGroupRepository(db.GetDB())
	
	// Pass config instead of just JWT secret
	authService := services.NewAuthService(userRepo, refreshTokenRepo, cfg)
	chatService := services.NewChatService(chatRepo, userRepo)
	groupService := services.NewGroupService(groupRepo, userRepo, chatRepo)
	
	// Create socket manager with Redis service
	socketManager := socket.NewSocketManager(authService, chatService, groupService, redisService)
	
	r := routes.SetupRoutes(authService, userRepo, chatService, groupService, socketManager)

	log.Printf("Server is running on %s", cfg.GetServerAddress())
	
	// Start the HTTP server
	if err := r.Run(cfg.GetServerAddress()); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}