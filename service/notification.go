package service

import (
	"context"
	"fmt"

	"github.com/adrr-dev/openPOS/backend/model"
)

type NotificationRepository interface {
	Create(ctx context.Context, n *model.Notification) error
	List(ctx context.Context, storeID uint, category string, status string, unreadOnly bool, page, limit int) ([]*model.Notification, int, error)
	UnreadCount(ctx context.Context, storeID uint) (int64, error)
	GetByID(ctx context.Context, storeID, id uint) (*model.Notification, error)
	MarkAsRead(ctx context.Context, storeID, id uint) error
	MarkAllAsRead(ctx context.Context, storeID uint) error
	Delete(ctx context.Context, storeID, id uint) error
	HasUnreadForReference(ctx context.Context, storeID uint, referenceID string, notifType string) (bool, error)
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

func (s *NotificationService) List(ctx context.Context, storeID uint, category, status string, unreadOnly bool, page, limit int) (*NotificationPage, error) {
	items, total, err := s.notifRepo.List(ctx, storeID, category, status, unreadOnly, page, limit)
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

func (s *NotificationService) UnreadCount(ctx context.Context, storeID uint) (int64, error) {
	return s.notifRepo.UnreadCount(ctx, storeID)
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

func (s *NotificationService) Emit(ctx context.Context, storeID uint, title, message string, category model.NotificationCategory, notifType string, actorID, actorName, refType, refID string) error {
	n := &model.Notification{
		StoreID:       storeID,
		Title:         title,
		Message:       message,
		Category:      category,
		Type:          notifType,
		ActorID:       actorID,
		ActorName:     actorName,
		ReferenceType: refType,
		ReferenceID:   refID,
		Read:          false,
	}
	return s.notifRepo.Create(ctx, n)
}

func (s *NotificationService) CheckAndNotifyLowStock(ctx context.Context, storeID uint, productID uint, productName string, currentStock int, unit string) error {
	if currentStock > 5 {
		return nil
	}
	refIDStr := fmt.Sprintf("%d", productID)
	notifType := "low_stock"
	if currentStock == 0 {
		notifType = "out_of_stock"
	}
	exists, err := s.notifRepo.HasUnreadForReference(ctx, storeID, refIDStr, notifType)
	if err != nil || exists {
		return err
	}
	title := "Stok menipis"
	message := fmt.Sprintf("Produk %s tersisa %d %s.", productName, currentStock, unit)
	if currentStock == 0 {
		title = "Stok habis"
		message = fmt.Sprintf("Produk %s habis (0 %s).", productName, unit)
	}
	return s.Emit(ctx, storeID, title, message, model.CategoryStok, notifType, "", "", "product", refIDStr)
}
