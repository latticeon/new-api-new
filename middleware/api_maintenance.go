package middleware

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// APIExecutionMaintenance belongs before endpoint preparation/distribution.
// Native plugin queries terminate in PrepareTaskPluginRoute before reaching it.
func APIExecutionMaintenance() gin.HandlerFunc {
	return func(c *gin.Context) {
		if RejectAPIExecution(c) {
			return
		}
		c.Next()
	}
}

func RejectAPIExecution(c *gin.Context) bool {
	apiError := service.APIMaintenanceError()
	if apiError == nil {
		return false
	}
	path := c.Request.URL.Path
	message := common.MessageWithRequestId(apiError.Error(), c.GetString(common.RequestIdKey))
	switch {
	case strings.HasPrefix(path, "/v1beta/") || strings.HasPrefix(path, "/v1/models/"):
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": http.StatusServiceUnavailable, "status": "UNAVAILABLE", "message": message}})
	case path == "/v1/messages":
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"type": "error", "error": gin.H{"type": "overloaded_error", "message": message}})
	case strings.Contains(path, "/mj/"):
		abortWithMidjourneyMessage(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, message)
	default:
		abortWithOpenAiMessage(c, http.StatusServiceUnavailable, apiError.Error(), apiError.GetErrorCode())
	}
	return true
}
