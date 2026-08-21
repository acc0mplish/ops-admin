package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestConfig(t *testing.T, credentialKey string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "app:\n  port: \"8082\"\nsecurity:\n  credential-key: \"" + credentialKey + "\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAcceptsConfiguredCredentialKey(t *testing.T) {
	key := strings.Repeat("k", 32)
	cfg, err := Load(writeTestConfig(t, key))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Security.CredentialKey != key {
		t.Fatalf("credential key was not loaded from config.yaml")
	}
}

func TestLoadRejectsShortCredentialKey(t *testing.T) {
	if _, err := Load(writeTestConfig(t, "too-short")); err == nil {
		t.Fatal("short credential key was accepted")
	}
}
