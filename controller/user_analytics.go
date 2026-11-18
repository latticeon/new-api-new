package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func AdminGetUserAnalytics(c *gin.Context) {
	parse := func(name string) (int64, bool) {
		raw := c.Query(name)
		if raw == "" {
			return 0, true
		}
		value, err := strconv.ParseInt(raw, 10, 64)
		return value, err == nil
	}
	from, fromOK := parse("from")
	to, toOK := parse("to")
	offset := 0
	if raw := c.Query("timezone_offset_minutes"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			common.ApiErrorT(c, "Invalid timezone offset")
			return
		}
		offset = value
	}
	if !fromOK || !toOK {
		common.ApiErrorT(c, "Invalid date range")
		return
	}
	analytics, err := model.GetUserAnalytics(from, to, offset)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, analytics)
}
