package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSSLCertificateUploadBodyIsNeverLogged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"certificatePem":"CERTIFICATE-CONTENT","privateKeyPem":"PRIVATE-KEY-CONTENT"}`
	request := httptest.NewRequest("POST", "/api/v1/domain/public/certificates/upload", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	summary := auditRequestSummary(context)
	if strings.Contains(summary, "CERTIFICATE-CONTENT") || strings.Contains(summary, "PRIVATE-KEY-CONTENT") {
		t.Fatalf("sensitive SSL payload leaked into audit summary: %s", summary)
	}
	if !strings.Contains(summary, "[request-body-omitted:sensitive-ssl]") {
		t.Fatalf("missing explicit skipped marker: %s", summary)
	}
}

func TestMaskSensitiveCoversPrivateKeyPEM(t *testing.T) {
	masked := maskSensitive(`{"privateKeyPem":"PRIVATE-KEY-CONTENT"}`)
	if strings.Contains(masked, "PRIVATE-KEY-CONTENT") || !strings.Contains(masked, `"privateKeyPem":"***"`) {
		t.Fatalf("privateKeyPem was not masked: %s", masked)
	}
}
