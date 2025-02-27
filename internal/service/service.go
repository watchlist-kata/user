package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"

	userProto "github.com/watchlist-kata/protos/user"
	"github.com/watchlist-kata/user/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UserService представляет собой структуру сервиса пользователей
type UserService struct {
	userProto.UnimplementedUserServiceServer
	repo   repository.Repository
	logger *slog.Logger
}

// NewUserService создает новый экземпляр UserService
func NewUserService(repo repository.Repository, logger *slog.Logger) *UserService {
	return &UserService{
		repo:   repo,
		logger: logger,
	}
}

// checkContextCancelled проверяет отмену контекста и логирует ошибку
func (s *UserService) checkContextCancelled(ctx context.Context, method string) error {
	select {
	case <-ctx.Done():
		s.logger.ErrorContext(ctx, fmt.Sprintf("%s operation canceled", method), slog.Any("error", ctx.Err()))
		return ctx.Err()
	default:
		return nil
	}
}

// GenerateSalt генерирует случайную соль
func GenerateSalt() (string, error) {
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	if err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}
	return base64.StdEncoding.EncodeToString(salt), nil
}

// HashPassword хэширует пароль с использованием соли
func HashPassword(password string, salt string) (string, error) {
	hashedPassword := password + salt
	hash, err := bcrypt.GenerateFromPassword([]byte(hashedPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// Create создает нового пользователя
func (s *UserService) Create(ctx context.Context, req *userProto.CreateUserRequest) (*userProto.CreateUserResponse, error) {
	if err := s.checkContextCancelled(ctx, "Create"); err != nil {
		return nil, status.Error(codes.Canceled, err.Error())
	}

	// Проверка уникальности имени пользователя
	_, err := s.repo.GetUserByUsername(ctx, req.Username)
	if err == nil {
		s.logger.WarnContext(ctx, fmt.Sprintf("username already exists: %s", req.Username))
		return nil, status.Error(codes.AlreadyExists, "username already exists")
	} else if !errors.Is(err, repository.ErrUserNotFound) {
		s.logger.ErrorContext(ctx, fmt.Sprintf("failed to check username uniqueness: %s", req.Username), slog.Any("error", err))
		return nil, status.Error(codes.Internal, "failed to check username uniqueness")
	}

	// Проверка уникальности электронной почты
	_, err = s.repo.GetUserByEmail(ctx, req.Email)
	if err == nil {
		s.logger.WarnContext(ctx, fmt.Sprintf("email already exists: %s", req.Email))
		return nil, status.Error(codes.AlreadyExists, "email already exists")
	} else if !errors.Is(err, repository.ErrUserNotFound) {
		s.logger.ErrorContext(ctx, fmt.Sprintf("failed to check email uniqueness: %s", req.Email), slog.Any("error", err))
		return nil, status.Error(codes.Internal, "failed to check email uniqueness")
	}

	// Генерация соли
	salt, err := GenerateSalt()
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to generate salt", slog.Any("error", err))
		return nil, status.Error(codes.Internal, "failed to generate salt")
	}

	// Хеширование пароля
	hashedPassword, err := HashPassword(req.Password, salt)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to hash password", slog.Any("error", err))
		return nil, status.Error(codes.Internal, "failed to hash password")
	}

	// Создание нового пользователя
	newUser := &userProto.User{
		Username: req.Username,
		Email:    req.Email,
		Pwdhash:  hashedPassword,
		Salt:     salt,
	}

	// Сохранение пользователя в базе данных
	createdUser, err := s.repo.CreateUser(ctx, newUser)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to create user", slog.Any("error", err))
		return nil, status.Error(codes.Internal, "failed to create user")
	}

	s.logger.InfoContext(ctx, fmt.Sprintf("user created successfully with username: %s", req.Username))
	return &userProto.CreateUserResponse{User: createdUser}, nil
}

// GetByID получает пользователя по ID
func (s *UserService) GetByID(ctx context.Context, req *userProto.GetUserRequest) (*userProto.GetUserResponse, error) {
	if err := s.checkContextCancelled(ctx, "GetByID"); err != nil {
		return nil, status.Error(codes.Canceled, err.Error())
	}

	userID := uint(req.Id)
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			s.logger.WarnContext(ctx, fmt.Sprintf("user not found with ID: %d", userID))
			return nil, status.Error(codes.NotFound, "user not found")
		}
		s.logger.ErrorContext(ctx, fmt.Sprintf("failed to get user by ID: %d", userID), slog.Any("error", err))
		return nil, status.Error(codes.Internal, "failed to get user")
	}

	s.logger.InfoContext(ctx, fmt.Sprintf("user fetched successfully with ID: %d", userID))
	return &userProto.GetUserResponse{User: user}, nil
}

