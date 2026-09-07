// Cloud endpoint override gate tests (plan phase4 E-2 / M11, r2 gate — "env
// 게이트 단얫(GO_ENV별 2케이스)"): the override is active only under explicit
// GO_ENV=development AND a plain-http loopback URL; every other combination is
// inert and the operating endpoint path is untouched (판정 J7).
package util

import (
	"strings"
	"testing"
)

func TestCloudEndpointOverrideGate(t *testing.T) {
	const envName = CloudEndpointOverrideEnvPrefix + "ALIYUN"
	cases := []struct {
		name     string
		goEnv    string
		value    string
		want     string
		wantOn   bool
		wantErr  bool
		errParts []string
	}{
		{
			name:   "development and loopback http URL activate",
			goEnv:  "development", value: "http://127.0.0.1:18080",
			want: "http://127.0.0.1:18080", wantOn: true,
		},
		{
			name:   "development and localhost activate",
			goEnv:  "development", value: "http://localhost:18080/",
			want: "http://localhost:18080/", wantOn: true,
		},
		{
			name:   "unset GO_ENV keeps the override inert (case 1 of the 2-case gate)",
			goEnv:  "", value: "http://127.0.0.1:18080", wantOn: false,
			wantErr: true, errParts: []string{"GO_ENV", "development"},
		},
		{
			name:   "production GO_ENV keeps the override inert (case 2 of the 2-case gate)",
			goEnv:  "production", value: "http://127.0.0.1:18080", wantOn: false,
			wantErr: true, errParts: []string{"GO_ENV", "development"},
		},
		{
			name:   "remote https override refused",
			goEnv:  "development", value: "https://ecs.example.com", wantOn: false,
			wantErr: true, errParts: []string{"http", "127.0.0.1"},
		},
		{
			name:   "non-loopback host refused",
			goEnv:  "development", value: "http://10.0.0.9:9000", wantOn: false,
			wantErr: true, errParts: []string{"http", "127.0.0.1"},
		},
		{
			name:   "empty value is simply inactive",
			goEnv:  "development", value: "", wantOn: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GO_ENV", tc.goEnv)
			if tc.value == "" {
				t.Setenv(envName, "")
			} else {
				t.Setenv(envName, tc.value)
			}

			got, ok := CloudEndpointOverride("aliyun")
			if ok != tc.wantOn || (ok && got != tc.want) {
				t.Fatalf("CloudEndpointOverride = (%q, %v), want (%q, %v)", got, ok, tc.want, tc.wantOn)
			}
			active, err := CloudEndpointOverrideActive("aliyun")
			if active != tc.wantOn {
				t.Fatalf("CloudEndpointOverrideActive = %v, want %v", active, tc.wantOn)
			}
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an inert-override reason")
				}
				for _, part := range tc.errParts {
					if !strings.Contains(err.Error(), part) {
						t.Fatalf("reason %q does not mention %q", err, part)
					}
				}
			} else if err != nil {
				t.Fatalf("unexpected reason: %v", err)
			}
		})
	}
}

func TestCloudEndpointOverrideEnvName(t *testing.T) {
	for provider, want := range map[string]string{
		"aliyun":  "OPS_ADMIN_CLOUD_ENDPOINT_OVERRIDE_ALIYUN",
		"tencent": "OPS_ADMIN_CLOUD_ENDPOINT_OVERRIDE_TENCENT",
	} {
		if got := CloudEndpointOverrideEnvName(provider); got != want {
			t.Fatalf("CloudEndpointOverrideEnvName(%q) = %q, want %q", provider, got, want)
		}
	}
}

// ValidateCloudEndpointOverrides scans the whole environment: unset or active
// overrides pass, the first set-but-invalid one fails with its provider named.
func TestValidateCloudEndpointOverrides(t *testing.T) {
	t.Run("no override set is clean", func(t *testing.T) {
		t.Setenv("GO_ENV", "production")
		if provider, err := ValidateCloudEndpointOverrides(); provider != "" || err != nil {
			t.Fatalf("clean environment reported (%q, %v)", provider, err)
		}
	})
	t.Run("active override is clean", func(t *testing.T) {
		t.Setenv("GO_ENV", "development")
		t.Setenv(CloudEndpointOverrideEnvPrefix+"ALIYUN", "http://127.0.0.1:1")
		if provider, err := ValidateCloudEndpointOverrides(); provider != "" || err != nil {
			t.Fatalf("active override reported (%q, %v)", provider, err)
		}
	})
	t.Run("invalid override fails with the provider named", func(t *testing.T) {
		t.Setenv("GO_ENV", "development")
		t.Setenv(CloudEndpointOverrideEnvPrefix+"TENCENT", "https://cvm.example.com")
		provider, err := ValidateCloudEndpointOverrides()
		if err == nil {
			t.Fatalf("invalid override passed validation")
		}
		if provider != "tencent" {
			t.Fatalf("provider = %q, want tencent", provider)
		}
	})
}
