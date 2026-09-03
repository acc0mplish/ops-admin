package httpx

import (
	"ops-admin/backend/apperr"

	"github.com/gin-gonic/gin"
)

func Success(c *gin.Context, data any) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    data,
	})
}

func Failed(c *gin.Context, status int, message string) {
	FailedError(c, status, apperr.FromLegacy(message, status))
}

func FailedCode(c *gin.Context, status int, errorCode string, params map[string]any) {
	if params == nil {
		params = map[string]any{}
	}
	c.JSON(status, gin.H{
		"code":        status,
		"message":     errorCode,
		"errorCode":   errorCode,
		"errorParams": params,
		"data":        nil,
	})
}

func FailedError(c *gin.Context, status int, err error) {
	if err == nil {
		FailedCode(c, status, "OPERATION_FAILED", nil)
		return
	}

	appErr := err
	if _, _, ok := apperr.Extract(appErr); !ok {
		appErr = apperr.FromLegacy(err.Error(), status)
	}
	_ = c.Error(appErr)

	errorCode, params, _ := apperr.Extract(appErr)
	FailedCode(c, status, errorCode, params)
}
