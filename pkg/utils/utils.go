package utils

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"

	"github.com/watchlist-kata/user/internal/config"
)

// ConnectToDatabase устанавливает подключение к базе данных PostgreSQL
func ConnectToDatabase(cfg *config.Config) (*gorm.DB, error) {
	dsn := "host=" + cfg.DBHost + " user=" + cfg.DBUser + " password=" + cfg.DBPassword +
		" dbname=" + cfg.DBName + " port=" + cfg.DBPort + " sslmode=" + cfg.DBSSLMode

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("failed to connect to database: %v", err)
		return nil, err
	}

	return db, nil
}
