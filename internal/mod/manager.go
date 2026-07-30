package mod

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"gorm.io/gorm"
	"github.com/dts-panel/dts-panel/internal/db"
)

type Manager struct {
	db           *gorm.DB
	modDir       string
	instanceRoot string
}

func NewManager(gormDB *gorm.DB, modDir, instanceRoot string) *Manager {
	return &Manager{db: gormDB, modDir: modDir, instanceRoot: instanceRoot}
}

func (m *Manager) Add(modID, modName, modURL string) (*db.Mod, error) {
	var mod db.Mod
	if err := m.db.Where("mod_id = ?", modID).First(&mod).Error; err == nil {
		return &mod, nil
	}

	mod = db.Mod{
		ModID:       modID,
		ModName:     modName,
		ModURL:      modURL,
		Enabled:     true,
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := m.db.Create(&mod).Error; err != nil {
		return nil, err
	}
	log.Printf("[mod] 添加模组: %s (id=%d)", modID, mod.ID)
	return &mod, nil
}

func (m *Manager) List() ([]*db.Mod, error) {
	var mods []*db.Mod
	return mods, m.db.Order("id ASC").Find(&mods).Error
}

func (m *Manager) Get(modID string) (*db.Mod, error) {
	var mod db.Mod
	return &mod, m.db.Where("mod_id = ?", modID).First(&mod).Error
}

func (m *Manager) Enable(modID string, enabled bool) error {
	return m.db.Model(&db.Mod{}).Where("mod_id = ?", modID).
		Updates(map[string]interface{}{"enabled": enabled, "updated_at": time.Now()}).Error
}

func (m *Manager) Delete(modID string) error {
	m.db.Where("mod_id = ?", modID).Delete(&db.Mod{})
	return nil
}

func (m *Manager) LinkToInstance(instanceID int64, modID string, enabled bool) error {
	mod, err := m.Get(modID)
	if err != nil {
		return fmt.Errorf("模组 %s 不存在: %w", modID, err)
	}

	// 从 InstanceIDs 中解析已有列表
	var instIDs []int64
	if mod.InstanceIDs != "" {
		json.Unmarshal([]byte(mod.InstanceIDs), &instIDs)
	}

	// 检查是否已绑定
	found := false
	for _, id := range instIDs {
		if id == instanceID {
			found = true
			break
		}
	}

	if !found {
		instIDs = append(instIDs, instanceID)
	}

	data, _ := json.Marshal(instIDs)
	if err := m.db.Model(&db.Mod{}).Where("mod_id = ?", modID).Update("instance_ids", string(data)).Error; err != nil {
		return err
	}

	// 安装到实例目录
	if enabled {
		m.installMod(mod, instanceID)
	}

	log.Printf("[mod] 绑定模组 %s 到实例 %d", modID, instanceID)
	return nil
}

func (m *Manager) UnlinkFromInstance(instanceID int64, modID string) error {
	mod, err := m.Get(modID)
	if err != nil {
		return err
	}

	var instIDs []int64
	if mod.InstanceIDs != "" {
		json.Unmarshal([]byte(mod.InstanceIDs), &instIDs)
	}

	newIDs := make([]int64, 0)
	for _, id := range instIDs {
		if id != instanceID {
			newIDs = append(newIDs, id)
		}
	}

	data, _ := json.Marshal(newIDs)
	return m.db.Model(&db.Mod{}).Where("mod_id = ?", modID).Update("instance_ids", string(data)).Error
}

func (m *Manager) GetInstanceMods(instanceID int64) ([]*db.Mod, error) {
	var mods []*db.Mod
	var allMods []*db.Mod
	_ = m.db.Find(&allMods).Error

	for _, mod := range allMods {
		var instIDs []int64
		if mod.InstanceIDs != "" {
			_ = json.Unmarshal([]byte(mod.InstanceIDs), &instIDs)
		}
		for _, id := range instIDs {
			if id == instanceID {
				mods = append(mods, mod)
				break
			}
		}
	}
	return mods, nil
}

func (m *Manager) installMod(mod *db.Mod, instanceID int64) {
	// 获取实例名称
	var name string
	_ = m.db.Raw("SELECT name FROM instances WHERE id = ?", instanceID).Scan(&name).Error
	instModDir := filepath.Join(m.instanceRoot, name, "mods", mod.ModID)
	_ = os.MkdirAll(instModDir, 0755)

	manifest := fmt.Sprintf("mod_id=%s\nmod_name=%s\nmod_url=%s\n", mod.ModID, mod.ModName, mod.ModURL)
	_ = os.WriteFile(filepath.Join(instModDir, "manifest.txt"), []byte(manifest), 0644)
	log.Printf("[mod] 模组 %s 安装到实例 %s", mod.ModID, name)
}

func (m *Manager) DownloadMod(steamCMDPath, modID string) error {
	downloadDir := filepath.Join(m.modDir, modID)
	_ = os.MkdirAll(downloadDir, 0755)

	cmd := exec.Command(steamCMDPath, "+login", "anonymous", "+workshop_download_item", "108730", modID, "+quit")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (m *Manager) GenerateModIndex(instanceID int64, instanceName string) error {
	modDir := filepath.Join(m.instanceRoot, instanceName, "mods")
	_ = os.MkdirAll(modDir, 0755)

	mods, _ := m.GetInstanceMods(instanceID)
	var lines []string
	for _, m := range mods {
		if m.Enabled {
			lines = append(lines, m.ModID)
		}
	}

	content := ""
	if len(lines) > 0 {
		content = joinLines(lines) + "\n"
	}
	return os.WriteFile(filepath.Join(modDir, "mod_index.dat"), []byte(content), 0644)
}

func joinLines(s []string) string {
	if len(s) == 0 {
		return ""
	}
	result := s[0]
	for _, str := range s[1:] {
		result = result + "\n" + str
	}
	return result
}
