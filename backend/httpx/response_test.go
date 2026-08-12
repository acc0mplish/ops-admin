package httpx

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestFailedUsesProvidedHTTPStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	Failed(context, 401, "unauthorized")

	if recorder.Code != 401 {
		t.Fatalf("expected HTTP 401, got %d", recorder.Code)
	}
}
