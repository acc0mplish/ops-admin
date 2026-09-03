package httpx

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type failedResponse struct {
	Code        int            `json:"code"`
	Message     string         `json:"message"`
	ErrorCode   string         `json:"errorCode"`
	ErrorParams map[string]any `json:"errorParams"`
}

func decodeFailedResponse(t *testing.T, recorder *httptest.ResponseRecorder) failedResponse {
	t.Helper()
	var response failedResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}

func TestFailedUsesProvidedHTTPStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	Failed(context, 401, "unauthorized")

	if recorder.Code != 401 {
		t.Fatalf("expected HTTP 401, got %d", recorder.Code)
	}
	response := decodeFailedResponse(t, recorder)
	if response.ErrorCode != "AUTH_SESSION_EXPIRED" {
		t.Fatalf("expected session-expired code, got %s", response.ErrorCode)
	}
}

func TestFailedHidesLegacyMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	Failed(context, 400, "Prometheus API returned status 500: internal detail")

	response := decodeFailedResponse(t, recorder)
	if response.ErrorCode != "MONITOR_UPSTREAM_REQUEST_FAILED" {
		t.Fatalf("expected monitor upstream code, got %s", response.ErrorCode)
	}
	if response.Message != response.ErrorCode {
		t.Fatalf("expected stable message code, got %q", response.Message)
	}
	if strings.Contains(recorder.Body.String(), "internal detail") {
		t.Fatalf("legacy detail leaked to client: %s", recorder.Body.String())
	}
	if response.ErrorParams["system"] != "Prometheus" {
		t.Fatalf("expected Prometheus parameter, got %v", response.ErrorParams["system"])
	}
	if len(context.Errors) != 1 || !strings.Contains(context.Errors[0].Error(), "internal detail") {
		t.Fatal("expected internal diagnostics to remain on the Gin context")
	}
}

func TestFailedUnknownServerErrorUsesGenericCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	Failed(context, 500, "opaque database implementation detail")

	response := decodeFailedResponse(t, recorder)
	if response.ErrorCode != "OPERATION_FAILED" {
		t.Fatalf("expected generic operation code, got %s", response.ErrorCode)
	}
	if strings.Contains(recorder.Body.String(), "opaque database implementation detail") {
		t.Fatalf("internal detail leaked to client: %s", recorder.Body.String())
	}
}
