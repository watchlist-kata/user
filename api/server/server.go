package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/watchlist-kata/protos/user"
	"github.com/watchlist-kata/user/internal/config"
	"github.com/watchlist-kata/user/internal/repository"
	"github.com/watchlist-kata/user/internal/service"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Загрузка конфигурации из .env файла
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Database connection string
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort, cfg.DBSSLMode)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("failed to connect to database", slog.Any("error", err))
		panic(fmt.Sprintf("failed to connect to database: %v", err))
	}

	// AutoMigrate the schema
	err = db.AutoMigrate(&repository.GormUser{})
	if err != nil {
		logger.Error("failed to migrate database schema", slog.Any("error", err))
		panic(fmt.Sprintf("failed to migrate database schema: %v", err))
	}

	// Create repository instance
	repo := repository.NewPostgresRepository(db, logger)

	// Create service instance
	userService := service.NewUserService(repo, logger)

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register the user service with the gRPC server
	user.RegisterUserServiceServer(grpcServer, userService)

	// Configure the listener port
	listener, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		logger.Error("failed to listen", slog.Any("error", err))
		panic(fmt.Sprintf("failed to listen: %v", err))
	}

	// Start the server
	logger.Info("starting gRPC server", slog.String("port", cfg.GRPCPort))
	if err := grpcServer.Serve(listener); err != nil {
		logger.Error("failed to serve", slog.Any("error", err))
		panic(fmt.Sprintf("failed to serve: %v", err))
	}
}
