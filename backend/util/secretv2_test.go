package util

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"regexp"
	"sync"
	"testing"
)

var v2EnvelopePattern = regexp.MustCompile(`^v2:[^:]+:[A-Za-z0-9_-]+$`)

var sslPrivateKeyField = SecretField{Table: "ssl_certificates", Column: "private_key_cipher"}
var credentialPasswordField = SecretField{Table: "asset_credential", Column: "password"}

// configureMasterKeys pins the master key set for a test and restores the
// implicit set afterwards so tests cannot pollute each other's key state.
func configureMasterKeys(t *testing.T, spec string) {
	t.Helper()
	t.Setenv("OPS_SECRET_MASTER_KEYS", "")
	if err := ConfigureSecretMasterKeys(spec); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ConfigureSecretMasterKeys("") })
}

func TestEncryptSecretV2RoundTrip(t *testing.T) {
	configureMasterKeys(t, "t1:material-for-t1")
	sealed, err := EncryptSecretV2("sensitive-value")
	if err != nil {
		t.Fatal(err)
	}
	if !v2EnvelopePattern.MatchString(sealed) {
		t.Fatalf("envelope format mismatch: %q", sealed)
	}
	plain, err := DecryptSecretV2(sealed)
	if err != nil || plain != "sensitive-value" {
		t.Fatalf("round trip failed: %q %v", plain, err)
	}
}

func TestEncryptSecretV2EmptyStaysEmpty(t *testing.T) {
	configureMasterKeys(t, "t2:material-for-t2")
	sealed, err := EncryptSecretV2("")
	if err != nil {
		t.Fatal(err)
	}
	if sealed != "" {
		t.Fatalf("empty secret must stay envelope-free, got %q", sealed)
	}
}

func TestDecryptSecretV2RejectsMalformed(t *testing.T) {
	configureMasterKeys(t, "t3:material-for-t3")
	for _, value := range []string{"", "plaintext", "v2:", "v2:k1", "v2:k1:!!!", "v2:k1:abc"} {
		if plain, err := DecryptSecretV2(value); err == nil {
			t.Fatalf("malformed value %q must be rejected, got %q", value, plain)
		}
	}
}

func TestDecryptSecretV2UnknownKeyID(t *testing.T) {
	configureMasterKeys(t, "ghost:ghost-material")
	sealed, err := EncryptSecretV2("payload")
	if err != nil {
		t.Fatal(err)
	}
	configureMasterKeys(t, "current:current-material")
	if _, err := DecryptSecretV2(sealed); err == nil {
		t.Fatal("unknown key id must be rejected")
	} else if !bytes.Contains([]byte(err.Error()), []byte("ghost")) {
		t.Fatalf("error must name the unknown key id: %v", err)
	}
}

func TestKeySetRotationDualDecrypt(t *testing.T) {
	configureMasterKeys(t, "alpha:key-alpha-material")
	oldSealed, err := EncryptSecretV2("rotated-payload")
	if err != nil {
		t.Fatal(err)
	}
	configureMasterKeys(t, "beta:key-beta-material,alpha:key-alpha-material")
	plain, err := DecryptSecretV2(oldSealed)
	if err != nil || plain != "rotated-payload" {
		t.Fatalf("old key id must route to its retained key: %q %v", plain, err)
	}
	newSealed, err := EncryptSecretV2("new-payload")
	if err != nil {
		t.Fatal(err)
	}
	if got := newSealed[:len("v2:beta:")]; got != "v2:beta:" {
		t.Fatalf("writer must use the first key set entry, got %q", newSealed)
	}
}

