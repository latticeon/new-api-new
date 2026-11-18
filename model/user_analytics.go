package model

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const maxUserAnalyticsDays = 366

const unknownTopupCurrency = "UNKNOWN"

type UserAnalyticsDaily struct {
	Date                 string             `json:"date"`
	Registrations        int64              `json:"registrations"`
	ActiveUsers          int64              `json:"active_users"`
	SuccessfulLogins     int64              `json:"successful_logins"`
	FailedLogins         int64              `json:"failed_logins"`
	TopupOrders          int64              `json:"topup_orders"`
	Redemptions          int64              `json:"redemptions"`
	RedemptionQuota      int64              `json:"redemption_quota"`
	PayingUsers          int64              `json:"paying_users"`
	RevenueByCurrency    map[string]float64 `json:"revenue_by_currency"`
	RedemptionRevenueCNY float64            `json:"redemption_revenue_cny"`
}

type UserAnalyticsCurrencyTotal struct {
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"`
}

type UserAnalytics struct {
	From                    int64                        `json:"from"`
	To                      int64                        `json:"to"`
	GeneratedAt             int64                        `json:"generated_at"`
	TimezoneOffset          int                          `json:"timezone_offset_minutes"`
	LoginCoverageStart      int64                        `json:"login_coverage_start"`
	TotalUsers              int64                        `json:"total_users"`
	EnabledUsers            int64                        `json:"enabled_users"`
	DisabledUsers           int64                        `json:"disabled_users"`
	InvitedUsers            int64                        `json:"invited_users"`
	NewUsers                int64                        `json:"new_users"`
	UnknownRegistrationTime int64                        `json:"unknown_registration_time"`
	UnknownLastLoginTime    int64                        `json:"unknown_last_login_time"`
	DAU                     int64                        `json:"dau"`
	MAU                     int64                        `json:"mau"`
	SuccessfulLogins        int64                        `json:"successful_logins"`
	FailedLogins            int64                        `json:"failed_logins"`
	TopupOrders             int64                        `json:"topup_orders"`
	Redemptions             int64                        `json:"redemptions"`
	RedemptionQuota         int64                        `json:"redemption_quota"`
	PayingUsers             int64                        `json:"paying_users"`
	RevenueByCurrency       []UserAnalyticsCurrencyTotal `json:"revenue_by_currency"`
	RedemptionRevenueCNY    float64                      `json:"redemption_revenue_cny"`
	Daily                   []UserAnalyticsDaily         `json:"daily"`
}

type userAnalyticsLoginRow struct {
	Day       int64
	Users     int64
	Successes int64
	Failures  int64
}

type userAnalyticsTopupRow struct {
	Day     int64
	Orders  int64
	Revenue float64
}

type userAnalyticsRedemptionRow struct {
	Day     int64
	Count   int64
	Quota   int64
	Revenue float64
}

type userAnalyticsPayingRow struct {
	Day    int64
	UserId int
}

func analyticsDayExpression(db *gorm.DB, column string) string {
	switch db.Dialector.Name() {
	case "mysql":
		return "FLOOR((" + column + " + ?) / 86400)"
	case "clickhouse":
		return "intDiv(" + column + " + ?, 86400)"
	default:
		return "((" + column + " + ?) / 86400)"
	}
}

