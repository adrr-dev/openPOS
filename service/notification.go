package service

import (
	"context"
	"fmt"

	"github.com/adrr-dev/openPOS/backend/model"
)

type NotificationRepository interface {
	Create(ctx context.Context, n *model.Notification) error
	List(ctx context.Context, storeID uint, page, limit int, unreadOnly bool) ([]*model.Notification, int, error)
	GetByID(ctx context.Context, storeID, id uint) (*model.Notification, error)
	MarkAsRead(ctx context.Context, storeID, id uint) error
	MarkAllAsRead(ctx context.Context, storeID uint) error
	Delete(ctx context.Context, storeID, id uint) error
	HasUnreadForReference(ctx context.Context, storeID uint, referenceID uint, notifType string) (bool, error)
}

type NotificationService struct {
	notifRepo NotificationRepository
}

func NewNotificationService(notifRepo NotificationRepository) *NotificationService {
	return &NotificationService{notifRepo: notifRepo}
}

type NotificationPage struct {
	Items []*model.Notification `json:"items"`
	Total int                   `json:"total"`
	Page  int                   `json:"page"`
	Limit int                   `json:"limit"`
}

func (s *NotificationService) List(ctx context.Context, storeID uint, page, limit int, unreadOnly bool) (*NotificationPage, error) {
	items, total, err := s.notifRepo.List(ctx, storeID, page, limit, unreadOnly)
	if err != nil {
		return nil, err
	}
	return &NotificationPage{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

func (s *NotificationService) MarkAsRead(ctx context.Context, storeID, id uint) error {
	return s.notifRepo.MarkAsRead(ctx, storeID, id)
}

func (s *NotificationService) MarkAllAsRead(ctx context.Context, storeID uint) error {
	return s.notifRepo.MarkAllAsRead(ctx, storeID)
}

func (s *NotificationService) Delete(ctx context.Context, storeID, id uint) error {
	return s.notifRepo.Delete(ctx, storeID, id)
}

func (s *NotificationService) Create(ctx context.Context, storeID uint, title, message string, notifType model.NotificationType, refID *uint) error {
	n := &model.Notification{
		StoreID:     storeID,
		Title:       title,
		Message:     message,
		Type:        notifType,
		Read:        false,
		ReferenceID: refID,
	}
	return s.notifRepo.Create(ctx, n)
}

func (s *NotificationService) CheckAndNotifyLowStock(ctx context.Context, storeID uint, productID uint, productName string, currentStock int, unit string) error {
	if currentStock > 5 {
		return nil
	}
	exists, err := s.notifRepo.HasUnreadForReference(ctx, storeID, productID, string(model.NotificationLowStock))
	if err != nil || exists {
		return err
	}
	title := "Stok Menipis"
	message := fmt.Sprintf("Produk %s tersisa %d %s.", productName, currentStock, unit)
	return s.Create(ctx, storeID, title, message, model.NotificationLowStock, &productID)
}
