package room

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"
)

// Room 房间配置
type Room struct {
	ID              int64     `json:"id"`
	InstanceID      int64     `json:"instance_id"`
	Name            string    `json:"name"`
	WorldName       string    `json:"world_name"`
	WorldGenOptions RoomGen   `json:"world_gen_options"`
	Description     string    `json:"description"`
	MaxPlayers      int       `json:"max_players"`
	Seed            string    `json:"seed"`
	Autopause       bool      `json:"autopause"`
	AllowTransfer   bool      `json:"allow_transfer"`
	Paused          bool      `json:"paused"`
	CreatedAt       time.Time `json:"created_at"`
}

// RoomGen 世界生成选项
type RoomGen struct {
	MapSeed     string `json:"map_seed"`
	MapSize     string `json:"map_size"`
	NoSpiders   bool   `json:"no_spiders"`
	NoCave      bool   `json:"no_cave"`
	NoCaveBiome bool   `json:"no_cave_biome"`

	DayLength    string `json:"day_length"`
	Season       string `json:"season"`
	WinterLength string `json:"winter_length"`
	Difficulty   string `json:"difficulty"`

	Trees      bool `json:"trees"`
	Rock       bool `json:"rock"`
	Mushrooms  bool `json:"mushrooms"`
	Fish       bool `json:"fish"`
	AutoSave   bool `json:"auto_save"`
	NoBuildPurge bool `json:"no_build_purge"`
}

// Manager 房间管理器
type Manager struct {
	db *sql.DB
}

func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// Create 创建/更新房间配置
func (m *Manager) Create(room *Room) error {
	optsJSON, _ := json.Marshal(room.WorldGenOptions)

	if room.ID == 0 {
		result, err := m.db.Exec(
			"INSERT INTO rooms (instance_id, name, world_name, description, world_gen_options, "+
				"max_players, seed, autopause, allow_transfer, paused) "+
				"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			room.InstanceID, room.Name, room.WorldName, room.Description,
			string(optsJSON), room.MaxPlayers, room.Seed,
			boolToInt(room.Autopause), boolToInt(room.AllowTransfer), boolToInt(room.Paused))
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		room.ID = id
		log.Printf("[room] 创建房间: %s (instance=%d)", room.Name, room.InstanceID)
	} else {
		_, err := m.db.Exec(
			"UPDATE rooms SET name=?, world_name=?, description=?, world_gen_options=?, "+
				"max_players=?, seed=?, autopause=?, allow_transfer=?, paused=? "+
				"WHERE id=?",
			room.Name, room.WorldName, room.Description, string(optsJSON),
			room.MaxPlayers, room.Seed,
			boolToInt(room.Autopause), boolToInt(room.AllowTransfer), boolToInt(room.Paused),
			room.ID)
		if err != nil {
			return err
		}
		log.Printf("[room] 更新房间: %s", room.Name)
	}
	return nil
}

// Get 获取房间
func (m *Manager) Get(id int64) (*Room, error) {
	var r Room
	var optsJSON, createdAt string
	err := m.db.QueryRow(
		"SELECT id, instance_id, name, world_name, description, world_gen_options, "+
			"max_players, seed, autopause, allow_transfer, paused, created_at "+
			"FROM rooms WHERE id=?", id).Scan(
		&r.ID, &r.InstanceID, &r.Name, &r.WorldName, &r.Description, &optsJSON,
		&r.MaxPlayers, &r.Seed, &r.Autopause, &r.AllowTransfer, &r.Paused, &createdAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(optsJSON), &r.WorldGenOptions)
	r.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &r, nil
}

// ListByInstance 按实例列出房间
func (m *Manager) ListByInstance(instanceID int64) ([]*Room, error) {
	rows, err := m.db.Query(
		"SELECT id, instance_id, name, world_name, description, world_gen_options, "+
			"max_players, seed, autopause, allow_transfer, paused, created_at "+
			"FROM rooms WHERE instance_id=? ORDER BY created_at DESC", instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []*Room
	for rows.Next() {
		var r Room
		var optsJSON, createdAt string
		if err := rows.Scan(&r.ID, &r.InstanceID, &r.Name, &r.WorldName, &r.Description, &optsJSON,
			&r.MaxPlayers, &r.Seed, &r.Autopause, &r.AllowTransfer, &r.Paused, &createdAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(optsJSON), &r.WorldGenOptions)
		r.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		rooms = append(rooms, &r)
	}
	return rooms, nil
}

// Delete 删除房间
func (m *Manager) Delete(id int64) error {
	_, err := m.db.Exec("DELETE FROM rooms WHERE id=?", id)
	return err
}

// DefaultRoom 默认房间
func DefaultRoom(instanceID int64) *Room {
	return &Room{
		InstanceID:    instanceID,
		Name:          "Default World",
		WorldName:     "Default World",
		MaxPlayers:    10,
		Autopause:     true,
		AllowTransfer: true,
		WorldGenOptions: RoomGen{
			MapSize:    "large",
			Difficulty: "normal",
			DayLength:  "medium",
		},
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