func GetUserAnalytics(from, to int64, timezoneOffsetMinutes int) (*UserAnalytics, error) {
	if DB == nil || LOG_DB == nil {
		return nil, fmt.Errorf("analytics database is unavailable")
	}
	if timezoneOffsetMinutes < -14*60 || timezoneOffsetMinutes > 14*60 {
		return nil, fmt.Errorf("timezone offset is out of range")
	}
	now := time.Now().Unix()
	if to <= 0 || to > now {
		to = now
	}
	if from <= 0 {
		from = to - 29*86400
	}
	if from > to {
		return nil, fmt.Errorf("date range must be within %d days", maxUserAnalyticsDays)
	}

	offsetSeconds := timezoneOffsetMinutes * 60
	start := localDayStart(from, offsetSeconds)
	end := localDayStart(to, offsetSeconds) + 86400
	if (end-start)/86400 > maxUserAnalyticsDays {
		return nil, fmt.Errorf("date range must be within %d days", maxUserAnalyticsDays)
	}
	startUTC, endUTC := start-int64(offsetSeconds), end-int64(offsetSeconds)
	result := &UserAnalytics{
		From: from, To: to, GeneratedAt: now, TimezoneOffset: timezoneOffsetMinutes,
		RevenueByCurrency: make([]UserAnalyticsCurrencyTotal, 0),
		Daily:             make([]UserAnalyticsDaily, 0, (end-start)/86400),
	}
	dayIndex := make(map[int64]int, cap(result.Daily))
	for day := start; day < end; day += 86400 {
		dayIndex[day/86400] = len(result.Daily)
		result.Daily = append(result.Daily, UserAnalyticsDaily{
			Date:              time.Unix(day, 0).UTC().Format("2006-01-02"),
			RevenueByCurrency: make(map[string]float64),
		})
	}

	if err := DB.Model(&User{}).Count(&result.TotalUsers).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&User{}).Where("status = ?", common.UserStatusEnabled).Count(&result.EnabledUsers).Error; err != nil {
		return nil, err
	}
	result.DisabledUsers = result.TotalUsers - result.EnabledUsers
	if err := DB.Model(&User{}).Where("inviter_id > 0").Count(&result.InvitedUsers).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&User{}).Where("created_at >= ? AND created_at < ?", startUTC, endUTC).Count(&result.NewUsers).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&User{}).Where("created_at <= 0").Count(&result.UnknownRegistrationTime).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&User{}).Where("last_login_at <= 0").Count(&result.UnknownLastLoginTime).Error; err != nil {
		return nil, err
	}

	var registrations []struct {
		Day   int64
		Count int64
	}
	if err := DB.Model(&User{}).
		Select(analyticsDayExpression(DB, "created_at")+" AS day, COUNT(*) AS count", offsetSeconds).
		Where("created_at > 0 AND created_at >= ? AND created_at < ?", startUTC, endUTC).
		Group("day").Scan(&registrations).Error; err != nil {
		return nil, err
	}
	for _, row := range registrations {
		if i, ok := dayIndex[row.Day]; ok {
			result.Daily[i].Registrations = row.Count
		}
	}

	loginQuery := func() *gorm.DB {
		return LOG_DB.Model(&AuditLog{}).Where("category = ?", AuditCategoryLogin)
	}
	successfulLogins := func() *gorm.DB {
		return loginQuery().Where("success = ? AND user_id > 0 AND created_at > 0 AND created_at <= ?", true, now)
	}
	if err := successfulLogins().Select("COALESCE(MIN(created_at), 0)").Scan(&result.LoginCoverageStart).Error; err != nil {
		return nil, err
	}
	todayStart := localDayStart(now, offsetSeconds) - int64(offsetSeconds)
	if err := successfulLogins().Where("created_at >= ?", todayStart).Distinct("user_id").Count(&result.DAU).Error; err != nil {
		return nil, err
	}
	if err := successfulLogins().Where("created_at >= ?", todayStart-29*86400).Distinct("user_id").Count(&result.MAU).Error; err != nil {
		return nil, err
	}

	var loginRows []userAnalyticsLoginRow
	if err := loginQuery().Where("created_at > 0 AND created_at >= ? AND created_at < ?", startUTC, min(endUTC, now+1)).
		Select(analyticsDayExpression(LOG_DB, "created_at")+" AS day, "+
			"COUNT(DISTINCT CASE WHEN success AND user_id > 0 THEN user_id END) AS users, "+
			"SUM(CASE WHEN success THEN 1 ELSE 0 END) AS successes, "+
			"SUM(CASE WHEN NOT success THEN 1 ELSE 0 END) AS failures", offsetSeconds).
		Group("day").Scan(&loginRows).Error; err != nil {
		return nil, err
	}
	for _, row := range loginRows {
		if i, ok := dayIndex[row.Day]; ok {
			result.Daily[i].ActiveUsers = row.Users
			result.Daily[i].SuccessfulLogins = row.Successes
			result.Daily[i].FailedLogins = row.Failures
			result.SuccessfulLogins += row.Successes
			result.FailedLogins += row.Failures
		}
	}

	payingByDay := make([]map[int]struct{}, len(result.Daily))
	payingUsers := make(map[int]struct{})
	recordPayer := func(day int64, userID int) {
		i, ok := dayIndex[day]
		if !ok {
			return
		}
		if payingByDay[i] == nil {
			payingByDay[i] = make(map[int]struct{})
		}
		payingByDay[i][userID] = struct{}{}
		payingUsers[userID] = struct{}{}
	}
	orderTotals := make(map[string]decimal.Decimal)
	orderTotalsByDay := make([]map[string]decimal.Decimal, len(result.Daily))
	var topupRows []TopUp
	if err := DB.Model(&TopUp{}).Select("complete_time, user_id, money, payment_snapshot").
		Where("status = ? AND user_id > 0 AND complete_time > 0 AND complete_time >= ? AND complete_time < ?", common.TopUpStatusSuccess, startUTC, endUTC).
		Find(&topupRows).Error; err != nil {
		return nil, err
	}
	for _, order := range topupRows {
		day := localDayStart(order.CompleteTime, offsetSeconds) / 86400
		i, ok := dayIndex[day]
		if !ok {
			continue
		}
		result.Daily[i].TopupOrders++
		result.TopupOrders++
		currency, amount := unknownTopupCurrency, decimal.Zero
		if receipt := parseReceipt(order.PaymentSnapshot); receipt != nil && receipt.Currency != "" && receipt.Evidence == "provider_payment" {
			if paid, err := decimal.NewFromString(receipt.Amount); err == nil && paid.IsPositive() {
				currency, amount = receipt.Currency, paid
			}
		}
		if !amount.IsPositive() && order.Money > 0 {
			amount = decimal.NewFromFloat(order.Money)
		}
		if !amount.IsPositive() {
			continue
		}
		recordPayer(day, order.UserId)
		if orderTotalsByDay[i] == nil {
			orderTotalsByDay[i] = make(map[string]decimal.Decimal)
		}
		orderTotalsByDay[i][currency] = orderTotalsByDay[i][currency].Add(amount)
		orderTotals[currency] = orderTotals[currency].Add(amount)
	}
	for i, totals := range orderTotalsByDay {
		for currency, amount := range totals {
			result.Daily[i].RevenueByCurrency[currency] = amount.InexactFloat64()
		}
	}
	for currency, amount := range orderTotals {
		result.RevenueByCurrency = append(result.RevenueByCurrency, UserAnalyticsCurrencyTotal{Currency: currency, Amount: amount.InexactFloat64()})
	}
	slices.SortFunc(result.RevenueByCurrency, func(a, b UserAnalyticsCurrencyTotal) int {
		return strings.Compare(a.Currency, b.Currency)
	})

	redemptionQuery := DB.Unscoped().Model(&Redemption{})
	redemptionWhere := "status = ? AND used_user_id > 0 AND redeemed_time > 0 AND redeemed_time >= ? AND redeemed_time < ?"
	redemptionArgs := []any{common.RedemptionCodeStatusUsed, startUTC, endUTC}
	var redemptionRows []userAnalyticsRedemptionRow
	redemptionDay := analyticsDayExpression(DB, "redeemed_time")
	if err := redemptionQuery.Select(redemptionDay+" AS day, COUNT(*) AS count, COALESCE(SUM(quota), 0) AS quota, COALESCE(SUM(CASE WHEN sale_price IS NOT NULL THEN sale_price ELSE 0 END), 0) AS revenue", offsetSeconds).
		Where(redemptionWhere, redemptionArgs...).Group("day").Scan(&redemptionRows).Error; err != nil {
		return nil, err
	}
	for _, row := range redemptionRows {
		if i, ok := dayIndex[row.Day]; ok {
			result.Daily[i].Redemptions = row.Count
			result.Daily[i].RedemptionQuota = row.Quota
			result.Daily[i].RedemptionRevenueCNY = row.Revenue
			result.Redemptions += row.Count
			result.RedemptionQuota += row.Quota
			result.RedemptionRevenueCNY += row.Revenue
		}
	}
	var redeemedUsers []userAnalyticsPayingRow
	if err := redemptionQuery.Select(redemptionDay+" AS day, used_user_id AS user_id", offsetSeconds).
		Where(redemptionWhere+" AND sale_price > 0", common.RedemptionCodeStatusUsed, startUTC, endUTC).Distinct().Find(&redeemedUsers).Error; err != nil {
		return nil, err
	}
	for _, row := range redeemedUsers {
		recordPayer(row.Day, row.UserId)
	}
	for i, users := range payingByDay {
		result.Daily[i].PayingUsers = int64(len(users))
	}
	result.PayingUsers = int64(len(payingUsers))
	return result, nil
}

func localDayStart(timestamp int64, offsetSeconds int) int64 {
	shifted := timestamp + int64(offsetSeconds)
	return shifted / 86400 * 86400
}
