package main

import (
	"log"
	"net"

	"github.com/watchlist-kata/protos/user"
	"github.com/watchlist-kata/user/internal/repository"
	"github.com/watchlist-kata/user/internal/service"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Настройка подключения к базе данных PostgreSQL
	dsn := "host=localhost user=youruser password=yourpassword dbname=yourdb port=5432 sslmode=disable"
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
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Println("Starting server on :50051...")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
