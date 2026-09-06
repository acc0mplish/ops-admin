package util

import (
	"strings"
	"testing"
)

func TestEnsureSecretKeySource(t *testing.T) {
	cases := []struct {
		name          string
		goEnv         string
		credentialKey string
		envCredential string
		jwtSecret     string
		masterKeys    string
		wantErr       bool
		errContains   string
	}{
		{name: "nothing configured, GO_ENV unset fails", goEnv: "", wantErr: true, errContains: "GO_ENV=development"},
		{name: "nothing configured, production fails", goEnv: "production", wantErr: true, errContains: "GO_ENV=development"},
		{name: "nothing configured, development is explicit", goEnv: "development"},
		{name: "config credential key present, prod allowed", goEnv: "production", credentialKey: "config-yaml-credential-key-at-least-32-bytes"},
		{name: "OPS_ADMIN_CREDENTIAL_KEY present, prod allowed", goEnv: "production", envCredential: "env-credential-key-at-least-32-bytes"},
		{name: "OPS_ADMIN_JWT_SECRET present, prod allowed", goEnv: "production", jwtSecret: "jwt-secret-fallback-at-least-32-bytes"},
		{name: "OPS_SECRET_MASTER_KEYS present, prod allowed", goEnv: "production", masterKeys: "primary:primary-material-at-least-32-bytes"},
		{name: "whitespace-only sources count as missing, prod fails", goEnv: "production", credentialKey: "   ", envCredential: "  ", jwtSecret: " ", masterKeys: "\t", wantErr: true, errContains: "GO_ENV=development"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GO_ENV", tc.goEnv)
			t.Setenv("OPS_ADMIN_CREDENTIAL_KEY", tc.envCredential)
			t.Setenv("OPS_ADMIN_JWT_SECRET", tc.jwtSecret)
			t.Setenv("OPS_SECRET_MASTER_KEYS", tc.masterKeys)
			err := EnsureSecretKeySource(tc.credentialKey)
			if tc.wantErr {
				if err == nil {
					t.Fatal("missing key source in a non-development environment must fail")
				}
				if !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("error must name the development escape hatch: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected pass, got %v", err)
			}
		})
	}
}
