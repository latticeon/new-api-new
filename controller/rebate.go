package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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
