package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/watchlist-kata/protos/user"
	"gorm.io/gorm"
	"log/slog"
	"time"
	"unicode/utf8"
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
		r.logger.Error("CreateUser operation canceled", slog.Any("error", ctx.Err()))
		return nil, ctx.Err()
	default:
	}

	// Валидация входных данные
	if err := r.validateUser(user); err != nil {
		r.logger.Error("failed to create user: invalid data", slog.Any("error", err))
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
		r.logger.Error("failed to begin transaction", slog.Any("error", tx.Error))
		return nil, tx.Error
	}

	// Проверка контекста перед созданием пользователя
	select {
	case <-ctx.Done():
		tx.Rollback()
		r.logger.Error("CreateUser operation canceled during transaction", slog.Any("error", ctx.Err()))
		return nil, ctx.Err()
	default:
	}

	if err := tx.Create(gormUser).Error; err != nil {
		tx.Rollback()
		r.logger.Error("failed to create user", slog.Any("error", err))
		return nil, err
	}

	// Проверка контекста после создания пользователя
	select {
	case <-ctx.Done():
		tx.Rollback()
		r.logger.Error("CreateUser operation canceled after user creation", slog.Any("error", ctx.Err()))
		return nil, ctx.Err()
	default:
	}

	if err := tx.Commit().Error; err != nil {
		r.logger.Error("failed to commit transaction", slog.Any("error", err))
		return nil, err
	}

	r.logger.Info("user created successfully", slog.Int64("user_id", int64(gormUser.ID)))
	return convertToProtoUser(gormUser), nil
}

// GetUserByID получает пользователя по ID
func (r *PostgresRepository) GetUserByID(ctx context.Context, id uint) (*user.User, error) {
	// Проверка отмены контекста
	select {
	case <-ctx.Done():
		r.logger.Error("GetUserByID operation canceled", slog.Any("error", ctx.Err()))
		return nil, ctx.Err()
	default:
	}

	var gormUser GormUser
	if err := r.db.First(&gormUser, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger.Warn("user not found", slog.Int("user_id", int(id)))
			return nil, ErrUserNotFound
		}
		r.logger.Error("failed to get user by ID", slog.Any("error", err), slog.Int("user_id", int(id)))
		return nil, err
	}

	r.logger.Info("user fetched successfully", slog.Int("user_id", int(id)))
	return convertToProtoUser(&gormUser), nil
}

// GetUserByUsername получает пользователя по имени пользователя
func (r *PostgresRepository) GetUserByUsername(ctx context.Context, username string) (*user.User, error) {
	// Проверка отмены контекста
	select {
	case <-ctx.Done():
		r.logger.Error("GetUserByUsername operation canceled", slog.Any("error", ctx.Err()))
		return nil, ctx.Err()
	default:
	}

	var gormUser GormUser
	if err := r.db.Where("username = ?", username).First(&gormUser).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger.Warn("user not found", slog.String("username", username))
			return nil, ErrUserNotFound
		}
		r.logger.Error("failed to get user by username", slog.Any("error", err), slog.String("username", username))
		return nil, err
	}

	r.logger.Info("user fetched successfully", slog.String("username", username))
	return convertToProtoUser(&gormUser), nil
}

// GetUserByEmail получает пользователя по электронной почте
func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (*user.User, error) {
	// Проверка отмены контекста
	select {
	case <-ctx.Done():
		r.logger.Error("GetUserByEmail operation canceled", slog.Any("error", ctx.Err()))
		return nil, ctx.Err()
	default:
	}

	var gormUser GormUser
	if err := r.db.Where("email = ?", email).First(&gormUser).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger.Warn("user not found", slog.String("email", email))
			return nil, ErrUserNotFound
		}
		r.logger.Error("failed to get user by email", slog.Any("error", err), slog.String("email", email))
		return nil, err
	}

	r.logger.Info("user fetched successfully", slog.String("email", email))
	return convertToProtoUser(&gormUser), nil
}

// UpdateUser обновляет информацию о пользователе
func (r *PostgresRepository) UpdateUser(ctx context.Context, user *user.User) (*user.User, error) {
	// Проверка отмены контекста
	select {
	case <-ctx.Done():
		r.logger.Error("UpdateUser operation canceled", slog.Any("error", ctx.Err()))
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
			r.logger.Warn("user not found", slog.Int("user_id", int(gormUser.ID)))
			return nil, ErrUserNotFound
		}
		r.logger.Error("failed to check user existence", slog.Any("error", err), slog.Int("user_id", int(gormUser.ID)))
		return nil, err
	}

	// Выполнение обновления
	result := r.db.Model(&existingUser).Updates(gormUser)
	if result.Error != nil {
		r.logger.Error("failed to update user", slog.Any("error", result.Error), slog.Int("user_id", int(gormUser.ID)))
		return nil, result.Error
	}

	updatedUser := convertToProtoUser(&existingUser)

	r.logger.Info("user updated successfully", slog.Int("user_id", int(gormUser.ID)))
	return updatedUser, nil
}

// DeleteUser удаляет пользователя по ID
func (r *PostgresRepository) DeleteUser(ctx context.Context, id uint) error {
	// Проверка отмены контекста
	select {
	case <-ctx.Done():
		r.logger.Error("DeleteUser operation canceled", slog.Any("error", ctx.Err()))
		return ctx.Err()
	default:
	}

	// Проверка существования пользователя перед удалением
	var existingUser GormUser
	if err := r.db.First(&existingUser, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger.Warn("user not found", slog.Int("user_id", int(id)))
			return ErrUserNotFound
		}
		r.logger.Error("failed to check user existence", slog.Any("error", err), slog.Int("user_id", int(id)))
		return err
	}

	// Выполнение удаления
	result := r.db.Delete(&existingUser)
	if result.Error != nil {
		r.logger.Error("failed to delete user", slog.Any("error", result.Error), slog.Int("user_id", int(id)))
		return result.Error
	}

	r.logger.Info("user deleted successfully", slog.Int("user_id", int(id)))
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