// Update обновляет информацию о пользователе
func (s *UserService) Update(ctx context.Context, req *userProto.UpdateUserRequest) (*userProto.UpdateUserResponse, error) {
	if err := s.checkContextCancelled(ctx, "Update"); err != nil {
		return nil, status.Error(codes.Canceled, err.Error())
	}

	userID := req.Id
	s.logger.DebugContext(ctx, fmt.Sprintf("received request to update user with ID: %d", userID), slog.Any("request", req))

	// Получаем существующего пользователя по ID
	existingUser, err := s.repo.GetUserByID(ctx, uint(req.Id))
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			s.logger.WarnContext(ctx, fmt.Sprintf("user not found with ID: %d", userID))
			return nil, status.Error(codes.NotFound, "user not found")
		}
		s.logger.ErrorContext(ctx, fmt.Sprintf("failed to get user for update with ID: %d", userID), slog.Any("error", err))
		return nil, status.Error(codes.Internal, "failed to get user")
	}

	// Создаем объект для обновления
	userToUpdate := &userProto.User{
		Id:        req.Id,
		Username:  existingUser.Username,
		Email:     existingUser.Email,
		Pwdhash:   existingUser.Pwdhash,
		Salt:      existingUser.Salt,
		CreatedAt: existingUser.CreatedAt,
		UpdatedAt: existingUser.UpdatedAt,
	}

	// Обновляем разрешенные поля
	if req.Username != "" {
		userToUpdate.Username = req.Username
	}
	if req.Email != "" {
		userToUpdate.Email = req.Email
	}

	// Если передан новый пароль, генерируем соль и хэшируем
	if req.Password != "" {
		salt, err := GenerateSalt()
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to generate salt for password update", slog.Any("error", err))
			return nil, status.Error(codes.Internal, "failed to update password")
		}
		hashedPassword, err := HashPassword(req.Password, salt)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to hash password for update", slog.Any("error", err))
			return nil, status.Error(codes.Internal, "failed to update password")
		}
		userToUpdate.Pwdhash = hashedPassword
		userToUpdate.Salt = salt
	}

	// Обновляем пользователя в репозитории
	updatedUser, err := s.repo.UpdateUser(ctx, userToUpdate)
	if err != nil {
		s.logger.ErrorContext(ctx, fmt.Sprintf("failed to update user with ID: %d", userID), slog.Any("error", err))
		return nil, status.Error(codes.Internal, "failed to update user")
	}

	s.logger.InfoContext(ctx, fmt.Sprintf("user updated successfully with ID: %d", userID))
	return &userProto.UpdateUserResponse{User: updatedUser}, nil
}

// Delete удаляет пользователя по ID
func (s *UserService) Delete(ctx context.Context, req *userProto.DeleteUserRequest) (*userProto.DeleteUserResponse, error) {
	if err := s.checkContextCancelled(ctx, "Delete"); err != nil {
		return nil, status.Error(codes.Canceled, err.Error())
	}

	userID := uint(req.Id)
	err := s.repo.DeleteUser(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			s.logger.WarnContext(ctx, fmt.Sprintf("user not found with ID: %d", userID))
			return nil, status.Error(codes.NotFound, "user not found")
		}
		s.logger.ErrorContext(ctx, fmt.Sprintf("failed to delete user with ID: %d", userID), slog.Any("error", err))
		return nil, status.Error(codes.Internal, "failed to delete user")
	}

	s.logger.InfoContext(ctx, fmt.Sprintf("user deleted successfully with ID: %d", userID))
	return &userProto.DeleteUserResponse{Success: true}, nil
}

// CheckPass проверяет правильность пароля для заданного пользователя
func (s *UserService) CheckPass(ctx context.Context, req *userProto.CheckPasswordRequest) (*userProto.CheckPasswordResponse, error) {
	if err := s.checkContextCancelled(ctx, "CheckPass"); err != nil {
		return nil, status.Error(codes.Canceled, err.Error())
	}

	userID := req.UserId
	s.logger.DebugContext(ctx, fmt.Sprintf("received request to check password for user with ID: %d", userID))

	user, err := s.repo.GetUserByID(ctx, uint(req.UserId))
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			s.logger.WarnContext(ctx, fmt.Sprintf("user not found with ID: %d", userID))
			return nil, status.Error(codes.NotFound, "user not found")
		}
		s.logger.ErrorContext(ctx, fmt.Sprintf("failed to get user for password check with ID: %d", userID), slog.Any("error", err))
		return nil, status.Error(codes.Internal, "failed to check password")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Pwdhash), []byte(req.Password+user.Salt))
	if err != nil {
		s.logger.DebugContext(ctx, fmt.Sprintf("incorrect password for user with ID: %d", userID))
		return &userProto.CheckPasswordResponse{Valid: false}, nil
	}

	s.logger.InfoContext(ctx, fmt.Sprintf("password check successful for user with ID: %d", userID))
	return &userProto.CheckPasswordResponse{Valid: true}, nil
}
