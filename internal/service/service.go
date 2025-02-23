package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"

	userProto "github.com/watchlist-kata/protos/user"
	"github.com/watchlist-kata/user/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// UserService представляет собой структуру сервиса пользователей
type UserService struct {
	userProto.UnimplementedUserServiceServer                       // Встраиваем структуру
	repo                                     repository.Repository // Интерфейс репозитория пользователей
}

// NewUserService создает новый экземпляр UserService
func NewUserService(repo repository.Repository) *UserService {
	return &UserService{
		repo: repo,
	}
}

// GenerateSalt генерирует случайную соль
func GenerateSalt() (string, error) {
	salt := make([]byte, 16) // Генерируем 16 байт соли
	_, err := rand.Read(salt)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(salt), nil
}

// HashPassword хэширует пароль с использованием соли
func HashPassword(password string, salt string) (string, error) {
	hashedPassword := password + salt // Добавляем соль к паролю
	hash, err := bcrypt.GenerateFromPassword([]byte(hashedPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// Create создает нового пользователя
func (s *UserService) Create(ctx context.Context, req *userProto.CreateUserRequest) (*userProto.CreateUserResponse, error) {
	// Проверка уникальности имени пользователя и электронной почты
	if _, err := s.repo.GetUserByUsername(req.Username); err == nil {
		return nil, errors.New("username already exists")
	}

	if _, err := s.repo.GetUserByEmail(req.Email); err == nil {
		return nil, errors.New("email already exists")
	}

	// Генерация соли
	salt, err := GenerateSalt()
	if err != nil {
		return nil, err
	}

	// Хеширование пароля
	hashedPassword, err := HashPassword(req.Password, salt)
	if err != nil {
		return nil, err
	}

	// Создание нового пользователя
	newUser := &userProto.User{
		Username: req.Username,
		Email:    req.Email,
		Pwdhash:  hashedPassword,
		Salt:     salt,
	}

	// Сохранение пользователя в базе данных
	if err := s.repo.CreateUser(newUser); err != nil {
		return nil, err // Обработка ошибки при сохранении
	}

	return &userProto.CreateUserResponse{User: newUser}, nil
}

// GetByID получает пользователя по ID
func (s *UserService) GetByID(ctx context.Context, req *userProto.GetUserRequest) (*userProto.GetUserResponse, error) {
	user, err := s.repo.GetUserByID(uint(req.Id))
	if err != nil {
		return nil, err // Пользователь не найден или произошла ошибка
	}
	return &userProto.GetUserResponse{User: user}, nil
}

// Update обновляет информацию о пользователе
func (s *UserService) Update(ctx context.Context, req *userProto.UpdateUserRequest) (*userProto.UpdateUserResponse, error) {
	// Логируем полученный ID и другие поля
	log.Printf("Received request to update user with ID: %d", req.Id)
	log.Printf("Request body: %+v", req)

	// Создаем объект userToUpdate на основе данных из запроса
	userToUpdate := &userProto.User{
		Id:       req.Id,
		Username: req.Username,
		Email:    req.Email,
		// Мы не устанавливаем Pwdhash и Salt здесь, так как они будут обновлены только если передан новый пароль
	}

	// Получаем существующего пользователя по ID из базы данных
	existingUser, err := s.repo.GetUserByID(uint(req.Id))
	if err != nil {
		log.Printf("Error retrieving user: %v", err)
		return nil, err // Обработка ошибки при получении пользователя
	}

	// Обновляем поля пользователя, если они были изменены в запросе
	if req.Username != "" {
		userToUpdate.Username = req.Username
	} else {
		userToUpdate.Username = existingUser.Username // Сохраняем старое имя, если новое не передано
	}

	if req.Email != "" {
		userToUpdate.Email = req.Email
	} else {
		userToUpdate.Email = existingUser.Email // Сохраняем старый email, если новый не передан
	}

	if req.Password != "" {
		salt, err := GenerateSalt()
		if err != nil {
			return nil, err
		}
		hashedPassword, err := HashPassword(req.Password, salt)
		if err != nil {
			return nil, err
		}
		userToUpdate.Pwdhash = hashedPassword
		userToUpdate.Salt = salt
	} else {
		userToUpdate.Pwdhash = existingUser.Pwdhash // Сохраняем старый хеш пароля, если новый не передан
		userToUpdate.Salt = existingUser.Salt       // Сохраняем старую соль, если новый не передан
	}

	// Сохраняем обновленного пользователя в базе данных
	if err := s.repo.UpdateUser(userToUpdate); err != nil {
		log.Printf("Error updating user: %v", err)
		return nil, err // Обработка ошибки при обновлении
	}

	log.Printf("User updated successfully: %+v", userToUpdate)

	return &userProto.UpdateUserResponse{User: userToUpdate}, nil // Возвращаем обновленного пользователя в ответе
}

// Delete удаляет пользователя по ID
func (s *UserService) Delete(ctx context.Context, req *userProto.DeleteUserRequest) (*userProto.DeleteUserResponse, error) {
	err := s.repo.DeleteUser(uint(req.Id))
	if err != nil {
		return &userProto.DeleteUserResponse{Success: false}, err // Обработка ошибки при удалении
	}
	return &userProto.DeleteUserResponse{Success: true}, nil
}

// CheckPass проверяет правильность пароля для заданного пользователя
func (s *UserService) CheckPass(ctx context.Context, req *userProto.CheckPasswordRequest) (*userProto.CheckPasswordResponse, error) {
	log.Printf("Received request to check password for userId: %d", req.UserId) // Логируем полученный userId

	user, err := s.repo.GetUserByID(uint(req.UserId))
	if err != nil {
		log.Printf("Error retrieving user: %v", err)
		return nil, errors.New("user not found") // Пользователь не найден или произошла ошибка
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Pwdhash), []byte(req.Password+user.Salt))
	if err != nil {
		return &userProto.CheckPasswordResponse{Valid: false}, nil // Пароль неверен
	}

	return &userProto.CheckPasswordResponse{Valid: true}, nil // Пароль верен
}
