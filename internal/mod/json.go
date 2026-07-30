package mod

import (
	"encoding/json"
	"time"
)

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

func nullTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t.Format("2006-01-02 15:04:05")
}
