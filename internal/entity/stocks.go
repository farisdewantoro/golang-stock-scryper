package entity

import (
	"time"

	"gorm.io/gorm"
)

type Stock struct {
	ID        uint           `gorm:"primaryKey"`
	Code      string         `gorm:"not null"`
	Name      string         `gorm:"not null"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
