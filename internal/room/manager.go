package room

import (
	"log"

	"gorm.io/gorm"
	"github.com/dts-panel/dts-panel/internal/db"
)

type Manager struct{ db *gorm.DB }

func NewManager(gormDB *gorm.DB) *Manager { return &Manager{db: gormDB} }

func (m *Manager) Create(r *db.Room) error {
	if err := m.db.Create(r).Error; err != nil {
		return err
	}
	log.Printf("[room] 创建房间: %s (instance=%d)", r.Name, r.InstanceID)
	return nil
}

func (m *Manager) Get(id int64) (*db.Room, error) {
	var r db.Room
	return &r, m.db.First(&r, id).Error
}

func (m *Manager) List() ([]*db.Room, error) {
	var rooms []*db.Room
	return rooms, m.db.Order("created_at DESC").Find(&rooms).Error
}

func (m *Manager) ListByInstance(instanceID int64) ([]*db.Room, error) {
	var rooms []*db.Room
	return rooms, m.db.Where("instance_id = ?", instanceID).Order("created_at DESC").Find(&rooms).Error
}

func (m *Manager) Delete(id int64) error {
	return m.db.Where("id = ?", id).Delete(&db.Room{}).Error
}
