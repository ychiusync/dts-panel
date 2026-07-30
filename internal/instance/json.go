package instance

import (
	"encoding/json"
	"time"
)

// MarshalJSON 自定义序列化，处理零值时间
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
