package httpx

import (
	"github.com/gin-gonic/gin"
	"ops-admin/backend/apperr"
)

func Success(c *gin.Context, data any) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    data,
	})
}

func Failed(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{
		"code":        code,
		"message":     message,
		"errorCode":   "",
		"errorParams": gin.H{},
		"data":        nil,
	})
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
	if errorCode, params, ok := apperr.Extract(err); ok {
		FailedCode(c, status, errorCode, params)
		return
	}
	Failed(c, status, err.Error())
}
