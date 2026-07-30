package db

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// Init 初始化 SQLite 数据库
func Init(dataDir string) (*sql.DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dataDir, "dts.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}

	// 启用 WAL 模式提高并发
	_, _ = db.Exec("PRAGMA journal_mode=WAL")
	_, _ = db.Exec("PRAGMA foreign_keys=ON")

	return db, nil
}

// Migrate 执行表创建/升级
func Migrate(db *sql.DB) error {
	statements := []string{
		// 实例表
		`CREATE TABLE IF NOT EXISTS instances (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			status TEXT DEFAULT 'stopped',
			master_port INTEGER,
			cluster_port INTEGER,
			max_players INTEGER DEFAULT 10,
			server_token TEXT,
			config_dir TEXT,
			log_dir TEXT,
			created_at DATETIME DEFAULT (datetime('now')),
			updated_at DATETIME DEFAULT (datetime('now'))
		)`,

		// 房间表（关联实例）
		`CREATE TABLE IF NOT EXISTS rooms (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			instance_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			world_name TEXT,
			world_gen_options TEXT,
			description TEXT,
			max_players INTEGER,
			seed TEXT,
			modlist_override TEXT,
			autopause INTEGER DEFAULT 1,
			allow_transfer INTEGER DEFAULT 1,
			paused INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT (datetime('now')),
			FOREIGN KEY (instance_id) REFERENCES instances(id)
		)`,

		// 模组表
		`CREATE TABLE IF NOT EXISTS mods (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mod_id TEXT NOT NULL,
			mod_name TEXT,
			mod_url TEXT,
			enabled INTEGER DEFAULT 1,
			installed_at DATETIME DEFAULT (datetime('now')),
			updated_at DATETIME DEFAULT (datetime('now'))
		)`,

		// 实例-模组 关联表
		`CREATE TABLE IF NOT EXISTS instance_mods (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			instance_id INTEGER NOT NULL,
			mod_id TEXT NOT NULL,
			enabled INTEGER DEFAULT 1,
			FOREIGN KEY (instance_id) REFERENCES instances(id),
			UNIQUE(instance_id, mod_id)
		)`,

		// 系统日志表
		`CREATE TABLE IF NOT EXISTS system_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			instance_id INTEGER,
			level TEXT DEFAULT 'info',
			message TEXT,
			created_at DATETIME DEFAULT (datetime('now')),
			FOREIGN KEY (instance_id) REFERENCES instances(id)
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
