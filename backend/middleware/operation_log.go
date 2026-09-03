package middleware

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"time"

	"ops-admin/backend/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func OperationLog(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestSummary := auditRequestSummary(c)
		c.Next()

		if !strings.HasPrefix(c.Request.URL.Path, "/api/v1") {
			return
		}
		if c.Request.Method == "GET" || c.Request.URL.Path == "/api/v1/login" {
			return
		}

		userID, _ := c.Get("userID")
		username, _ := c.Get("username")
		statusCode := c.Writer.Status()
		logItem := model.OperationLog{
			Method:         c.Request.Method,
			IP:             c.ClientIP(),
			URL:            c.Request.URL.Path,
			Description:    actionDescription(c.Request.URL.Path, c.Request.Method),
			RiskLevel:      riskLevel(c.Request.URL.Path, c.Request.Method),
			StatusCode:     statusCode,
			Success:        statusCode >= 200 && statusCode < 400,
			DurationMs:     time.Since(start).Milliseconds(),
			RequestSummary: requestSummary,
			CreatedAt:      time.Now(),
		}
		if id, ok := userID.(uint); ok {
			logItem.AdminID = id
		}
		if name, ok := username.(string); ok {
			logItem.Username = name
		}
		db.Create(&logItem)
	}
}

func auditRequestSummary(c *gin.Context) string {
	parts := make([]string, 0, 2)
	if query := strings.TrimSpace(c.Request.URL.RawQuery); query != "" {
		parts = append(parts, "query: "+maskSensitive(query))
	}
	if c.Request.Body != nil && c.Request.Method != "GET" {
		if c.Request.URL.Path == "/api/v1/domain/public/certificates/upload" {
			parts = append(parts, "body: [request-body-omitted:sensitive-ssl]")
			return strings.Join(parts, "\n")
		}
		contentType := strings.ToLower(c.GetHeader("Content-Type"))
		if strings.Contains(contentType, "multipart/form-data") {
			parts = append(parts, "body: [request-body-omitted:multipart]")
			return strings.Join(parts, "\n")
		}
		if c.Request.ContentLength > 4096 || c.Request.ContentLength < 0 {
			parts = append(parts, "body: [request-body-omitted:oversized]")
			return strings.Join(parts, "\n")
		}
		data, _ := io.ReadAll(io.LimitReader(c.Request.Body, 4096))
		c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
		body := strings.TrimSpace(string(data))
		if body != "" {
			parts = append(parts, "body: "+maskSensitive(compactText(body, 1200)))
		}
	}
	return strings.Join(parts, "\n")
}

func maskSensitive(value string) string {
	result := value
	sensitiveKeys := []string{
		"password", "passwd", "secret", "secretKey", "accessKey", "token", "kubeConfig", "kubeconfig",
		"client-key-data", "clientCertificateData", "privateKey", "privateKeyPem", "keyData", "credential",
	}
	for _, key := range sensitiveKeys {
		jsonPattern := regexp.MustCompile(`(?i)("` + regexp.QuoteMeta(key) + `"\s*:\s*")([^"]*)(")`)
		queryPattern := regexp.MustCompile(`(?i)(` + regexp.QuoteMeta(key) + `=)([^&\s]*)`)
		result = jsonPattern.ReplaceAllString(result, `${1}***${3}`)
		result = queryPattern.ReplaceAllString(result, `${1}***`)
	}
	return result
}

func compactText(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "...(truncated)"
}

func riskLevel(path string, method string) string {
	if method == "DELETE" || strings.Contains(path, "/clean") || strings.Contains(path, "/batch/delete") {
		return "high"
	}
	highRiskPaths := []string{
		"/database/sql", "/database/export", "/database/import", "/k8s/yaml", "/k8s/workload/images",
		"/ops/quick", "/ops/job", "/ops/schedule", "/ops/application/pipeline/run",
	}
	for _, item := range highRiskPaths {
		if strings.Contains(path, item) {
			return "high"
		}
	}
	if method == "POST" || method == "PUT" {
		return "medium"
	}
	return "normal"
}

func actionDescription(path string, method string) string {
	switch {
	case strings.Contains(path, "/asset/host"):
		return "Host Asset Management"
	case strings.Contains(path, "/asset/database"):
		return "Database Asset Management"
	case strings.Contains(path, "/asset/gateway"):
		return "Gateway Asset Management"
	case strings.Contains(path, "/database"):
		return "Database Workbench"
	case strings.Contains(path, "/k8s"):
		return "Kubernetes Resource Management"
	case strings.Contains(path, "/ops/quick"):
		return "Quick Execution"
	case strings.Contains(path, "/ops/job"):
		return "Job Orchestration"
	case strings.Contains(path, "/ops/schedule"):
		return "Scheduled Tasks"
	case strings.Contains(path, "/ops/application"):
		return "Application Delivery"
	case strings.Contains(path, "/notify"):
		return "Notifications"
	case strings.Contains(path, "/monitor"):
		return "Monitoring Center"
	case strings.Contains(path, "/domain"):
		return "Domain Management"
	case strings.Contains(path, "/admin"):
		return "User Management"
	case strings.Contains(path, "/role"):
		return "Role Management"
	case strings.Contains(path, "/menu"):
		return "Menu Management"
	case strings.Contains(path, "/dept"):
		return "Department Management"
	case strings.Contains(path, "/post"):
		return "Position Management"
	case strings.Contains(path, "/sysLoginInfo"):
		return "Login Log Management"
	case strings.Contains(path, "/sysOperationLog"):
		return "Operation Log Management"
	default:
		return method + " " + path
	}
}
