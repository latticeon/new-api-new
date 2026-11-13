package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/hotrestart"
	"github.com/gin-gonic/gin"
)

// These handlers must be mounted behind RootAuth; never expose process control
// on public status or relay routes.
func GetHotRestartStatus(c *gin.Context) {
	status := hotrestart.Status()
	common.ApiSuccess(c, gin.H{
		"supported": status.Supported, "pending": status.Pending, "draining": status.Draining,
		"background": status.Background, "last_error": status.LastError, "pid": status.PID,
		"started_at": status.StartedAt, "version": common.Version,
	})
}
func HotRestart(c *gin.Context) {
	if err := hotrestart.RequestRestart(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.SysLog(common.LogText("Hot restart requested by administrator user_id=%d", c.GetInt("id")))
	common.ApiSuccess(c, nil)
}
