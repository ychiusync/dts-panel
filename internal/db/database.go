package db

import (
	"fmt"
	"os"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

// Init 初始化 SQLite 数据库
func Init(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	dsn := fmt.Sprintf("%s/dts.db?cache=shared&_pragma=busy_timeout(5000)", dataDir)
	var err error
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}

	sqlDB, _ := DB.DB()
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	DB.Exec("PRAGMA journal_mode=WAL")
	DB.Exec("PRAGMA synchronous=NORMAL")

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
