package db

import (
	"fmt"
	"os"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Init 初始化 SQLite 数据库
func Init(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	dsn := fmt.Sprintf("%s/dts.db", dataDir)
	var err error
	DB, err = gorm.Open(Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("gorm open failed: %w", err)
	}

	sqlDB, _ := DB.DB()
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return nil
}

// AutoMigrate 自动迁移所有表
func AutoMigrate() error {
	return DB.AutoMigrate(
		&Instance{},
		&Room{},
		&Mod{},
		&SystemLog{},
	)
}
