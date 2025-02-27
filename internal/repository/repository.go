package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/watchlist-kata/protos/user"
	"gorm.io/gorm"
)

var ErrUserNotFound = errors.New("user not found")

type Repository interface {
	CreateUser(ctx context.Context, user *user.User) (*user.User, error)
	GetUserByID(ctx context.Context, id uint) (*user.User, error)
	GetUserByUsername(ctx context.Context, username string) (*user.User, error)
	GetUserByEmail(ctx context.Context, email string) (*user.User, error)
	UpdateUser(ctx context.Context, user *user.User) (*user.User, error)
	DeleteUser(ctx context.Context, id uint) error
}

// PostgresRepository реализация репозитория с использованием GORM
type PostgresRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewPostgresRepository создает новый экземпляр PostgresRepository
func NewPostgresRepository(db *gorm.DB, logger *slog.Logger) *PostgresRepository {
	return &PostgresRepository{db: db, logger: logger}
}

// validateUser проверяет основные поля пользователя
func (r *PostgresRepository) validateUser(user *user.User) error {
	if user == nil {
		return fmt.Errorf("user cannot be nil")
	}

	if user.Username == "" || len(user.Username) > 50 || !utf8.ValidString(user.Username) {
		return fmt.Errorf("invalid username: must be 1-50 characters and valid UTF-8")
	}

	if user.Email == "" || len(user.Email) > 254 || !utf8.ValidString(user.Email) {
		return fmt.Errorf("invalid email: must be 1-254 characters and valid UTF-8")
	}

	if user.Pwdhash == "" || len(user.Pwdhash) > 1000 {
		return fmt.Errorf("invalid password hash: must be 1-1000 characters")
	}

	if user.Salt == "" || len(user.Salt) > 255 {
		return fmt.Errorf("invalid salt: must be 1-255 characters")
	}

	return nil
}

// CreateUser создает нового пользователя в базе данных
func (r *PostgresRepository) CreateUser(ctx context.Context, user *user.User) (*user.User, error) {
	// Проверка отмены контекста
	select {
	case <-ctx.Done():
		r.logger.Error(fmt.Sprintf("CreateUser operation canceled for user ID: %d", user.Id), slog.Any("error", ctx.Err()))
		return nil, ctx.Err()
	default:
	}

	// Валидация входных данные
	if err := r.validateUser(user); err != nil {
		r.logger.Error(fmt.Sprintf("failed to create user: invalid data for user ID: %d", user.Id), slog.Any("error", err))
		return nil, err
	}

	gormUser := &GormUser{
		Username: user.Username,
		Email:    user.Email,
		Pwdhash:  user.Pwdhash,
		Salt:     user.Salt,
	}

	// Транзакционная операция
	tx := r.db.Begin()
	if tx.Error != nil {
		r.logger.Error(fmt.Sprintf("failed to begin transaction for user ID: %d", user.Id), slog.Any("error", tx.Error))
		return nil, tx.Error
	}

	// Проверка контекста перед созданием пользователя
	select {
	case <-ctx.Done():
		tx.Rollback()
		r.logger.Error(fmt.Sprintf("CreateUser operation canceled during transaction for user ID: %d", user.Id), slog.Any("error", ctx.Err()))
		return nil, ctx.Err()
	default:
	}

	if err := tx.Create(gormUser).Error; err != nil {
		tx.Rollback()
		r.logger.Error(fmt.Sprintf("failed to create user with username: %s", user.Username), slog.Any("error", err))
		return nil, err
	}

	// Проверка контекста после создания пользователя
	select {
	case <-ctx.Done():
		tx.Rollback()
		r.logger.Error(fmt.Sprintf("CreateUser operation canceled after user creation for user ID: %d", user.Id), slog.Any("error", ctx.Err()))
		return nil, ctx.Err()
	default:
	}

	if err := tx.Commit().Error; err != nil {
		r.logger.Error(fmt.Sprintf("failed to commit transaction for user ID: %d", user.Id), slog.Any("error", err))
		return nil, err
	}

	r.logger.Info(fmt.Sprintf("user created successfully with username: %s", user.Username))
	return convertToProtoUser(gormUser), nil
}

// GetUserByID получает пользователя по ID
func (r *PostgresRepository) GetUserByID(ctx context.Context, id uint) (*user.User, error) {
	// Проверка отмены контекста
	select {
	case <-ctx.Done():
		r.logger.Error(fmt.Sprintf("GetUserByID operation canceled for user ID: %d", id), slog.Any("error", ctx.Err()))
		return nil, ctx.Err()
	default:
	}

	var gormUser GormUser
	if err := r.db.First(&gormUser, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger.Warn(fmt.Sprintf("user not found with ID: %d", id))
			return nil, ErrUserNotFound
		}
		r.logger.Error(fmt.Sprintf("failed to get user with ID: %d", id), slog.Any("error", err))
		return nil, err
	}

	r.logger.Info(fmt.Sprintf("user fetched successfully with ID: %d", id))
	return convertToProtoUser(&gormUser), nil
}

