package postgres

import (
	"context"

	"gorm.io/gorm"
)

type HealthModel struct{ db *gorm.DB }

func NewHealthModel(db *gorm.DB) *HealthModel { return &HealthModel{db: db} }

func (m *HealthModel) Ping(ctx context.Context) error {
	var n int
	return m.db.WithContext(ctx).Raw("SELECT 1").Scan(&n).Error
}
