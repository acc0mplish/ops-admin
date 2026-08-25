package util

import "testing"

func TestSecretEncryptionRoundTrip(t *testing.T) {
	ConfigureCredentialKey("")
	t.Cleanup(func() { ConfigureCredentialKey("") })
	t.Setenv("OPS_ADMIN_CREDENTIAL_KEY", "unit-test-domain-credential-key")
	encrypted, err := EncryptSecret("sensitive-value")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "sensitive-value" {
		t.Fatal("secret stored as plaintext")
	}
	plain, err := DecryptSecret(encrypted)
	if err != nil || plain != "sensitive-value" {
		t.Fatalf("round trip failed: %q %v", plain, err)
	}
}

func TestConfiguredCredentialKeyTakesPriority(t *testing.T) {
	t.Setenv("OPS_ADMIN_CREDENTIAL_KEY", "legacy-environment-key")
	ConfigureCredentialKey("config-yaml-credential-key-at-least-32-bytes")
	t.Cleanup(func() { ConfigureCredentialKey("") })
	encrypted, err := EncryptSecret("configured-secret")
	if err != nil {
		t.Fatal(err)
	}
	ConfigureCredentialKey("")
	if _, err := DecryptSecret(encrypted); err == nil {
		t.Fatal("ciphertext unexpectedly decrypted with the environment fallback")
	}
	ConfigureCredentialKey("config-yaml-credential-key-at-least-32-bytes")
	plain, err := DecryptSecret(encrypted)
	if err != nil || plain != "configured-secret" {
		t.Fatalf("configured key round trip failed: %q %v", plain, err)
	}
}
