package repository

import (
	"time"
)

// GormUser представляет модель пользователя в базе данных
type GormUser struct {
	ID        uint      `gorm:"primaryKey"`      // Уникальный идентификатор пользователя
	Username  string    `gorm:"unique;not null"` // Имя пользователя (уникальное)
	Email     string    `gorm:"unique;not null"` // Электронная почта (уникальная)
	Pwdhash   string    `gorm:"not null"`        // Хеш пароля
	Salt      string    `gorm:"not null"`        // Соль для хеширования пароля
	CreatedAt time.Time `gorm:"autoCreateTime"`  // Дата создания
	UpdatedAt time.Time `gorm:"autoUpdateTime"`  // Дата обновления
}

// TableName указывает GORM использовать имя таблицы "users"
func (GormUser) TableName() string {
	return "user"
}
