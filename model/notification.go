package model

type NotificationType string

const (
	NotificationInfo    NotificationType = "info"
	NotificationWarning NotificationType = "warning"
	NotificationAlert   NotificationType = "alert"
	NotificationLowStock NotificationType = "low_stock"
)

type Notification struct {
	Model
	StoreID     uint             `gorm:"not null;index" json:"store_id"`
	Title       string           `gorm:"not null" json:"title"`
	Message     string           `gorm:"not null" json:"message"`
	Type        NotificationType `gorm:"not null;default:'info'" json:"type"`
	Read        bool             `gorm:"not null;default:false" json:"read"`
	ReferenceID *uint            `json:"reference_id,omitempty"`
}

func (Notification) TableName() string {
	return "notifications"
}
