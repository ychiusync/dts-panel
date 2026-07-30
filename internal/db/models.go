package db

import "time"

type Instance struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string    `gorm:"uniqueIndex;not null" json:"name"`
	Status       string    `gorm:"default:'stopped'" json:"status"`
	MasterPort   int       `json:"master_port"`
	ClusterPort  int       `json:"cluster_port"`
	MaxPlayers   int       `gorm:"default:10" json:"max_players"`
	ServerToken  string    `gorm:"column:server_token" json:"-"`
	ConfigDir    string    `json:"config_dir"`
	LogDir       string    `json:"log_dir"`
	GameRoot     string    `gorm:"-" json:"game_root"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Room struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	InstanceID      int64     `gorm:"index" json:"instance_id"`
	Name            string    `gorm:"not null" json:"name"`
	WorldName       string    `json:"world_name"`
	Description     string    `json:"description"`
	WorldGenOptions string    `gorm:"type:text" json:"world_gen_options"`
	MaxPlayers      int       `json:"max_players"`
	Seed            string    `json:"seed"`
	Autopause       bool      `gorm:"default:true" json:"autopause"`
	AllowTransfer   bool      `gorm:"default:true" json:"allow_transfer"`
	Paused          bool      `json:"paused"`
	CreatedAt       time.Time `json:"created_at"`
}

type Mod struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ModID       string    `gorm:"uniqueIndex;not null" json:"mod_id"`
	ModName     string    `json:"mod_name"`
	ModURL      string    `json:"mod_url"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	InstanceIDs string    `gorm:"type:text;column:instance_ids" json:"-"`
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SystemLog struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	InstanceID int64     `gorm:"index;default:0" json:"instance_id"`
	Level      string    `gorm:"default:'info'" json:"level"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"created_at"`
}
