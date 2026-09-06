// Compose wiring test — the assembly root registers the built-in adapters
// exactly once and hands every component the shared dependency set.
package compose_test

import (
	"strings"
	"testing"

	"ops-admin/backend/internal/infra/compose"
	"ops-admin/backend/internal/testutil"
)

// TestBuildWiresStack — Build must yield a registry carrying both built-in
// adapters and a runner/broker wired onto the same db handle. A second Build
// on a fresh stack must not collide (registration is per-Registry — no
// global state, §11.1).
func TestBuildWiresStack(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)

	stack, err := compose.Build(db)
	if err != nil {
		t.Fatalf("compose.Build: %v", err)
	}
	names := stack.Registry.ProviderTypeNames()
	if len(names) != 2 {
		t.Fatalf("registry provider types = %v, want [fake kubernetes]", names)
	}
	got := map[string]bool{}
	for _, n := range names {
		got[n] = true
	}
	if !got["fake"] || !got["kubernetes"] {
		t.Errorf("registry provider types = %v, want fake and kubernetes", names)
	}
	if stack.Runner == nil || stack.Broker == nil || stack.Counters == nil {
		t.Fatal("stack left a component unwired")
	}
	// The shared counter set pre-registers the kubernetes provider row — the
	// gate ③ flat proof reads the {provider="kubernetes"} 0 line from the
	// report artifact's render.
	if want := `provider_rate_limit_total{provider="kubernetes"} 0`; !strings.Contains(stack.Counters.Render(), want) {
		t.Errorf("counters render missing %q:\n%s", want, stack.Counters.Render())
	}

	// A second assembly (fresh registry, same db) must succeed — explicit
	// composition, no global registration state.
	if _, err := compose.Build(db); err != nil {
		t.Errorf("second compose.Build: %v", err)
	}
}