func TestParseMasterKeys(t *testing.T) {
	configureMasterKeys(t, "a:first-key-material")
	if err := ConfigureSecretMasterKeys("a:one,a:two"); err == nil {
		t.Fatal("duplicate key ids must be rejected")
	}
	if err := ConfigureSecretMasterKeys("legacy:reserved"); err == nil {
		t.Fatal("explicit legacy key id must be rejected")
	}
	if err := ConfigureSecretMasterKeys(":material"); err == nil {
		t.Fatal("empty key id must be rejected")
	}
	if err := ConfigureSecretMasterKeys("a:"); err == nil {
		t.Fatal("empty key material must be rejected")
	}
	if err := ConfigureSecretMasterKeys("no-colon"); err == nil {
		t.Fatal("missing colon must be rejected")
	}
	if err := ConfigureSecretMasterKeys("a b:material"); err == nil {
		t.Fatal("whitespace inside a key id must be rejected")
	}
	// Surrounding whitespace is trimmed, not rejected.
	if err := ConfigureSecretMasterKeys(" b : second , a : first "); err != nil {
		t.Fatal(err)
	}
	sealedB, err := EncryptSecretV2("value-b")
	if err != nil {
		t.Fatal(err)
	}
	if got := sealedB[:len("v2:b:")]; got != "v2:b:" {
		t.Fatalf("first entry must be current, got %q", sealedB)
	}
	// "" resets to the implicit legacy set (M-4).
	if err := ConfigureSecretMasterKeys(""); err != nil {
		t.Fatal(err)
	}
	implicit, err := EncryptSecretV2("implicit")
	if err != nil {
		t.Fatal(err)
	}
	if got := implicit[:len("v2:legacy:")]; got != "v2:legacy:" {
		t.Fatalf("empty spec must reset to the implicit legacy set, got %q", implicit)
	}
}

func TestImplicitLegacyKeySetDerivesAtFirstUse(t *testing.T) {
	configureMasterKeys(t, "")
	ConfigureCredentialKey("first-credential-seed")
	t.Cleanup(func() { ConfigureCredentialKey("") })
	first, err := EncryptSecretV2("first-payload")
	if err != nil {
		t.Fatal(err)
	}
	if got := first[:len("v2:legacy:")]; got != "v2:legacy:" {
		t.Fatalf("unset master keys must emit the implicit legacy set, got %q", first)
	}
	plain, err := DecryptSecretV2(first)
	if err != nil || plain != "first-payload" {
		t.Fatalf("implicit legacy set must self-decrypt: %q %v", plain, err)
	}
	// M-3: the implicit legacy key is derived at use time, so reconfiguring the
	// credential seed must change the key later envelopes are sealed with.
	ConfigureCredentialKey("second-credential-seed")
	second, err := EncryptSecretV2("second-payload")
	if err != nil {
		t.Fatal(err)
	}
	if plain, err := DecryptSecretV2(second); err != nil || plain != "second-payload" {
		t.Fatalf("implicit legacy envelope must follow the current seed: %q %v", plain, err)
	}
	if _, err := DecryptSecretV2(first); err == nil {
		t.Fatal("envelope sealed under the previous seed must not decrypt after reconfiguration")
	}
}

func TestReadSecretFieldDualKey(t *testing.T) {
	configureMasterKeys(t, "t8:material-for-t8")
	ConfigureCredentialKey("read-secret-field-seed")
	t.Cleanup(func() { ConfigureCredentialKey("") })
	v2Value, err := EncryptSecretV2("v2-plain")
	if err != nil {
		t.Fatal(err)
	}
	legacyValue, err := EncryptSecret("legacy-plain")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := ReadSecretField(v2Value, sslPrivateKeyField, true)
	if err != nil || plain != "v2-plain" {
		t.Fatalf("v2 value not restored: %q %v", plain, err)
	}
	plain, err = ReadSecretField(legacyValue, sslPrivateKeyField, true)
	if err != nil || plain != "legacy-plain" {
		t.Fatalf("legacy value not restored: %q %v", plain, err)
	}
}

