package service

import (
	"context"

	"github.com/adrr-dev/openPOS/backend/model"
	"github.com/adrr-dev/openPOS/backend/repo"
)

type ActivityRepository interface {
	Create(ctx context.Context, storeID uint, actorID, actorName, action, detail, refType, refID string) error
	List(ctx context.Context, storeID uint, f repo.ActivityFilter) ([]model.ActivityLog, int64, error)
}

const (
	ActivityLogin           = "LOGIN"
	ActivityLogout          = "LOGOUT"
	ActivitySwitch          = "SWITCH"
	ActivityUserCreated     = "USER_CREATED"
	ActivityProfileUpdated  = "PROFILE_UPDATED"
	ActivityAccountDisabled = "ACCOUNT_DISABLED"
	ActivityAccountEnabled  = "ACCOUNT_ENABLED"
	ActivityPasscodeChanged = "PASSCODE_CHANGED"
)

type ActivityService struct {
	repo ActivityRepository
}

func NewActivityService(repo ActivityRepository) *ActivityService {
	return &ActivityService{repo: repo}
}

func (s *ActivityService) Log(ctx context.Context, storeID uint, actorID, actorName, action, detail, refType, refID string) {
	if s.repo == nil {
		return
	}
	_ = s.repo.Create(ctx, storeID, actorID, actorName, action, detail, refType, refID)
}

func (s *ActivityService) List(ctx context.Context, storeID uint, f repo.ActivityFilter) ([]model.ActivityLog, int64, error) {
	if s.repo == nil {
		return []model.ActivityLog{}, 0, nil
	}
	return s.repo.List(ctx, storeID, f)
}
