package db

import (
	"encoding/json"
	"time"
)

func (r *Room) MarshalJSON() ([]byte, error) {
	type Alias Room
	return json.Marshal(&struct {
		CreatedAt interface{} `json:"created_at"`
		*Alias
	}{
		CreatedAt: nullTime(r.CreatedAt),
		Alias:     (*Alias)(r),
	})
}

func (m *Mod) MarshalJSON() ([]byte, error) {
	type Alias Mod
	return json.Marshal(&struct {
		InstalledAt interface{} `json:"installed_at"`
		UpdatedAt   interface{} `json:"updated_at"`
		*Alias
	}{
		InstalledAt: nullTime(m.InstalledAt),
		UpdatedAt:   nullTime(m.UpdatedAt),
		Alias:       (*Alias)(m),
	})
}

func (i *Instance) MarshalJSON() ([]byte, error) {
	type Alias Instance
	return json.Marshal(&struct {
		CreatedAt interface{} `json:"created_at"`
		UpdatedAt interface{} `json:"updated_at"`
		*Alias
	}{
		CreatedAt: nullTime(i.CreatedAt),
		UpdatedAt: nullTime(i.UpdatedAt),
		Alias:     (*Alias)(i),
	})
}

func nullTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t.Format("2006-01-02 15:04:05")
}
