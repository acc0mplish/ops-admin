// Package contract is the V2 infrastructure control-plane vocabulary: the
// provider/capability/operation types and closed vocabularies that adapters
// and the registry are built on (spec §7.1, §8.5, §10, §11).
//
// Purity rule (arch-boundary R3 precedent from backend/opdef): this package
// imports the standard library only. Vocabulary packages carry no
// dependencies.
//
// This is the V2 infra provider contract — not the DNS provider domain at
// internal/domain/provider (spec §3 row 13, REMAIN / V2 no-contact).
package contract

// JSONMap is the spec's map-typed payload container (§7.2 TLSProfile/
// ConfigJSON). Values crossing this type must be secret-free.
type JSONMap map[string]any

// ProviderTypeDescriptor is spec §7.1 verbatim.
type ProviderTypeDescriptor struct {
	Type            string
	AdapterVersion  string
	ProtocolVersion string
	ConfigSchema    func() ConfigSpec // typed builder, not a JSON blob (§7.1)
	ContextKinds    []string
	BuiltIn         bool
}

// ConfigSpec/ConfigFieldSpec — 최소 형상(가정 A11): §7.2 ConfigJSON·TLSProfile이
// "시크릿 없는 설정 + SecretRef 참조"임을 표현할 수 있는 최소 집합.
// 필드 확장은 Phase 1(PR 16, connection 모델) 소관.
type ConfigSpec struct {
	Fields []ConfigFieldSpec
}

type ConfigFieldSpec struct {
	Name     string
	Type     string // string | bool | int | secretref
	Required bool
	Secret   bool // true면 값은 SecretRef UID로만 전달(§7.2: ConfigJSON에 시크릿 값 금지)
}

// §7.1 "Registered types in Milestone 1" + §11.1 fake
var M1ProviderTypeNames = []string{"kubernetes", "aliyun", "tencent", "fake"}

// ProviderTypeAliases maps the legacy v1 provider vocabulary onto the §7.1 V2
// provider types (§5.4 propagation rule 1 — J6): each legacy alias and its
// canonical form resolve to the §7.1 registered type of the same family.
// Declared here as vocabulary, not behaviour: core packages look keys up (map
// lookup only — no switch), so the R2 v2 arch-boundary line-wise symbol
// exception covers this declaration and any branch pattern crossing it still
// fails the scan (④review HIGH-1). The literal stays on this declaration line
// so the line-wise symbol pin keeps covering it.
var ProviderTypeAliases = map[string]string{"aliyun": "aliyun", "alicloud": "aliyun", "tencent": "tencent", "tencentcloud": "tencent"}

// §7.1 "Reserved (descriptors land with their milestone, not before)"
var ReservedProviderTypeNames = []string{"proxmox", "vcenter", "cloudstack", "openstack"}

// §7.3 Kind 주석: cluster, account, project, subscription
var ProviderContextKinds = []string{"cluster", "account", "project", "subscription"}