func TestClassifyAllBranches(t *testing.T) {
	configureMasterKeys(t, "t9:material-for-t9")
	ConfigureCredentialKey("classify-all-branches-seed")
	t.Cleanup(func() { ConfigureCredentialKey("") })
	v2Value, err := EncryptSecretV2("v2-value")
	if err != nil {
		t.Fatal(err)
	}
	legacyValue, err := EncryptSecret("legacy-value")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		value    string
		field    SecretField
		declared bool
		want     SecretFormat
	}{
		{"unregistered field", "anything", SecretField{Table: "no_table", Column: "no_column"}, true, FormatNotSecret},
		{"empty", "", sslPrivateKeyField, true, FormatEmpty},
		{"v2", v2Value, sslPrivateKeyField, true, FormatV2},
		{"bare prefix", "v2:", sslPrivateKeyField, true, FormatUnknown},
		{"invalid base64 payload", "v2:k1:!!!", sslPrivateKeyField, true, FormatUnknown},
		{"unregistered key id", "v2:ghost:QUJD", sslPrivateKeyField, true, FormatUnknown},
		{"legacy", legacyValue, sslPrivateKeyField, true, FormatLegacy},
		{"plaintext P-class", "user-password", credentialPasswordField, true, FormatPlaintext},
		{"unknown E-class", "not-base64!!!", sslPrivateKeyField, true, FormatUnknown},
	}
	for _, item := range cases {
		if got := ClassifySecret(item.value, item.field, item.declared); got != item.want {
			t.Errorf("%s: ClassifySecret(%q) = %q, want %q", item.name, item.value, got, item.want)
		}
	}
}

func TestClassifyBase64Trap(t *testing.T) {
	configureMasterKeys(t, "t10:material-for-t10")
	ConfigureCredentialKey("base64-trap-seed")
	t.Cleanup(func() { ConfigureCredentialKey("") })
	// Decodes as rawurl base64 but GCM-open with the legacy key must fail.
	trap := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 40))
	if got := ClassifySecret(trap, sslPrivateKeyField, true); got != FormatUnknown {
		t.Fatalf("base64 trap must be UNKNOWN, got %q", got)
	}
}

func TestReadSecretFieldUnknownFailsClosed(t *testing.T) {
	configureMasterKeys(t, "t11:material-for-t11")
	ConfigureCredentialKey("fail-closed-seed")
	t.Cleanup(func() { ConfigureCredentialKey("") })
	if plain, err := ReadSecretField("not-base64!!!", sslPrivateKeyField, true); err == nil {
		t.Fatalf("UNKNOWN must fail closed, got %q", plain)
	}
}

func TestReadSecretFieldPlaintextPassthrough(t *testing.T) {
	configureMasterKeys(t, "t12:material-for-t12")
	plain, err := ReadSecretField("plain-value", credentialPasswordField, true)
	if err != nil || plain != "plain-value" {
		t.Fatalf("P-class plaintext must pass through: %q %v", plain, err)
	}
}

