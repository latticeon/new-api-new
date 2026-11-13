package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"strconv"
)

// Mount these handlers behind the existing administrator user-management
// permissions. No user-supplied identity is accepted by the self endpoint.
func AdminRebateOverview(c *gin.Context) {
	var filter model.RebateQuery
	if err := c.ShouldBindQuery(&filter); err != nil {
		common.ApiError(c, err)
		return
	}
	data, err := model.GetRebateOverview(filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}
func AdminRebateLedger(c *gin.Context) {
	var filter model.RebateQuery
	if err := c.ShouldBindQuery(&filter); err != nil {
		common.ApiError(c, err)
		return
	}
	page := common.GetPageQuery(c)
	rows, total, err := model.ListArrivalRebates(filter, page)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetTotal(int(total))
	page.SetItems(rows)
	common.ApiSuccess(c, page)
}
func AdminRebateRanking(c *gin.Context)  { adminRebateParticipants(c, true) }
func AdminRebateInvitees(c *gin.Context) { adminRebateParticipants(c, false) }
func adminRebateParticipants(c *gin.Context, ranking bool) {
	var filter model.RebateQuery
	if err := c.ShouldBindQuery(&filter); err != nil {
		common.ApiError(c, err)
		return
	}
	page := common.GetPageQuery(c)
	rows, total, err := model.ListRebateParticipants(filter, ranking, page)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetTotal(int(total))
	page.SetItems(rows)
	common.ApiSuccess(c, page)
}
func AdminRetryRebate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if err := model.PayArrivalRebate(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "rebate.retry", map[string]any{"event_id": id})
	common.ApiSuccess(c, nil)
}
func AdminSetRebateOverride(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	user, err := model.GetUserById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !canManageTargetRole(c.GetInt("role"), user.Role) {
		common.ApiErrorT(c, "No permission to update users of same or higher permission level")
		return
	}
	var request struct {
		Mode    string   `json:"mode"`
		Percent *float64 `json:"percent"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if err := model.SetInviterRebateOverride(id, request.Mode, request.Percent); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	recordManageAuditFor(c, id, "rebate.override", map[string]any{"mode": request.Mode, "percent": request.Percent})
	common.ApiSuccess(c, model.EffectiveRebate(&model.User{RebateMode: request.Mode, RebatePercent: request.Percent}))
}
func GetSelfRebateRule(c *gin.Context) {
	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"mode": user.RebateMode, "percent": user.RebatePercent, "effective": model.EffectiveRebate(user), "compliance_confirmed": operation_setting.IsPaymentComplianceConfirmed()})
}
