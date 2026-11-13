package model

import (
	"errors"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var (
	ErrInvalidUserQuotaAdjustment = errors.New("invalid user quota adjustment")
	ErrUserQuotaPermission        = errors.New("cannot adjust quota for this user role")
)

// UserQuotaAdjustment is the immutable database snapshot of a committed manual
// adjustment. Pending relay deductions in the quota cache are not part of it.
type UserQuotaAdjustment struct {
	UserID   int
	Username string
	Before   int
	After    int
}

type AdminArrivalOptions struct {
	RebateEnabled  bool
	ReceivedAmount *float64
	Reference      string
}

func AdjustUserQuota(userID, operatorRole int, mode string, value int, options ...AdminArrivalOptions) (*UserQuotaAdjustment, error) {
	if userID <= 0 || (mode != "add" && mode != "subtract" && mode != "override") {
		return nil, ErrInvalidUserQuotaAdjustment
	}
	if mode != "override" && value <= 0 {
		return nil, ErrInvalidUserQuotaAdjustment
	}
	if value > common.MaxWalletQuota || value < -common.MaxWalletQuota {
		return nil, ErrWalletQuotaLimitExceeded
	}

	var opt AdminArrivalOptions
	if len(options) > 0 {
		opt = options[0]
	}
	if opt.RebateEnabled && (opt.ReceivedAmount == nil || opt.Reference == "" || len(opt.Reference) > 191) {
		return nil, ErrInvalidUserQuotaAdjustment
	}
	if opt.ReceivedAmount != nil && (math.IsNaN(*opt.ReceivedAmount) || math.IsInf(*opt.ReceivedAmount, 0) || *opt.ReceivedAmount < 0 || *opt.ReceivedAmount > float64(common.MaxWalletQuota)) {
		return nil, ErrInvalidUserQuotaAdjustment
	}
	if opt.Reference == "" {
		opt.Reference = common.GetUUID()
	}
	var adjustment UserQuotaAdjustment
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).First(&user, userID).Error; err != nil {
			return err
		}
		if operatorRole != common.RoleRootUser && operatorRole <= user.Role {
			return ErrUserQuotaPermission
		}
		var prior ArrivalEvent
		found := tx.Where("source = ? AND reference = ?", "admin", opt.Reference).Limit(1).Find(&prior)
		if found.Error != nil {
			return found.Error
		}
		if prior.Id != 0 {
			return ErrInvalidUserQuotaAdjustment
		}
		if user.Quota > common.MaxWalletQuota || user.Quota < -common.MaxWalletQuota {
			return ErrWalletQuotaLimitExceeded
		}
		quota := decimal.NewFromInt(int64(value))
		switch mode {
		case "add":
			quota = decimal.NewFromInt(int64(user.Quota)).Add(quota)
		case "subtract":
			quota = decimal.NewFromInt(int64(user.Quota)).Sub(quota)
		}
		after, err := common.WalletQuotaFromDecimalStrict(quota)
		if err != nil {
			return ErrWalletQuotaLimitExceeded
		}
		// An unchanged override is a successful operation, including on MySQL
		// configurations that count only changed rows in RowsAffected.
		if after != user.Quota {
			result := tx.Model(&User{}).Where("id = ?", userID).Update("quota", after)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
		}
		adjustment = UserQuotaAdjustment{UserID: user.Id, Username: user.Username, Before: user.Quota, After: after}
		if after <= user.Quota {
			if opt.RebateEnabled {
				return ErrInvalidUserQuotaAdjustment
			}
			return nil
		}
		var receipt *PaymentReceipt
		if opt.ReceivedAmount != nil {
			receipt = NewPaymentReceipt(decimal.NewFromFloat(*opt.ReceivedAmount), "CNY", "registered_receipt")
		}
		return RecordArrivalTx(tx, ArrivalInput{Source: "admin", Reference: opt.Reference, UserId: userID, Quota: after - user.Quota, Payment: receipt, Participate: opt.RebateEnabled})
	})
	if err != nil {
		return nil, err
	}

	// Apply only the committed difference, preserving outstanding reservations.
	// Both balances are bounded above, so their difference fits in int64.
	delta := int64(adjustment.After) - int64(adjustment.Before)
	if delta != 0 {
		if err := cacheIncrUserQuota(userID, delta); err != nil {
			common.SysError(common.LogText("failed to sync manual quota adjustment for user %d: %s", userID, err))
		}
	}
	if delta > 0 {
		ProcessArrivalEffects("admin", opt.Reference, userID)
	}
	return &adjustment, nil
}