// GetUserByUsername получает пользователя по имени пользователя
func (r *PostgresRepository) GetUserByUsername(ctx context.Context, username string) (*user.User, error) {
	// Проверка отмены контекста
	select {
	case <-ctx.Done():
		r.logger.Error(fmt.Sprintf("GetUserByUsername operation canceled for username: %s", username), slog.Any("error", ctx.Err()))
		return nil, ctx.Err()
	default:
	}

	var gormUser GormUser
	if err := r.db.Where("username = ?", username).First(&gormUser).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger.Warn(fmt.Sprintf("user not found with username: %s", username))
			return nil, ErrUserNotFound
		}
		r.logger.Error(fmt.Sprintf("failed to get user with username: %s", username), slog.Any("error", err))
		return nil, err
	}

	r.logger.Info(fmt.Sprintf("user fetched successfully with username: %s", username))
	return convertToProtoUser(&gormUser), nil
}

// GetUserByEmail получает пользователя по электронной почте
func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (*user.User, error) {
	// Проверка отмены контекста
	select {
	case <-ctx.Done():
		r.logger.Error(fmt.Sprintf("GetUserByEmail operation canceled for email: %s", email), slog.Any("error", ctx.Err()))
		return nil, ctx.Err()
	default:
	}

	var gormUser GormUser
	if err := r.db.Where("email = ?", email).First(&gormUser).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger.Warn(fmt.Sprintf("user not found with email: %s", email))
			return nil, ErrUserNotFound
		}
		r.logger.Error(fmt.Sprintf("failed to get user with email: %s", email), slog.Any("error", err))
		return nil, err
	}

	r.logger.Info(fmt.Sprintf("user fetched successfully with email: %s", email))
	return convertToProtoUser(&gormUser), nil
}

// UpdateUser обновляет информацию о пользователе
func (r *PostgresRepository) UpdateUser(ctx context.Context, user *user.User) (*user.User, error) {
	// Проверка отмены контекста
	select {
	case <-ctx.Done():
		r.logger.Error(fmt.Sprintf("UpdateUser operation canceled for user ID: %d", user.Id), slog.Any("error", ctx.Err()))
		return nil, ctx.Err()
	default:
	}

	// Преобразование proto-структуры в GORM-структуру
	gormUser := &GormUser{
		ID:       uint(user.Id),
		Username: user.Username,
		Email:    user.Email,
		Pwdhash:  user.Pwdhash,
		Salt:     user.Salt,
	}

	// Проверка существования пользователя перед обновлением
	var existingUser GormUser
	if err := r.db.First(&existingUser, gormUser.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger.Warn(fmt.Sprintf("user not found with ID: %d", user.Id))
			return nil, ErrUserNotFound
		}
		r.logger.Error(fmt.Sprintf("failed to check user existence for user ID: %d", user.Id), slog.Any("error", err))
		return nil, err
	}

	// Выполнение обновления
	result := r.db.Model(&existingUser).Updates(gormUser)
	if result.Error != nil {
		r.logger.Error(fmt.Sprintf("failed to update user with ID: %d", user.Id), slog.Any("error", result.Error))
		return nil, result.Error
	}

	r.logger.Info(fmt.Sprintf("user updated successfully with ID: %d", user.Id))
	updatedUser := convertToProtoUser(&existingUser)
	return updatedUser, nil
}

// DeleteUser удаляет пользователя по ID
func (r *PostgresRepository) DeleteUser(ctx context.Context, id uint) error {
	// Проверка отмены контекста
	select {
	case <-ctx.Done():
		r.logger.Error(fmt.Sprintf("DeleteUser operation canceled for user ID: %d", id), slog.Any("error", ctx.Err()))
		return ctx.Err()
	default:
	}

	var existingUser GormUser
	if err := r.db.First(&existingUser, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger.Warn(fmt.Sprintf("user not found with ID: %d", id))
			return ErrUserNotFound
		}
		r.logger.Error(fmt.Sprintf("failed to check user existence for user ID: %d", id), slog.Any("error", err))
		return err
	}

	// Выполнение удаления
	result := r.db.Delete(&existingUser)
	if result.Error != nil {
		r.logger.Error(fmt.Sprintf("failed to delete user with ID: %d", id), slog.Any("error", result.Error))
		return result.Error
	}

	r.logger.Info(fmt.Sprintf("user deleted successfully with ID: %d", id))
	return nil
}

// convertToProtoUser преобразует GormUser в User для возврата из репозитория
func convertToProtoUser(gormUser *GormUser) *user.User {
	return &user.User{
		Id:        int64(gormUser.ID),
		Username:  gormUser.Username,
		Email:     gormUser.Email,
		Pwdhash:   gormUser.Pwdhash,
		Salt:      gormUser.Salt,
		CreatedAt: gormUser.CreatedAt.Format(time.RFC3339), // Форматируем в RFC3339
		UpdatedAt: gormUser.UpdatedAt.Format(time.RFC3339), // Форматируем в RFC3339
	}
}
