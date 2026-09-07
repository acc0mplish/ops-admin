// Development-only cloud endpoint override (plan phase4 E-2 / M11, r2 gate):
// the legacy capture path (v1 fetchAliyunCloudInstances / TencentCloudService)
// has no Connection.Endpoint to stuff, so the mock pair run (판정 J1 Path M)
// reaches it through an environment variable instead. The override is
// double-gated — explicit GO_ENV=development AND a plain-http URL to a
// loopback host — so a production process that happens to carry the variable
// keeps the operating endpoint path untouched (판정 J7: 운영 무변경).
package util

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// CloudEndpointOverrideEnvPrefix is the variable stem: the provider's
// upper-cased name completes it (OPS_ADMIN_CLOUD_ENDPOINT_OVERRIDE_ALIYUN,
// OPS_ADMIN_CLOUD_ENDPOINT_OVERRIDE_TENCENT).
const CloudEndpointOverrideEnvPrefix = "OPS_ADMIN_CLOUD_ENDPOINT_OVERRIDE_"

// CloudEndpointOverrideEnvName returns the override variable name for one
// provider type.
func CloudEndpointOverrideEnvName(provider string) string {
	return CloudEndpointOverrideEnvPrefix + strings.ToUpper(strings.TrimSpace(provider))
}

// CloudEndpointOverride returns the mock endpoint override for a provider.
// It is active only when BOTH gates pass:
//
//  1. GO_ENV=development is explicit (secret_guard.go precedent — an unset,
//     empty or production-like GO_ENV makes the variable inert);
//  2. the value is a plain http URL whose host is 127.0.0.1 or localhost
//     (any port) — a remote https override is refused.
//
// Endpoint precedence mirrors the V2 adapter contract: where a connection
// endpoint exists it wins (adapter client.go), the override sits below it and
// above the provider default. The legacy capture path has no connection
// endpoint (A3 — endpoint resolves per region), so there the override is the
// only injection face. Returns ("", false) when inactive.
func CloudEndpointOverride(provider string) (string, bool) {
	if !IsDevelopmentEnvironment() {
		return "", false
	}
	raw := strings.TrimSpace(os.Getenv(CloudEndpointOverrideEnvName(provider)))
	if raw == "" {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if parsed.Scheme != "http" {
		return "", false
	}
	host := parsed.Hostname()
	if host != "127.0.0.1" && host != "localhost" {
		return "", false
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", false
	}
	return raw, true
}

// CloudEndpointOverrideActive reports why an override is inert, for the
// mock-pair runbook: ("", nil) when active, an error describing the failed
// gate otherwise. The compare CLI uses it to decide the §13-10 interim
// marking without duplicating the gate logic.
func CloudEndpointOverrideActive(provider string) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(CloudEndpointOverrideEnvName(provider)))
	if raw == "" {
		return false, nil
	}
	if !IsDevelopmentEnvironment() {
		return false, fmt.Errorf("%s is set but GO_ENV is not %q — the override stays inert",
			CloudEndpointOverrideEnvName(provider), DevelopmentEnvironmentName)
	}
	if _, ok := CloudEndpointOverride(provider); !ok {
		return false, fmt.Errorf("%s must be a plain http URL to 127.0.0.1 or localhost (got %q)",
			CloudEndpointOverrideEnvName(provider), raw)
	}
	return true, nil
}

// ValidateCloudEndpointOverrides checks EVERY set
// OPS_ADMIN_CLOUD_ENDPOINT_OVERRIDE_* variable against the gates, returning
// the provider name and gate error of the first failure — ("", nil) when none
// is set or every set one is active. The compare CLI calls it before touching
// any database: a misconfigured "mock" run whose override is silently inert
// would otherwise send the legacy capture at the operating provider.
func ValidateCloudEndpointOverrides() (string, error) {
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if !ok || !strings.HasPrefix(name, CloudEndpointOverrideEnvPrefix) {
			continue
		}
		provider := strings.ToLower(strings.TrimPrefix(name, CloudEndpointOverrideEnvPrefix))
		if _, err := CloudEndpointOverrideActive(provider); err != nil {
			return provider, err
		}
	}
	return "", nil
}
