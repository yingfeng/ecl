package entity

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type BaseModel struct {
	CreateTime *int64     `gorm:"column:create_time;index" json:"create_time,omitempty"`
	CreateDate *time.Time `gorm:"column:create_date;index" json:"create_date,omitempty"`
	UpdateTime *int64     `gorm:"column:update_time;index" json:"update_time,omitempty"`
	UpdateDate *time.Time `gorm:"column:update_date;index" json:"update_date,omitempty"`
}

func autoModelTime() (int64, time.Time) {
	now := time.Now()
	return now.UnixMilli(), now.Truncate(time.Second)
}

func (m *BaseModel) BeforeCreate(tx *gorm.DB) error {
	ts, dt := autoModelTime()
	if m.CreateTime == nil {
		m.CreateTime = &ts
	}
	if m.CreateDate == nil {
		m.CreateDate = &dt
	}
	if m.UpdateTime == nil {
		m.UpdateTime = &ts
	}
	if m.UpdateDate == nil {
		m.UpdateDate = &dt
	}
	return nil
}

func (m *BaseModel) BeforeUpdate(tx *gorm.DB) error {
	ts, dt := autoModelTime()
	m.UpdateTime = &ts
	m.UpdateDate = &dt
	return nil
}

type JSONMap map[string]interface{}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return json.Unmarshal([]byte(value.(string)), j)
	}
	return json.Unmarshal(b, j)
}

type JSONSlice []interface{}

func (j JSONSlice) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONSlice) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return json.Unmarshal([]byte(value.(string)), j)
	}
	return json.Unmarshal(b, j)
}
