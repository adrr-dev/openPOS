package model

type NotificationCategory string

const (
	CategoryStok      NotificationCategory = "stok"
	CategoryTransaksi NotificationCategory = "transaksi"
	CategorySistem    NotificationCategory = "sistem"
)

type Notification struct {
	Model
	StoreID       uint                 `gorm:"not null;index" json:"store_id"`
	Title         string               `gorm:"not null" json:"title"`
	Message       string               `gorm:"not null" json:"message"`
	Category      NotificationCategory `gorm:"not null;default:'sistem'" json:"category"`
	Type          string               `gorm:"not null;default:'info'" json:"type"`
	ActorID       string               `gorm:"not null;default:''" json:"actor_id,omitempty"`
	ActorName     string               `gorm:"not null;default:''" json:"actor_name,omitempty"`
	ReferenceType string               `gorm:"not null;default:''" json:"reference_type,omitempty"`
	ReferenceID   string               `gorm:"not null;default:''" json:"reference_id,omitempty"`
	Read          bool                 `gorm:"not null;default:false" json:"read"`
}

func (Notification) TableName() string {
	return "notifications"
}
