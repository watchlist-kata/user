package main

import (
	"github.com/watchlist-kata/protos/user"
	"github.com/watchlist-kata/user/internal/config"
	"github.com/watchlist-kata/user/internal/repository"
	"github.com/watchlist-kata/user/internal/service"
	"github.com/watchlist-kata/user/pkg/logger"
	"github.com/watchlist-kata/user/pkg/utils"
	"google.golang.org/grpc"
	"log"
	"net"
)

func main() {
	// Загружаем конфигурацию из .env файла
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Инициализируем кастомный логгер
	customLogger, err := logger.NewLogger(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.ServiceName, cfg.LogBufferSize)
	if err != nil {
		log.Fatalf("failed to create custom logger: %v", err)
	}
	defer func() {
		if multiHandler, ok := customLogger.Handler().(*logger.MultiHandler); ok {
			multiHandler.CloseAll()
		}
	}()

	// Создание подключения к базе данных
	db, err := utils.ConnectToDatabase(cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// Создание экземпляра репозитория
	repo := repository.NewPostgresRepository(db, customLogger)

	// Создание экземпляра сервиса пользователей
	userService := service.NewUserService(repo, customLogger)

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
