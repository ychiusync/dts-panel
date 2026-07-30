package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	DataDir          string `json:"data_dir"`
	GameInstallDir   string `json:"game_install_dir"`
	InstanceRoot     string `json:"instance_root"`
	PanelHost        string `json:"panel_host"`
	PanelPort        int    `json:"panel_port"`
	DefaultMasterPort int   `json:"default_master_port"`
	DefaultClusterPort int   `json:"default_cluster_port"`
	DefaultMaxPlayers int    `json:"default_max_players"`
	SteamCMDPath     string `json:"steamcmd_path"`
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	if home == "" { home = "/root" }
	return &Config{
		DataDir:          filepath.Join(home, ".dts-panel"),
		GameInstallDir:   filepath.Join(home, "dst-server"),
		InstanceRoot:     filepath.Join(home, ".dts-panel", "instances"),
		PanelHost:        "0.0.0.0",
		PanelPort:        8090,
		DefaultMasterPort: 10999,
		DefaultClusterPort: 11000,
		DefaultMaxPlayers: 10,
		SteamCMDPath:     filepath.Join(home, ".dts-panel", "steamcmd", "steamcmd.sh"),
	}
}

func Load() *Config {
	c := DefaultConfig()
	data, _ := os.ReadFile(filepath.Join(c.DataDir, "config.json"))
	_ = json.Unmarshal(data, c)
	return c
}
