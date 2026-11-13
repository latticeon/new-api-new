package model

import (
	"errors"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"math"
	"strings"
)

// PaymentReceipt preserves original currency and the contemporaneous CNY conversion.
// Registered prices/receipts are deliberately not labelled verified external payment.
type PaymentReceipt struct {
	Amount    string `json:"amount"`
	Currency  string `json:"currency"`
	CNYRate   string `json:"cny_rate"`
	AmountCNY string `json:"amount_cny"`
	Evidence  string `json:"evidence"`
}

func NewPaymentReceipt(amount decimal.Decimal, currency, evidence string) *PaymentReceipt {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	rate := decimal.NewFromInt(1)
	if currency == "USD" {
		r := operation_setting.USDExchangeRate
		if math.IsNaN(r) || math.IsInf(r, 0) || r <= 0 {
			return &PaymentReceipt{Amount: amount.String(), Currency: currency, Evidence: evidence}
		}
		rate = decimal.NewFromFloat(r)
	} else if currency != "CNY" {
		if amount.IsNegative() {
			return nil
		}
		return &PaymentReceipt{Amount: amount.String(), Currency: currency, Evidence: evidence}
	}
	if amount.IsNegative() || amount.GreaterThan(decimal.NewFromInt(common.MaxWalletQuota)) {
		return nil
	}
	return &PaymentReceipt{Amount: amount.String(), Currency: currency, CNYRate: rate.String(), AmountCNY: amount.Mul(rate).String(), Evidence: evidence}
}

func PaymentReceiptJSON(receipt *PaymentReceipt) string {
	if receipt == nil {
		return ""
	}
	data, err := common.Marshal(receipt)
	if err != nil {
		return ""
	}
	return string(data)
}
func parseReceipt(data string) *PaymentReceipt {
	if data == "" {
		return nil
	}
	var receipt PaymentReceipt
	if common.UnmarshalJsonStr(data, &receipt) != nil {
		return nil
	}
	return &receipt
}

func EffectiveRebate(user *User) operation_setting.RebateSetting {
	cfg := *operation_setting.GetRebateSetting()
	if user.RebateMode == "on" {
		cfg.Enabled = true
	}
	if user.RebateMode == "off" {
		cfg.Enabled = false
	}
	if user.RebatePercent != nil {
		cfg.Percent = *user.RebatePercent
	}
	return cfg
}

func snapshotArrivalRebateTx(tx *gorm.DB, event *ArrivalEvent, user *User, participate bool) error {
	cfg := *operation_setting.GetRebateSetting()
	sourceEnabled := event.Source == "online" && cfg.Online || event.Source == "subscription" && cfg.Subscription || event.Source == "redemption" && cfg.Redemption || event.Source == "admin" && cfg.Admin
	amount, amountErr := decimal.NewFromString(event.AmountCNY)
	originalAmount, originalErr := decimal.NewFromString(event.Amount)
	event.RebateQualified = participate && sourceEnabled && originalErr == nil && originalAmount.IsPositive()
	// This marker persists even with global or inviter disabled, or without an inviter.
	// Locking reads see current committed rows even if an earlier source query
	// established a MySQL REPEATABLE READ snapshot before the invitee lock.
	var priorEvents []ArrivalEvent
	if err := lockForUpdate(tx).Select("id", "reward", "status", "rebate_qualified").Where("user_id = ?", user.Id).Find(&priorEvents).Error; err != nil {
		return err
	}
	var prior, reserved int64
	for _, previous := range priorEvents {
		if previous.RebateQualified {
			prior++
		}
		if previous.Status == "pending" || previous.Status == "paid" {
			reserved += int64(previous.Reward)
		}
	}
	if user.InviterId <= 0 || user.InviterId == user.Id {
		event.Reason = "no_direct_inviter"
		return nil
	}
	var inviter User
	if err := tx.First(&inviter, user.InviterId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			event.Reason = "inviter_unavailable"
			return nil
		}
		return err
	}
	event.InviterName = inviter.Username
	cfg = EffectiveRebate(&inviter)
	event.Percent = cfg.Percent
	event.Destination = cfg.Destination
	if cfg.Validate() != nil {
		event.Reason = "invalid_configuration"
		return nil
	}
	snapshot, err := common.Marshal(map[string]any{"effective": cfg, "inviter_mode": inviter.RebateMode, "inviter_percent": inviter.RebatePercent, "global_enabled": operation_setting.GetRebateSetting().Enabled})
	if err != nil {
		return err
	}
	event.RebateSnapshot = string(snapshot)
	if !event.RebateQualified {
		event.Reason = "source_not_qualified"
		return nil
	}
	if !cfg.Enabled {
		event.Reason = "disabled"
		return nil
	}
	if !operation_setting.IsPaymentComplianceConfirmed() {
		event.Reason = "compliance_not_confirmed"
		return nil
	}
	if cfg.Rule == "first" && prior > 0 {
		event.Reason = "not_first_qualified_success"
		return nil
	}
	if amountErr != nil || !amount.IsPositive() {
		event.Reason = "currency_conversion_unavailable"
		return nil
	}
	reward, err := common.WalletQuotaFromDecimalStrict(amount.Mul(decimal.NewFromFloat(cfg.Percent)).Div(decimal.NewFromInt(100)).Mul(decimal.NewFromFloat(cfg.QuotaPerYuan)))
	if err != nil {
		event.Reason = "reward_out_of_range"
		return nil
	}
	// The invitee row held by RecordArrivalTx serializes reservations across all
	// four sources. Pending reservations count toward the same lifetime cap.
	if reserved > int64(common.MaxWalletQuota)-int64(reward) {
		event.Reason = "lifetime_reward_limit"
		return nil
	}
	if cfg.MaxPerInvitee > 0 {
		reward = min(reward, max(0, cfg.MaxPerInvitee-int(reserved)))
	}
	if reward <= 0 {
		event.Reason = "zero_reward_or_cap_reached"
		return nil
	}
	event.Reward = reward
	event.Status = "pending"
	event.Reason = ""
	return nil
}

