// Package postgres 封装 PostgreSQL 的 GORM 模型
// 这里仅放与 DB 映射相关的结构体与最小数据操作。
package postgres

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// User 表模型（对应 PostgreSQL 的 public."user"）
// 注意：表名为关键字，GORM 会自动加引号；TableName() 显式返回 "user"
type User struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"` // 主键
	Name      string    `gorm:"type:varchar(255);not null"`
	Age       int       `gorm:"not null;check:age >= 0 AND age <= 150"`
	Email     string    `gorm:"type:varchar(255);not null"` // 是否唯一看你后续需求
	TagsJSON  string    `gorm:"type:text"`                  // 简化处理：示例里把 tags 序列化成 JSON 存 text（下一步可换 JSONB）
	CreatedAt time.Time `gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time `gorm:"not null;autoUpdateTime"`
}

func (User) TableName() string { return "user" }

// UserModel 封装与 User 表相关的最小操作集合
type UserModel struct {
	db *gorm.DB
}

// NewUserModel 通过注入的 *gorm.DB 创建模型实例
func NewUserModel(db *gorm.DB) *UserModel { return &UserModel{db: db} }

// Create 新增用户（要求 ctx 带上链路追踪；调用方需确保字段已校验）
func (m *UserModel) Create(ctx context.Context, u *User) error {
	return m.db.WithContext(ctx).Create(u).Error
}
