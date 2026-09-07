// Tencent legacy path tests (④review LOW-1 + E-2 M11): a 2xx response the SDK
// did not classify as an error (an Error envelope the typed response cannot
// see) must be classified as a query failure — not a nil dereference, not a
// silent empty success — and the E-2 development override drives the SDK at a
// mock endpoint.
package util

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestTencentGetInstancesClassifies2xxErrorEnvelope — LOW-1: the CVM API (and
// any gateway in front of it) may answer HTTP 200 with an error envelope whose
// shape the SDK's classification misses (here: no "Code", no "Response"
// wrapper). GetInstances must classify the region as failed.
func TestTencentGetInstancesClassifies2xxErrorEnvelope(t *testing.T) {
	t.Setenv("GO_ENV", "development")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Error":{"Code":"","Message":"gateway rejected"},"RequestId":"req-1"}`))
	}))
	defer srv.Close()
	t.Setenv(CloudEndpointOverrideEnvPrefix+"TENCENT", srv.URL)

	service := NewTencentCloudService("ak-test", "sk-test")
	instances, err := service.GetInstances([]string{"ap-guangzhou"})
	if err == nil {
		t.Fatalf("an unclassifiable 2xx error envelope must be classified as a failure")
	}
	if len(instances) != 0 {
		t.Fatalf("no instances may be returned from a failed region: %+v", instances)
	}
	if !strings.Contains(err.Error(), "response envelope") {
		t.Fatalf("error %q does not name the classification", err)
	}
}

// TestTencentGetInstancesUsesDevelopmentOverride — E-2/M11: with the gates
// open the SDK reaches the mock endpoint; without them the default CVM host
// is untouched.
func TestTencentGetInstancesUsesDevelopmentOverride(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Response":{"TotalCount":0,"InstanceSet":[],"RequestId":"req-2"}}`))
	}))
	defer srv.Close()

	t.Run("override active under GO_ENV=development", func(t *testing.T) {
		t.Setenv("GO_ENV", "development")
		t.Setenv(CloudEndpointOverrideEnvPrefix+"TENCENT", srv.URL)
		service := NewTencentCloudService("ak-test", "sk-test")
		if _, err := service.GetInstances([]string{"ap-guangzhou"}); err != nil {
			t.Fatalf("mock path should answer cleanly: %v", err)
		}
		if calls == 0 {
			t.Fatalf("the mock endpoint was never called")
		}
	})

	t.Run("inert override keeps the operating endpoint", func(t *testing.T) {
		calls = 0
		t.Setenv("GO_ENV", "production")
		t.Setenv(CloudEndpointOverrideEnvPrefix+"TENCENT", srv.URL)
		service := NewTencentCloudService("ak-test", "sk-test")
		// The default cvm.tencentcloudapi.com path is unreachable in tests —
		// the failure itself proves the override did not fire.
		if _, err := service.GetInstances([]string{"ap-guangzhou"}); err == nil {
			t.Fatalf("expected the default endpoint path to fail in the sandbox")
		}
		if calls != 0 {
			t.Fatalf("the override fired despite a production GO_ENV")
		}
	})
}