// TestSecretRegistryMatchesInventory pins the registry to the §4.1 inventory
// (architecture r2.5): 14 rows, E-legacy rows 1-4, P rows 5-14 (row 7 removed — phantom), unique
// table+column, the row 4 mixed-declaration flag and the row 14 conditional
// webhook_url flag.
func TestSecretRegistryMatchesInventory(t *testing.T) {
	type expect struct {
		row    int
		model  string
		table  string
		column string
		class  SecretFieldClass
	}
	expected := []expect{
		{1, "PublicDNSAccount", "domain_public_dns_account", "access_key_cipher", ClassELegacy},
		{1, "PublicDNSAccount", "domain_public_dns_account", "secret_key_cipher", ClassELegacy},
		{2, "SSLCertificate", "ssl_certificates", "private_key_cipher", ClassELegacy},
		{3, "SSLCertificateVersion", "ssl_certificate_versions", "private_key_cipher", ClassELegacy},
		{4, "OpsScheduleTask", "ops_schedule_task", "variables", ClassELegacy},
		{5, "AssetCredential", "asset_credential", "password", ClassPlaintext},
		{5, "AssetCredential", "asset_credential", "private_key", ClassPlaintext},
		{5, "AssetCredential", "asset_credential", "passphrase", ClassPlaintext},
		{6, "AssetCloudAccount", "asset_cloud_account", "access_key", ClassPlaintext},
		{6, "AssetCloudAccount", "asset_cloud_account", "secret_key", ClassPlaintext},
		{8, "K8sCluster", "k8s_cluster", "kube_config", ClassPlaintext},
		{9, "IntegrationFinOpsAccount", "integration_finops_account", "secret_key", ClassPlaintext},
		{9, "IntegrationFinOpsAccount", "integration_finops_account", "billing_token", ClassPlaintext},
		{10, "MonitorDatasource", "monitor_datasource", "password", ClassPlaintext},
		{10, "MonitorDatasource", "monitor_datasource", "token", ClassPlaintext},
		{11, "OpsImageRegistry", "ops_image_registry", "password", ClassPlaintext},
		{12, "IntegrationAIModel", "integration_ai_model", "api_key", ClassPlaintext},
		{13, "LDAPConfig", "sys_ldap_config", "bind_password", ClassPlaintext},
		{14, "NotifyChannel", "notify_channel", "secret", ClassPlaintext},
		{14, "NotifyChannel", "notify_channel", "webhook_url", ClassPlaintext},
		{15, "AssetDatabase", "asset_database", "password", ClassPlaintext},
	}
	if len(SecretFields) != len(expected) {
		t.Fatalf("registry has %d entries, want %d", len(SecretFields), len(expected))
	}
	rows := map[int]int{}
	seen := map[string]struct{}{}
	mixed := 0
	conditional := 0
	for index, field := range SecretFields {
		want := expected[index]
		if field.Row != want.row || field.Model != want.model || field.Table != want.table || field.Column != want.column || field.Class != want.class {
			t.Fatalf("entry %d = %+v, want row %d %s %s.%s %s", index, field, want.row, want.model, want.table, want.column, want.class)
		}
		rows[field.Row]++
		key := field.Table + "." + field.Column
		if _, dup := seen[key]; dup {
			t.Fatalf("duplicate table+column %s", key)
		}
		seen[key] = struct{}{}
		if field.MixedDeclaration {
			mixed++
		}
		if field.Conditional {
			conditional++
		}
	}
	if len(rows) != 14 {
		t.Fatalf("registry spans %d inventory rows, want 14", len(rows))
	}
	for row := 1; row <= 15; row++ {
		if row == 7 {
			continue // phantom row removed in architecture r2.5
		}
		if rows[row] == 0 {
			t.Fatalf("inventory row %d missing from the registry", row)
		}
	}
	classRows := map[SecretFieldClass]map[int]struct{}{ClassELegacy: {}, ClassPlaintext: {}}
	for _, field := range SecretFields {
		classRows[field.Class][field.Row] = struct{}{}
	}
	eRows, pRows := len(classRows[ClassELegacy]), len(classRows[ClassPlaintext])
	if eRows != 4 || pRows != 10 {
		t.Fatalf("class distribution must be E=4 rows / P=11 rows, got E=%d P=%d", eRows, pRows)
	}
	if mixed != 1 {
		t.Fatalf("exactly one mixed-declaration entry expected (row 4), got %d", mixed)
	}
	if conditional != 1 {
		t.Fatalf("exactly one conditional entry expected (notify_channel.webhook_url), got %d", conditional)
	}
	field, ok := LookupSecretField("ops_schedule_task", "variables")
	if !ok || !field.MixedDeclaration {
		t.Fatal("ops_schedule_task.variables must be the mixed-declaration entry")
	}
	field, ok = LookupSecretField("notify_channel", "webhook_url")
	if !ok || !field.Conditional {
		t.Fatal("notify_channel.webhook_url must be the conditional entry")
	}
	if _, ok := LookupSecretField("not_a_table", "not_a_column"); ok {
		t.Fatal("unregistered field must not resolve")
	}
}

