package mod

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Mod 模组信息
type Mod struct {
	ID          int64     `json:"id"`
	ModID       string    `json:"mod_id"`
	ModName     string    `json:"mod_name"`
	ModURL      string    `json:"mod_url"`
	Enabled     bool      `json:"enabled"`
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Manager 模组管理器
type Manager struct {
	db           *sql.DB
	modDir       string
	instanceRoot string
}

func NewManager(db *sql.DB, modDir, instanceRoot string) *Manager {
	return &Manager{
		db:           db,
		modDir:       modDir,
		instanceRoot: instanceRoot,
	}
}

// Add 添加模组
func (m *Manager) Add(modID, modName, modURL string) (*Mod, error) {
	var count int
	_ = m.db.QueryRow("SELECT COUNT(*) FROM mods WHERE mod_id=?", modID).Scan(&count)
	if count > 0 {
		return m.Get(modID)
	}

	result, err := m.db.Exec(
		"INSERT INTO mods (mod_id, mod_name, mod_url, enabled) VALUES (?, ?, ?, ?)",
		modID, modName, modURL, 1)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	log.Printf("[mod] 添加模组: %s (id=%d)", modID, id)
	return m.Get(modID)
}

// Get 获取模组
func (m *Manager) Get(modID string) (*Mod, error) {
	var mod Mod
	var instAt, updAt string
	err := m.db.QueryRow(
		"SELECT id, mod_id, mod_name, mod_url, enabled, installed_at, updated_at FROM mods WHERE mod_id=?",
		modID).Scan(&mod.ID, &mod.ModID, &mod.ModName, &mod.ModURL, &mod.Enabled, &instAt, &updAt)
	if err != nil {
		return nil, err
	}
	mod.InstalledAt, _ = time.Parse("2006-01-02 15:04:05", instAt)
	mod.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updAt)
	return &mod, nil
}

// List 列出所有模组
func (m *Manager) List() ([]*Mod, error) {
	rows, err := m.db.Query(
		"SELECT id, mod_id, mod_name, mod_url, enabled, installed_at, updated_at FROM mods ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mods []*Mod
	for rows.Next() {
		var m Mod
		var instAt, updAt string
		if err := rows.Scan(&m.ID, &m.ModID, &m.ModName, &m.ModURL, &m.Enabled, &instAt, &updAt); err != nil {
			return nil, err
		}
		m.InstalledAt, _ = time.Parse("2006-01-02 15:04:05", instAt)
		m.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updAt)
		mods = append(mods, &m)
	}
	return mods, nil
}

// Enable 启用/禁用模组
func (m *Manager) Enable(modID string, enabled bool) error {
	_, err := m.db.Exec(
		"UPDATE mods SET enabled=?, updated_at=datetime('now') WHERE mod_id=?",
		boolToInt(enabled), modID)
	return err
}

// Delete 删除模组
func (m *Manager) Delete(modID string) error {
	_, _ = m.db.Exec("DELETE FROM instance_mods WHERE mod_id=?", modID)
	_, err := m.db.Exec("DELETE FROM mods WHERE mod_id=?", modID)
	return err
}

// LinkToInstance 将模组绑定到实例
func (m *Manager) LinkToInstance(instanceID int64, modID string, enabled bool) error {
	mod, err := m.Get(modID)
	if err != nil {
		return fmt.Errorf("模组 %s 不存在: %w", modID, err)
	}

	var count int
	_ = m.db.QueryRow(
		"SELECT COUNT(*) FROM instance_mods WHERE instance_id=? AND mod_id=?",
		instanceID, modID).Scan(&count)

	if count > 0 {
		_, err = m.db.Exec(
			"UPDATE instance_mods SET enabled=? WHERE instance_id=? AND mod_id=?",
			boolToInt(enabled), instanceID, modID)
	} else {
		_, err = m.db.Exec(
			"INSERT INTO instance_mods (instance_id, mod_id, enabled) VALUES (?, ?, ?)",
			instanceID, modID, boolToInt(enabled))
	}
	if err != nil {
		return err
	}

	if enabled {
		if err := m.installModToInstance(instanceID, mod); err != nil {
			log.Printf("[mod] 安装模组到实例失败: %v", err)
		}
	}

	log.Printf("[mod] 绑定模组 %s 到实例 %d", modID, instanceID)
	return nil
}

