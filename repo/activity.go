package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/adrr-dev/openPOS/backend/model"
)

type ActivityRepo struct {
	db *gorm.DB
}

func NewActivityRepo(db *gorm.DB) *ActivityRepo {
	return &ActivityRepo{db: db}
}

type ActivityFilter struct {
	Actor  string
	Action string
	Date   string
	Page   int
	Limit  int
}

func (r *ActivityRepo) Create(ctx context.Context, storeID uint, actorID, actorName, action, detail, refType, refID string) error {
	log := model.ActivityLog{
		StoreID:       storeID,
		ActorID:       actorID,
		ActorName:     actorName,
		Action:        action,
		Detail:        detail,
		ReferenceType: refType,
		ReferenceID:   refID,
	}
	return r.db.WithContext(ctx).Create(&log).Error
}

func (r *ActivityRepo) List(ctx context.Context, storeID uint, f ActivityFilter) ([]model.ActivityLog, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.ActivityLog{}).Where("store_id = ?", storeID)

	if f.Actor != "" {
		query = query.Where("actor_name LIKE ? OR actor_id = ?", "%"+f.Actor+"%", f.Actor)
	}
	if f.Action != "" {
		query = query.Where("action = ?", f.Action)
	}
	if f.Date != "" {
		start := f.Date + " 00:00:00"
		end := f.Date + " 23:59:59"
		query = query.Where("created_at >= ? AND created_at <= ?", start, end)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, mapDBErr(err)
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	limit := f.Limit
	if limit < 1 || limit > 200 {
		limit = 20
	}
	offset := (page - 1) * limit

	var logs []model.ActivityLog
	err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(limit).Find(&logs).Error
	if err != nil {
		return nil, 0, mapDBErr(err)
	}
	return logs, total, nil
}
