package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config 全局配置
type Config struct {
	// 数据目录（存放 SQLite、实例目录等）
	DataDir string `json:"data_dir"`

	// 游戏安装根目录（SteamCMD 安装位置）
	GameInstallDir string `json:"game_install_dir"`

	// 实例根目录（每个实例独立）
	InstanceRoot string `json:"instance_root"`

	// Web 面板
	PanelHost string `json:"panel_host"`
	PanelPort int    `json:"panel_port"`

	// 默认实例配置
	DefaultMasterPort int `json:"default_master_port"`
	DefaultClusterPort int `json:"default_cluster_port"`
	DefaultMaxPlayers int `json:"default_max_players"`

	// SteamCMD
	SteamCMDPath string `json:"steamcmd_path"`
}

// DefaultConfig 默认值
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/root"
	}
	return &Config{
		DataDir:          filepath.Join(home, ".dts-panel"),
		GameInstallDir:   filepath.Join(home, "dst-server"),
		InstanceRoot:     filepath.Join(home, ".dts-panel", "instances"),
		PanelHost:        "0.0.0.0",
		PanelPort:        8080,
		DefaultMasterPort: 10999,
		DefaultClusterPort: 11000,
		DefaultMaxPlayers: 10,
		SteamCMDPath:     filepath.Join(home, ".dts-panel", "steamcmd", "steamcmd.sh"),
	}
}

// SavePath 配置文件路径
func (c *Config) SavePath() string {
	return filepath.Join(c.DataDir, "config.json")
}

// Save 保存配置到文件
func (c *Config) Save() error {
	if err := os.MkdirAll(c.DataDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.SavePath(), data, 0644)
}

// Load 加载配置，不存在则使用默认值
func Load() *Config {
	c := DefaultConfig()
	p := c.SavePath()
	data, err := os.ReadFile(p)
	if err != nil {
		return c
	}
	if err := json.Unmarshal(data, c); err != nil {
		return c
	}
	return c
}