// UnlinkFromInstance 解除实例的模组绑定
func (m *Manager) UnlinkFromInstance(instanceID int64, modID string) error {
	_, err := m.db.Exec("DELETE FROM instance_mods WHERE instance_id=? AND mod_id=?", instanceID, modID)
	return err
}

// GetInstanceMods 获取实例的所有模组
func (m *Manager) GetInstanceMods(instanceID int64) ([]*Mod, error) {
	rows, err := m.db.Query(
		"SELECT m.id, m.mod_id, m.mod_name, m.mod_url, m.enabled, "+
			"COALESCE(im.enabled, m.enabled), m.installed_at, m.updated_at "+
			"FROM mods m LEFT JOIN instance_mods im ON m.mod_id = im.mod_id AND im.instance_id = ? "+
			"WHERE im.instance_id = ? OR m.id IN (SELECT mod_id FROM instance_mods WHERE instance_id = ?) "+
			"ORDER BY m.mod_name", instanceID, instanceID, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mods []*Mod
	for rows.Next() {
		var mod Mod
		var instAt, updAt string
		var instEnabled sql.NullInt64
		if err := rows.Scan(&mod.ID, &mod.ModID, &mod.ModName, &mod.ModURL,
			&mod.Enabled, &instEnabled, &instAt, &updAt); err != nil {
			return nil, err
		}
		if instEnabled.Valid {
			mod.Enabled = instEnabled.Int64 == 1
		}
		mod.InstalledAt, _ = time.Parse("2006-01-02 15:04:05", instAt)
		mod.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updAt)
		mods = append(mods, &mod)
	}
	return mods, nil
}

// installModToInstance 将模组安装到实例 mods 目录
func (m *Manager) installModToInstance(instanceID int64, mod *Mod) error {
	// 获取实例名称
	var name string
	err := m.db.QueryRow("SELECT name FROM instances WHERE id=?", instanceID).Scan(&name)
	if err != nil {
		return err
	}

	instModDir := filepath.Join(m.instanceRoot, name, "mods", mod.ModID)
	if err := os.MkdirAll(instModDir, 0755); err != nil {
		return err
	}

	// 写入模组标识文件
	manifest := fmt.Sprintf("mod_id=%s\nmod_name=%s\nmod_url=%s\n", mod.ModID, mod.ModName, mod.ModURL)
	if err := os.WriteFile(filepath.Join(instModDir, "manifest.txt"), []byte(manifest), 0644); err != nil {
		return err
	}

	log.Printf("[mod] 模组 %s 安装到实例 %s", mod.ModID, name)
	return nil
}

// DownloadMod 从 Steam Workshop 下载模组
func (m *Manager) DownloadMod(steamCMDPath, modID string) error {
	log.Printf("[mod] 下载 Steam Workshop 模组: %s", modID)

	downloadDir := filepath.Join(m.modDir, modID)
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return err
	}

	cmd := exec.Command(steamCMDPath,
		"+login", "anonymous",
		"+workshop_download_item", "108730", modID,
		"+quit")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GenerateModIndex 为实例生成 mod_index.dat
func (m *Manager) GenerateModIndex(instanceID int64, instanceName string) error {
	modDir := filepath.Join(m.instanceRoot, instanceName, "mods")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		return err
	}

	mods, err := m.GetInstanceMods(instanceID)
	if err != nil {
		return err
	}

	var lines []string
	for _, mod := range mods {
		if mod.Enabled {
			lines = append(lines, mod.ModID)
		}
	}

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(modDir, "mod_index.dat"), []byte(content), 0644); err != nil {
		return err
	}
	log.Printf("[mod] 为实例 %s 生成 mod_index.dat (%d 个启用模组)", instanceName, len(lines))
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
