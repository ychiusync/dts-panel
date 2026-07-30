package room

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

func nullTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t.Format("2006-01-02 15:04:05")
}