// PayArrivalRebate never rolls back the originating payment. The event and
// recipient wallet update commit atomically; retry uses only the saved snapshot.
func PayArrivalRebate(id int) error {
	var paid ArrivalEvent
	err := DB.Transaction(func(tx *gorm.DB) error {
		var event ArrivalEvent
		if err := lockForUpdate(tx).First(&event, id).Error; err != nil {
			return err
		}
		if event.Status != "pending" {
			return nil
		}
		if !operation_setting.IsPaymentComplianceConfirmed() {
			return errors.New("payment compliance is not confirmed")
		}
		column := "quota"
		if event.Destination == "affiliate" {
			column = "aff_quota"
		}
		result := tx.Model(&User{}).Where("id = ? AND "+column+" <= ? AND aff_history <= ?", event.InviterId, common.MaxWalletQuota-event.Reward, common.MaxWalletQuota-event.Reward).Updates(map[string]any{column: gorm.Expr(column+" + ?", event.Reward), "aff_history": gorm.Expr("aff_history + ?", event.Reward)})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("recipient missing or reward balance limit exceeded")
		}
		// CAS also protects SQLite, where SELECT FOR UPDATE is unavailable.
		result = tx.Model(&ArrivalEvent{}).Where("id = ? AND status = ?", id, "pending").Updates(map[string]any{"status": "paid", "reason": "", "attempts": gorm.Expr("attempts + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("rebate already claimed")
		}
		paid = event
		return nil
	})
	if err != nil {
		// Failure metadata is separately durable, while the wallet transaction rolled back.
		DB.Model(&ArrivalEvent{}).Where("id = ? AND status = ?", id, "pending").Updates(map[string]any{"reason": "payout_failed", "attempts": gorm.Expr("attempts + 1")})
		common.SysError(common.LogText("Arrival rebate payout failed event_id=%d: %s", id, err.Error()))
		return err
	}
	if paid.Id != 0 && paid.Destination == "wallet" {
		syncCreditUserQuotaCache(paid.InviterId, paid.Reward, "arrival rebate")
	}
	return nil
}

func RetryPendingArrivalRebates(limit int) (int, error) {
	var events []ArrivalEvent
	if err := DB.Where("status = ?", "pending").Order("attempts asc, id asc").Limit(min(max(limit, 1), 1000)).Find(&events).Error; err != nil {
		return 0, err
	}
	count := 0
	var lastErr error
	for _, event := range events {
		if err := PayArrivalRebate(event.Id); err != nil {
			lastErr = err
		} else {
			count++
		}
	}
	return count, lastErr
}

func SetInviterRebateOverride(id int, mode string, percent *float64) error {
	if mode != "inherit" && mode != "on" && mode != "off" {
		return errors.New("invalid rebate override mode")
	}
	if percent != nil && !operation_setting.ValidRebatePercent(*percent) {
		return errors.New("invalid rebate percent")
	}
	return DB.Model(&User{}).Where("id = ?", id).Updates(map[string]any{"rebate_mode": mode, "rebate_percent": percent}).Error
}

func receiptJSON(receipt *PaymentReceipt) string { return PaymentReceiptJSON(receipt) }
