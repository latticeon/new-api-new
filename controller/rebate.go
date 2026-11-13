package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"strconv"
)

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
