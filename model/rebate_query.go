package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"strconv"
)

type RebateQuery struct {
	InviterId int    `form:"inviter_id"`
	UserId    int    `form:"user_id"`
	Search    string `form:"search"`
	Source    string `form:"source"`
	Status    string `form:"status"`
	Start     int64  `form:"start"`
	End       int64  `form:"end"`
	Sort      string `form:"sort"`
	Order     string `form:"order"`
}

func (f RebateQuery) apply(db *gorm.DB) *gorm.DB {
	q := db.Model(&ArrivalEvent{})
	if f.InviterId > 0 {
		q = q.Where("inviter_id = ?", f.InviterId)
	}
	if f.UserId > 0 {
		q = q.Where("user_id = ?", f.UserId)
	}
	if f.Source != "" {
		q = q.Where("source = ?", f.Source)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Start > 0 {
		q = q.Where("created_at >= ?", f.Start)
	}
	if f.End > 0 {
		q = q.Where("created_at <= ?", f.End)
	}
	if f.Search != "" {
		pattern, err := sanitizeLikePattern(f.Search)
		if err != nil {
			return q.Where("1 = 0")
		}
		id, _ := strconv.Atoi(f.Search)
		q = q.Where("inviter_name LIKE ? ESCAPE '!' OR invitee_name LIKE ? ESCAPE '!' OR reference LIKE ? ESCAPE '!' OR inviter_id = ? OR user_id = ?", pattern, pattern, pattern, id, id)
	}
	return q
}
func ListArrivalRebates(filter RebateQuery, page *common.PageInfo) ([]ArrivalEvent, int64, error) {
	q := filter.apply(DB)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	column := "id"
	switch filter.Sort {
	case "created_at", "reward", "user_id", "inviter_id", "percent":
		column = filter.Sort
	}
	direction := "desc"
	if filter.Order == "asc" {
		direction = "asc"
	}
	var rows []ArrivalEvent
	err := q.Order(column + " " + direction).Order("id desc").Offset(page.GetStartIdx()).Limit(page.GetPageSize()).Find(&rows).Error
	return rows, total, err
}

type RebateOverview struct {
	Events        int64 `json:"events"`
	Invitees      int64 `json:"invitees"`
	PaidReward    int64 `json:"paid_reward"`
	PendingReward int64 `json:"pending_reward"`
	Skipped       int64 `json:"skipped"`
}

func GetRebateOverview(filter RebateQuery) (RebateOverview, error) {
	var result RebateOverview
	err := filter.apply(DB).Select("COUNT(*) AS events, COUNT(DISTINCT user_id) AS invitees, COALESCE(SUM(CASE WHEN status = 'paid' THEN reward ELSE 0 END),0) AS paid_reward, COALESCE(SUM(CASE WHEN status = 'pending' THEN reward ELSE 0 END),0) AS pending_reward, COALESCE(SUM(CASE WHEN status = 'skipped' THEN 1 ELSE 0 END),0) AS skipped").Scan(&result).Error
	return result, err
}

type RebateParticipant struct {
	Id        int    `json:"id"`
	Username  string `json:"username"`
	InviterId int    `json:"inviter_id"`
	Events    int64  `json:"events"`
	Reward    int64  `json:"reward"`
	Pending   int64  `json:"pending"`
}

// Participant statistics are derived exclusively from the durable ledger. Users
// with no known source records are still listed, with zero rather than invented history.
func ListRebateParticipants(filter RebateQuery, ranking bool, page *common.PageInfo) ([]RebateParticipant, int64, error) {
	joinColumn := "user_id"
	if ranking {
		joinColumn = "inviter_id"
	}
	ledgerFilter := filter
	ledgerFilter.Search = ""
	aggregate := ledgerFilter.apply(DB).Select(joinColumn + ", COUNT(*) AS events, SUM(CASE WHEN status = 'paid' THEN reward ELSE 0 END) AS reward, SUM(CASE WHEN status = 'pending' THEN reward ELSE 0 END) AS pending").Group(joinColumn)
	q := DB.Model(&User{}).Joins("LEFT JOIN (?) AS ledger ON ledger."+joinColumn+" = users.id", aggregate)
	if ranking {
		q = q.Where("users.id IN (?)", DB.Model(&User{}).Select("inviter_id").Where("inviter_id > 0"))
	} else {
		q = q.Where("users.inviter_id > 0")
		if filter.InviterId > 0 {
			q = q.Where("users.inviter_id = ?", filter.InviterId)
		}
	}
	if ranking && filter.InviterId > 0 {
		q = q.Where("users.id = ?", filter.InviterId)
	}
	if filter.UserId > 0 && !ranking {
		q = q.Where("users.id = ?", filter.UserId)
	}
	if filter.Search != "" {
		pattern, err := sanitizeLikePattern(filter.Search)
		if err != nil {
			return nil, 0, err
		}
		id, _ := strconv.Atoi(filter.Search)
		q = q.Where("users.username LIKE ? ESCAPE '!' OR users.id = ?", pattern, id)
	}
	if filter.Source != "" || filter.Status != "" || filter.Start > 0 || filter.End > 0 {
		q = q.Where("ledger.events > 0")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	column := "users.id"
	switch filter.Sort {
	case "reward", "events", "pending":
		column = filter.Sort
	case "username":
		column = "users.username"
	}
	direction := "desc"
	if filter.Order == "asc" {
		direction = "asc"
	}
	var rows []RebateParticipant
	err := q.Select("users.id, users.username, users.inviter_id, COALESCE(ledger.events,0) AS events, COALESCE(ledger.reward,0) AS reward, COALESCE(ledger.pending,0) AS pending").Order(column + " " + direction).Order("users.id desc").Offset(page.GetStartIdx()).Limit(page.GetPageSize()).Scan(&rows).Error
	return rows, total, err
}
