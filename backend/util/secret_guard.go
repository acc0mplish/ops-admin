package util

import (
	"fmt"
	"os"
	"strings"
)

// DevelopmentEnvironmentName is the only runtime environment the development
// fallback credential key is permitted in. §20 G-5: "fallback is permitted
// only when GO_ENV == development is explicit; production startup without a
// master key fails".
const DevelopmentEnvironmentName = "development"

// developmentEnvironment returns the GO_ENV value of the running process with
// surrounding whitespace removed; an unset GO_ENV reads as "".
func developmentEnvironment() string {
	return strings.TrimSpace(os.Getenv("GO_ENV"))
}

// IsDevelopmentEnvironment reports whether the process runs with an explicit
// GO_ENV=development. Anything unset, empty, misspelled or production-like
// counts as non-development so the fallback can never be implicit.
func IsDevelopmentEnvironment() bool {
	return developmentEnvironment() == DevelopmentEnvironmentName
}

// EnsureSecretKeySource implements the G-5 startup gate: when no secret key
// source is configured anywhere (the config.yaml credential-key, one of the
// credential seed environment variables, or the v2 master key set), startup
// fails unless the environment is explicitly development. The development
// fallback seed itself stays in place — its deletion is the Step 4 contract,
// not Step -1.
func EnsureSecretKeySource(configuredCredentialKey string) error {
	for _, source := range []string{
		configuredCredentialKey,
		os.Getenv("OPS_ADMIN_CREDENTIAL_KEY"),
		os.Getenv("OPS_ADMIN_JWT_SECRET"),
		os.Getenv("OPS_SECRET_MASTER_KEYS"),
	} {
		if strings.TrimSpace(source) != "" {
			return nil
		}
	}
	if IsDevelopmentEnvironment() {
		return nil
	}
	return fmt.Errorf("no secret key source configured (config.yaml security.credential-key, OPS_ADMIN_CREDENTIAL_KEY, OPS_ADMIN_JWT_SECRET or OPS_SECRET_MASTER_KEYS); " +
		"the development fallback key is permitted only when GO_ENV=development is explicit")
}
