package model

type ActivityLog struct {
	Model
	StoreID       uint   `gorm:"not null;index" json:"store_id"`
	ActorID       string `gorm:"not null;default:''" json:"actor_id"`
	ActorName     string `gorm:"not null;default:''" json:"actor_name"`
	Action        string `gorm:"not null;index" json:"action"`
	Detail        string `gorm:"not null" json:"detail"`
	ReferenceType string `gorm:"not null;default:''" json:"reference_type,omitempty"`
	ReferenceID   string `gorm:"not null;default:''" json:"reference_id,omitempty"`
}

func (ActivityLog) TableName() string {
	return "activity_logs"
}
