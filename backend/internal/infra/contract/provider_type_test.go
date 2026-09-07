package contract

import (
	"reflect"
	"testing"
)

// The legacy alias table (④review HIGH-1) covers exactly the §5.4 propagation
// vocabulary: the two aliases and their identities, mapping onto §7.1 types.
func TestProviderTypeAliasesCoverage(t *testing.T) {
	want := map[string]string{
		"aliyun": "aliyun", "alicloud": "aliyun",
		"tencent": "tencent", "tencentcloud": "tencent",
	}
	if !reflect.DeepEqual(ProviderTypeAliases, want) {
		t.Fatalf("ProviderTypeAliases = %v, want %v", ProviderTypeAliases, want)
	}
	registered := make(map[string]bool, len(M1ProviderTypeNames))
	for _, name := range M1ProviderTypeNames {
		registered[name] = true
	}
	for alias, target := range ProviderTypeAliases {
		if !registered[target] {
			t.Fatalf("alias %q maps to %q which is not a §7.1 registered type", alias, target)
		}
	}
}