func TestConcurrentKeySetAccess(t *testing.T) {
	configureMasterKeys(t, "")
	field, ok := LookupSecretField("ssl_certificates", "private_key_cipher")
	if !ok {
		t.Fatal("registry lookup failed")
	}
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for round := 0; round < 25; round++ {
				spec := fmt.Sprintf("worker-%d:material-%d", worker, round)
				if err := ConfigureSecretMasterKeys(spec); err != nil {
					t.Errorf("configure: %v", err)
					return
				}
				sealed, err := EncryptSecretV2("payload")
				if err != nil {
					t.Errorf("encrypt: %v", err)
					return
				}
				// Decrypt and classify may observe a key set rotated by another
				// worker; the assertion under test is the race detector.
				_, _ = DecryptSecretV2(sealed)
				_ = ClassifySecret(sealed, field, true)
				_, _ = ReadSecretField(sealed, field, true)
			}
		}(worker)
	}
	wg.Wait()
	if err := ConfigureSecretMasterKeys("final:final-material"); err != nil {
		t.Fatal(err)
	}
}

func TestClassifyMixedDeclarationGate(t *testing.T) {
	configureMasterKeys(t, "t15:material-for-t15")
	ConfigureCredentialKey("mixed-gate-seed")
	t.Cleanup(func() { ConfigureCredentialKey("") })
	field, ok := LookupSecretField("ops_schedule_task", "variables")
	if !ok {
		t.Fatal("registry lookup failed")
	}
	legacyValue, err := EncryptSecret("legacy-secret")
	if err != nil {
		t.Fatal(err)
	}
	v2Value, err := EncryptSecretV2("v2-secret")
	if err != nil {
		t.Fatal(err)
	}
	// Declaration wins over the value shape: a non-declared value in the mixed
	// column is NOT_SECRET-by-declaration no matter what it looks like.
	for name, value := range map[string]string{"legacy": legacyValue, "v2": v2Value, "plain": "plain-text"} {
		if got := ClassifySecret(value, field, false); got != FormatNotSecret {
			t.Fatalf("declared=false %s value must be NOT_SECRET, got %q", name, got)
		}
		if plain, err := ReadSecretField(value, field, false); err != nil || plain != value {
			t.Fatalf("declared=false %s value must pass through verbatim: %q %v", name, plain, err)
		}
	}
	if got := ClassifySecret(legacyValue, field, true); got != FormatLegacy {
		t.Fatalf("declared=true legacy must be LEGACY, got %q", got)
	}
	if got := ClassifySecret(v2Value, field, true); got != FormatV2 {
		t.Fatalf("declared=true v2 must be V2, got %q", got)
	}
	if got := ClassifySecret("garbage!!!", field, true); got != FormatUnknown {
		t.Fatalf("declared=true garbage must be UNKNOWN, got %q", got)
	}
	if got := ClassifySecret("", field, true); got != FormatEmpty {
		t.Fatalf("declared=true empty must be EMPTY, got %q", got)
	}
	// Non-mixed columns ignore the declared flag entirely.
	eField, ok := LookupSecretField("ssl_certificates", "private_key_cipher")
	if !ok {
		t.Fatal("registry lookup failed")
	}
	if ClassifySecret("garbage!!!", eField, true) != FormatUnknown || ClassifySecret("garbage!!!", eField, false) != FormatUnknown {
		t.Fatal("non-mixed E-class column must ignore the declared flag")
	}
	pField, ok := LookupSecretField("asset_credential", "password")
	if !ok {
		t.Fatal("registry lookup failed")
	}
	if ClassifySecret("plain", pField, true) != FormatPlaintext || ClassifySecret("plain", pField, false) != FormatPlaintext {
		t.Fatal("non-mixed P-class column must ignore the declared flag")
	}
}

