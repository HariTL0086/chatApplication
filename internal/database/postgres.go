package database

import (
	"Chat_App/internal/models"
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	GormDB *gorm.DB
}


func NewDB(databaseURL string) (*DB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test the connection
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("✅ Database connected successfully with GORM")
	return &DB{GormDB: db}, nil
}


func (db *DB) Close() {
	if db.GormDB != nil {
		sqlDB, err := db.GormDB.DB()
		if err == nil {
			sqlDB.Close()
			log.Println("🔌 Database connection closed")
		}
	}
}


func (db *DB) Ping() error {
	sqlDB, err := db.GormDB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}


func (db *DB) GetDB() *gorm.DB {
	return db.GormDB
}


func (db *DB) AutoMigrate() error {
	return db.GormDB.AutoMigrate(
		&models.User{},
		&models.Message{},
		&models.RefreshToken{},
		&models.Conversation{},
		&models.Group{},
		&models.GroupMember{},
	)
}