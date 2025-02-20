package main

import (
	"log"
	"net"

	"github.com/watchlist-kata/protos/user"
	"github.com/watchlist-kata/user/internal/config"
	"github.com/watchlist-kata/user/internal/repository"
	"github.com/watchlist-kata/user/internal/service"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Загружаем конфигурацию из .env файла
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Формируем строку подключения к базе данных PostgreSQL
	dsn := "host=" + cfg.DBHost + " user=" + cfg.DBUser + " password=" + cfg.DBPassword +
		" dbname=" + cfg.DBName + " port=" + cfg.DBPort + " sslmode=" + cfg.DBSSLMode

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// Создание экземпляра репозитория
	repo := repository.NewPostgresRepository(db)

	// Создание экземпляра сервиса пользователей
	userService := service.NewUserService(repo)

	// Создание нового gRPC сервера
	grpcServer := grpc.NewServer()

	// Регистрация сервиса пользователей в gRPC сервере
	user.RegisterUserServiceServer(grpcServer, userService)

	// Настройка порта для сервера
	listener, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Println("Starting server on " + cfg.GRPCPort + "...")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
