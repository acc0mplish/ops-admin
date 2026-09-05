package util

// SecretFieldClass marks how a registered secret field stores its value.
// The classes follow the §4.1 secret field inventory of the control-plane
// architecture (r2.4): E-legacy columns hold the legacy AES-GCM envelope,
// P columns hold plaintext until the dedicated P-class writer conversion.
type SecretFieldClass string

const (
	// ClassELegacy fields store AES-GCM ciphertext in the legacy envelope
	// (base64url(nonce‖ct), no version prefix). New writes use the v2
	// envelope; reads accept both formats during the migration window.
	ClassELegacy SecretFieldClass = "E-legacy"
	// ClassPlaintext fields store plaintext today. Step 1 leaves their
	// writer untouched; the registry only records them for classification.
	ClassPlaintext SecretFieldClass = "P"
)

// SecretField is one registered column of the §4.1 secret field inventory.
// Rows of the inventory that carry several fields expand into one entry per
// column so lookups stay exact; Row keeps the §4.1 row number.
type SecretField struct {
	// Row is the §4.1 inventory row number (1-15).
	Row int
	// Model is the GORM model that owns the column.
	Model string
	// Table is the physical table name used for direct scans.
	Table string
	// Column is the physical column name (GORM default snake_case).
	Column string
	// Class is the storage class of the field (E-legacy or P).
	Class SecretFieldClass
	// Labels carries short human-readable descriptions for reports.
	Labels []string
	// MixedDeclaration marks the schedule-variable column whose secretness
	// lives in per-value script metadata, not in the column itself. Every
	// classification of such a column must pass a caller-supplied
	// declared-secret gate; the registry alone never decides (§4.3).
	MixedDeclaration bool
	// Conditional marks fields that are secrets only in some configurations
	// (notify_channel.webhook_url when it embeds a path token). Step 1
	// classifies them by their storage class; the condition is decided by a
	// later conversion task.
	Conditional bool
}

// SecretFields is the §4.1 secret field inventory (r2.4), 15 rows, in
// inventory order. Anything not listed here is not a secret by contract.
var SecretFields = []SecretField{
	// Row 1 — PublicDNSAccount: legacy envelope.
	{Row: 1, Model: "PublicDNSAccount", Table: "domain_public_dns_account", Column: "access_key_cipher", Class: ClassELegacy, Labels: []string{"public DNS AccessKey"}},
	{Row: 1, Model: "PublicDNSAccount", Table: "domain_public_dns_account", Column: "secret_key_cipher", Class: ClassELegacy, Labels: []string{"public DNS SecretKey"}},
	// Row 2 — SSLCertificate: legacy envelope.
	{Row: 2, Model: "SSLCertificate", Table: "ssl_certificates", Column: "private_key_cipher", Class: ClassELegacy, Labels: []string{"certificate private key (current)"}},
	// Row 3 — SSLCertificateVersion: legacy envelope, write-only today.
	{Row: 3, Model: "SSLCertificateVersion", Table: "ssl_certificate_versions", Column: "private_key_cipher", Class: ClassELegacy, Labels: []string{"certificate private key (version archive)"}},
	// Row 4 — schedule variables: legacy envelope, secretness declared per value.
	{Row: 4, Model: "OpsScheduleTask", Table: "ops_schedule_task", Column: "variables", Class: ClassELegacy, Labels: []string{"schedule task variables (JSON map)"}, MixedDeclaration: true},
	// Rows 5-15 — P-class plaintext fields.
	{Row: 5, Model: "AssetCredential", Table: "asset_credential", Column: "password", Class: ClassPlaintext, Labels: []string{"asset credential password"}},
	{Row: 5, Model: "AssetCredential", Table: "asset_credential", Column: "private_key", Class: ClassPlaintext, Labels: []string{"asset credential private key"}},
	{Row: 5, Model: "AssetCredential", Table: "asset_credential", Column: "passphrase", Class: ClassPlaintext, Labels: []string{"asset credential passphrase"}},
	{Row: 6, Model: "AssetCloudAccount", Table: "asset_cloud_account", Column: "access_key", Class: ClassPlaintext, Labels: []string{"cloud account AccessKeyID"}},
	{Row: 6, Model: "AssetCloudAccount", Table: "asset_cloud_account", Column: "secret_key", Class: ClassPlaintext, Labels: []string{"cloud account SecretKey"}},
	{Row: 7, Model: "AssetGateway", Table: "asset_gateway", Column: "password", Class: ClassPlaintext, Labels: []string{"gateway password"}},
	{Row: 8, Model: "K8sCluster", Table: "k8s_cluster", Column: "kube_config", Class: ClassPlaintext, Labels: []string{"cluster kubeconfig"}},
	{Row: 9, Model: "IntegrationFinOpsAccount", Table: "integration_finops_account", Column: "secret_key", Class: ClassPlaintext, Labels: []string{"FinOps account SecretKey"}},
	{Row: 9, Model: "IntegrationFinOpsAccount", Table: "integration_finops_account", Column: "billing_token", Class: ClassPlaintext, Labels: []string{"FinOps billing token"}},
	{Row: 10, Model: "MonitorDatasource", Table: "monitor_datasource", Column: "password", Class: ClassPlaintext, Labels: []string{"monitor datasource password"}},
	{Row: 10, Model: "MonitorDatasource", Table: "monitor_datasource", Column: "token", Class: ClassPlaintext, Labels: []string{"monitor datasource token"}},
	{Row: 11, Model: "OpsImageRegistry", Table: "ops_image_registry", Column: "password", Class: ClassPlaintext, Labels: []string{"image registry password"}},
	{Row: 12, Model: "IntegrationAIModel", Table: "integration_ai_model", Column: "api_key", Class: ClassPlaintext, Labels: []string{"AI model API key"}},
	{Row: 13, Model: "LDAPConfig", Table: "sys_ldap_config", Column: "bind_password", Class: ClassPlaintext, Labels: []string{"LDAP bind password"}},
	{Row: 14, Model: "NotifyChannel", Table: "notify_channel", Column: "secret", Class: ClassPlaintext, Labels: []string{"notification channel secret"}},
	{Row: 14, Model: "NotifyChannel", Table: "notify_channel", Column: "webhook_url", Class: ClassPlaintext, Labels: []string{"notification webhook URL"}, Conditional: true},
	{Row: 15, Model: "AssetDatabase", Table: "asset_database", Column: "password", Class: ClassPlaintext, Labels: []string{"database instance password"}},
}

// LookupSecretField resolves a §4.1 registry entry by physical table and
// column. The second return value is false for fields outside the inventory.
func LookupSecretField(table, column string) (SecretField, bool) {
	for _, field := range SecretFields {
		if field.Table == table && field.Column == column {
			return field, true
		}
	}
	return SecretField{}, false
}