// TestMasterKeysFromEnvironmentAtFirstUse covers the chosen wiring option:
// when the setter never ran, OPS_SECRET_MASTER_KEYS is parsed at first use,
// an invalid spec fails closed, and an explicit setter call wins over env.
func TestMasterKeysFromEnvironmentAtFirstUse(t *testing.T) {
	t.Setenv("OPS_SECRET_MASTER_KEYS", "env-current:env-material")
	// Return the package to the "setter never ran" state.
	secretMasterKeys.Lock()
	secretMasterKeys.configured = false
	secretMasterKeys.entries = nil
	secretMasterKeys.Unlock()
	t.Cleanup(func() { _ = ConfigureSecretMasterKeys("") })
	sealed, err := EncryptSecretV2("env-payload")
	if err != nil {
		t.Fatal(err)
	}
	if got := sealed[:len("v2:env-current:")]; got != "v2:env-current:" {
		t.Fatalf("env spec must be picked up at first use, got %q", sealed)
	}
	if plain, err := DecryptSecretV2(sealed); err != nil || plain != "env-payload" {
		t.Fatalf("env key set must self-decrypt: %q %v", plain, err)
	}
	t.Setenv("OPS_SECRET_MASTER_KEYS", "broken-spec")
	if _, err := EncryptSecretV2("payload"); err == nil {
		t.Fatal("an invalid env spec must fail closed")
	}
	if err := ConfigureSecretMasterKeys("explicit:explicit-material"); err != nil {
		t.Fatal(err)
	}
	sealed, err = EncryptSecretV2("payload")
	if err != nil {
		t.Fatal(err)
	}
	if got := sealed[:len("v2:explicit:")]; got != "v2:explicit:" {
		t.Fatalf("an explicitly configured set must win over the environment, got %q", sealed)
	}
}

// TestImplicitLegacyKeyUsesCredentialSeedChain walks the seed chain mirror of
// credentialKey(): config seed, OPS_ADMIN_CREDENTIAL_KEY, OPS_ADMIN_JWT_SECRET,
// then the development fallback. Each stage must seal and open envelopes with
// the same key derivation the legacy envelope uses.
func TestImplicitLegacyKeyUsesCredentialSeedChain(t *testing.T) {
	configureMasterKeys(t, "")
	t.Cleanup(func() { ConfigureCredentialKey("") })
	t.Setenv("OPS_ADMIN_CREDENTIAL_KEY", "env-credential-seed")
	t.Setenv("OPS_ADMIN_JWT_SECRET", "")
	ConfigureCredentialKey("")
	sealed, err := EncryptSecretV2("credential-env-payload")
	if err != nil {
		t.Fatal(err)
	}
	if plain, err := DecryptSecretV2(sealed); err != nil || plain != "credential-env-payload" {
		t.Fatalf("env credential seed must seal and open: %q %v", plain, err)
	}
	t.Setenv("OPS_ADMIN_CREDENTIAL_KEY", "")
	t.Setenv("OPS_ADMIN_JWT_SECRET", "jwt-secret-seed")
	ConfigureCredentialKey("")
	sealed, err = EncryptSecretV2("jwt-payload")
	if err != nil {
		t.Fatal(err)
	}
	if plain, err := DecryptSecretV2(sealed); err != nil || plain != "jwt-payload" {
		t.Fatalf("jwt secret seed must seal and open: %q %v", plain, err)
	}
	t.Setenv("OPS_ADMIN_JWT_SECRET", "")
	ConfigureCredentialKey("")
	sealed, err = EncryptSecretV2("dev-fallback-payload")
	if err != nil {
		t.Fatal(err)
	}
	if plain, err := DecryptSecretV2(sealed); err != nil || plain != "dev-fallback-payload" {
		t.Fatalf("development fallback seed must seal and open: %q %v", plain, err)
	}
}

func TestAggregateFormats(t *testing.T) {
	counts := AggregateFormats([]SecretFormat{FormatV2, FormatV2, FormatLegacy, FormatPlaintext, FormatEmpty, FormatUnknown, FormatNotSecret})
	want := FieldCounts{Total: 7, V2: 2, Legacy: 1, Plaintext: 1, Empty: 1, Unknown: 1, NotSecret: 1}
	if counts != want {
		t.Fatalf("AggregateFormats = %+v, want %+v", counts, want)
	}
	if got := AggregateFormats(nil); got != (FieldCounts{}) {
		t.Fatalf("AggregateFormats(nil) = %+v, want zero counts", got)
	}
}
