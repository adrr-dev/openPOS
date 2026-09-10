package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/adrr-dev/openPOS/backend/model"
)

type NotificationRepo struct {
	db *gorm.DB
}

func NewNotificationRepo(db *gorm.DB) *NotificationRepo {
	return &NotificationRepo{db: db}
}

func (r *NotificationRepo) Create(ctx context.Context, n *model.Notification) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(n).Error; err != nil {
			return err
		}
		// Retention: max 10 latest per store_id and type (FIFO)
		var oldIDs []uint
		err := tx.Model(&model.Notification{}).
			Where("store_id = ? AND type = ?", n.StoreID, n.Type).
			Order("created_at ASC, id ASC").
			Pluck("id", &oldIDs).Error
		if err != nil {
			return err
		}
		if len(oldIDs) > 10 {
			excess := oldIDs[:len(oldIDs)-10]
			if err := tx.Where("id IN ?", excess).Unscoped().Delete(&model.Notification{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *NotificationRepo) List(ctx context.Context, storeID uint, category string, status string, unreadOnly bool, page, limit int) ([]*model.Notification, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 200 {
		limit = 200
	}

	query := r.db.WithContext(ctx).Model(&model.Notification{}).Where("store_id = ?", storeID)
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if status == "unread" || unreadOnly {
		query = query.Where("read = ?", false)
	} else if status == "read" {
		query = query.Where("read = ?", true)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []*model.Notification
	offset := (page - 1) * limit
	err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, int(total), nil
}

func (r *NotificationRepo) UnreadCount(ctx context.Context, storeID uint) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&model.Notification{}).Where("store_id = ? AND read = ?", storeID, false).Count(&total).Error
	return total, err
}

func (r *NotificationRepo) GetByID(ctx context.Context, storeID, id uint) (*model.Notification, error) {
	var n model.Notification
	if err := r.db.WithContext(ctx).Where("id = ? AND store_id = ?", id, storeID).First(&n).Error; err != nil {
		return nil, mapDBErr(err)
	}
	return &n, nil
}

func (r *NotificationRepo) MarkAsRead(ctx context.Context, storeID, id uint) error {
	res := r.db.WithContext(ctx).Model(&model.Notification{}).Where("id = ? AND store_id = ?", id, storeID).Update("read", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *NotificationRepo) MarkAllAsRead(ctx context.Context, storeID uint) error {
	return r.db.WithContext(ctx).Model(&model.Notification{}).Where("store_id = ? AND read = ?", storeID, false).Update("read", true).Error
}

func (r *NotificationRepo) Delete(ctx context.Context, storeID, id uint) error {
	res := r.db.WithContext(ctx).Where("id = ? AND store_id = ?", id, storeID).Delete(&model.Notification{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *NotificationRepo) HasUnreadForReference(ctx context.Context, storeID uint, referenceID string, notifType string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("store_id = ? AND reference_id = ? AND type = ? AND read = ?", storeID, referenceID, notifType, false).
		Count(&count).Error
	return count > 0, err
}
