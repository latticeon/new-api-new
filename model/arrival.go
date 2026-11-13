package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

// ArrivalEvent is a durable business receipt/outbox, independent of display logs.
// A source is recorded even while operations are disabled; enabling never replays it.
type ArrivalEvent struct {
	Id           int    `json:"id"`
	Source       string `json:"source" gorm:"type:varchar(24);uniqueIndex:idx_arrival_source"`
	Reference    string `json:"reference" gorm:"type:varchar(191);uniqueIndex:idx_arrival_source"`
	UserId       int    `json:"user_id" gorm:"index"`
	Quota        int    `json:"quota" gorm:"type:bigint"`
	CreatedAt    int64  `json:"created_at" gorm:"index"`
	GroupCounted bool   `json:"group_counted"`
}

type ArrivalInput struct {
	Source, Reference string
	UserId, Quota     int
}

// RecordArrivalTx must be called only on a newly successful business transition.
// Source transaction creates an immutable receipt; group cache refresh runs after commit.
func RecordArrivalTx(tx *gorm.DB, input ArrivalInput) error {
	var user User
	if err := lockForUpdate(tx).First(&user, input.UserId).Error; err != nil {
		return err
	}
	var existing ArrivalEvent
	if err := lockForUpdate(tx).Select("id").Where("source = ? AND reference = ?", input.Source, input.Reference).Limit(1).Find(&existing).Error; err != nil {
		return err
	}
	if existing.Id != 0 {
		return nil
	}
	event := ArrivalEvent{Source: input.Source, Reference: input.Reference, UserId: user.Id, Quota: input.Quota, CreatedAt: common.GetTimestamp()}
	cfg := *operation_setting.GetArrivalSetting()
	// Subscription payments are not a second wallet arrival; periodic resets never enter here.
	if cfg.Enabled && input.Quota > 0 && input.Source != "subscription" && (input.Source != "admin" || cfg.IncludeAdmin) {
		event.GroupCounted = true
		user.ArrivalTotal += min(input.Quota, common.MaxWalletQuota-user.ArrivalTotal)
		baseGroup := user.Group
		var sub UserSubscription
		result := lockForUpdate(tx).Where("user_id = ? AND status = ? AND upgrade_group <> ''", user.Id, "active").Order("id asc").Limit(1).Find(&sub)
		if result.Error != nil {
			return result.Error
		}
		if sub.Id != 0 && sub.PrevUserGroup != "" {
			baseGroup = sub.PrevUserGroup
		}
		if user.EarnedGroup != "" {
			baseGroup = user.EarnedGroup
		}
		reached := cfg.Mode == "single" && input.Quota >= cfg.Threshold || cfg.Mode == "cumulative" && user.ArrivalTotal >= cfg.Threshold
		updates := map[string]any{"arrival_total": user.ArrivalTotal}
		if reached && cfg.Threshold > 0 && cfg.TargetGroup != "" && (cfg.SourceGroup == "" || cfg.SourceGroup == baseGroup) {
			updates["earned_group"] = cfg.TargetGroup
			if sub.Id == 0 {
				updates["group"] = cfg.TargetGroup
			}
		}
		if err := tx.Model(&User{}).Where("id = ?", user.Id).Updates(updates).Error; err != nil {
			return err
		}
	}
	return tx.Create(&event).Error
}

func earnedGroupTx(tx *gorm.DB, userId int) (string, error) {
	var user User
	err := lockForUpdate(tx).Select("id", "earned_group").First(&user, userId).Error
	return user.EarnedGroup, err
}

// ProcessArrivalEffects refreshes group caches only after the source transaction commits.
func ProcessArrivalEffects(source, reference string, userId int) {
	_ = RefreshUserGroupCache(userId)
}
