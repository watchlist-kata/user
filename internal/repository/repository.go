package repository

import (
	"github.com/watchlist-kata/protos/user"
	"gorm.io/gorm"
	"time"
)

// Repository интерфейс для работы с пользователями
type Repository interface {
	CreateUser(user *user.User) error
	GetUserByID(id uint) (*user.User, error)
	GetUserByUsername(username string) (*user.User, error) // Метод для получения пользователя по имени
	GetUserByEmail(email string) (*user.User, error)       // Метод для получения пользователя по электронной почте
	UpdateUser(user *user.User) error
	DeleteUser(id uint) error
}

// PostgresRepository реализация репозитория с использованием GORM
type PostgresRepository struct {
	db *gorm.DB
}

// NewPostgresRepository создает новый экземпляр PostgresRepository
func NewPostgresRepository(db *gorm.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// CreateUser создает нового пользователя в базе данных
func (r *PostgresRepository) CreateUser(user *user.User) error {
	gormUser := &GormUser{
		Username: user.Username,
		Email:    user.Email,
		Pwdhash:  user.Pwdhash,
		Salt:     user.Salt,
	}
	return r.db.Create(gormUser).Error
}

// GetUserByID получает пользователя по ID
func (r *PostgresRepository) GetUserByID(id uint) (*user.User, error) {
	var gormUser GormUser
	if err := r.db.First(&gormUser, id).Error; err != nil {
		return nil, err
	}
	return convertToProtoUser(&gormUser), nil
}

// GetUserByUsername получает пользователя по имени пользователя
func (r *PostgresRepository) GetUserByUsername(username string) (*user.User, error) {
	var gormUser GormUser
	if err := r.db.Where("username = ?", username).First(&gormUser).Error; err != nil {
		return nil, err // Пользователь не найден или произошла ошибка
	}
	return convertToProtoUser(&gormUser), nil
}

// GetUserByEmail получает пользователя по электронной почте
func (r *PostgresRepository) GetUserByEmail(email string) (*user.User, error) {
	var gormUser GormUser
	if err := r.db.Where("email = ?", email).First(&gormUser).Error; err != nil {
		return nil, err // Пользователь не найден или произошла ошибка
	}
	return convertToProtoUser(&gormUser), nil
}

// UpdateUser обновляет информацию о пользователе
func (r *PostgresRepository) UpdateUser(user *user.User) error {
	gormUser := &GormUser{
		ID:       uint(user.Id),
		Username: user.Username,
		Email:    user.Email,
		Pwdhash:  user.Pwdhash,
		Salt:     user.Salt,
	}
	return r.db.Model(&GormUser{}).Where("id = ?", gormUser.ID).Updates(gormUser).Error
}

// DeleteUser удаляет пользователя по ID
func (r *PostgresRepository) DeleteUser(id uint) error {
	return r.db.Delete(&GormUser{}, id).Error
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
