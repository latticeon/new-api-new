package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

func PreviewErrorMessageRewrite(c *gin.Context) {
	var request struct {
		Message string                    `json:"message"`
		Source  common.ErrorMessageSource `json:"source"`
		Rules   string                    `json:"rules"`
	}
	if err := common.DecodeJson(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20), &request); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Invalid parameters"))
		return
	}
	message, err := common.PreviewErrorMessageRewrite(request.Message, request.Rules, request.Source)
	if err != nil {
		common.ApiErrorT(c, "Invalid error rewrite rules: {{error}}", map[string]any{"error": err.Error()})
		return
	}
	common.ApiSuccess(c, gin.H{"message": message})
}
