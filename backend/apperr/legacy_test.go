package apperr

import "testing"

func TestFromLegacyClassifiesDomainErrors(t *testing.T) {
	tests := []struct {
		name    string
		message string
		status  int
		code    string
		param   string
	}{
		{name: "monitor query", message: "query is required", status: 400, code: "MONITOR_QUERY_REQUIRED"},
		{name: "monitor upstream", message: "Prometheus API returned status 500: internal detail", status: 400, code: "MONITOR_UPSTREAM_REQUEST_FAILED", param: "Prometheus"},
		{name: "k8s connection", message: "failed to connect to API Server (https://cluster): dial tcp refused", status: 500, code: "K8S_CLUSTER_CONNECTION_FAILED"},
		{name: "k8s yaml", message: "YAML validation failed; verify field formats", status: 400, code: "K8S_INVALID_YAML"},
		{name: "generic not found", message: "record not found", status: 500, code: "RESOURCE_NOT_FOUND"},
		{name: "unknown client", message: "unexpected request value", status: 400, code: "INVALID_REQUEST"},
		{name: "unknown server", message: "opaque internal detail", status: 500, code: "OPERATION_FAILED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FromLegacy(tt.message, tt.status)
			code, params, ok := Extract(err)
			if !ok {
				t.Fatal("expected an application error")
			}
			if code != tt.code {
				t.Fatalf("expected code %s, got %s", tt.code, code)
			}
			if tt.param != "" && params["system"] != tt.param {
				t.Fatalf("expected system %s, got %v", tt.param, params["system"])
			}
		})
	}
}

func TestFromLegacyKeepsStableCode(t *testing.T) {
	err := FromLegacy("K8S_CLUSTER_NOT_FOUND", 404)
	code, _, ok := Extract(err)
	if !ok || code != "K8S_CLUSTER_NOT_FOUND" {
		t.Fatalf("expected stable code to be preserved, got %q", code)
	}
}
