package metrics

import (
	"sort"
	"strconv"
	"strings"
)

// Render produces Prometheus text exposition for the §18.2 M1 family set:
//
//	# TYPE <name> counter|gauge|histogram
//	<name>{labels...} value
//
// Histagrams additionally carry _bucket lines (le 오름차순, +Inf 마지막,
// cumulative counts), _sum and _count. A family emits its `# TYPE` header only
// when it has at least one line — the empty counter set renders as "" and the
// legacy two families keep their byte-exact line shapes:
//
//	provider_rate_limit_total{provider="kubernetes"} 0
//	provider_api_errors_total{provider="kubernetes",op="discover",code="429"} 2
//
// Every registered (or touched) provider yields a rate-limit line — zero
// included — sorted by provider for deterministic artifacts. API error lines
// appear only for (provider,op,code) triples that fired (Prometheus-side
// absence equals zero; the report artifact needs no empty combinatorics).
// Label values are escaped (`\`, `"`, newline) and every family is sorted for
// deterministic artifacts.
func (c *Counters) Render() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var b strings.Builder
	c.renderHealth(&b)         // gauge
	c.renderAPILatency(&b)     // histogram
	c.renderAPIErrors(&b)      // counter (기존)
	c.renderRateLimits(&b)     // counter (기존)
	c.renderSyncDuration(&b)   // histogram
	c.renderSyncChanges(&b)    // counter
	c.renderSyncPartial(&b)    // counter
	c.renderTaskDuration(&b)   // histogram
	c.renderTaskFailures(&b)   // counter
	c.renderTaskRetries(&b)    // counter
	c.renderQueueDepth(&b)     // gauge
	c.renderLeaseExpired(&b)   // counter
	c.renderStaleResources(&b) // counter
	c.renderSecretAccess(&b)   // counter
	return b.String()
}

// renderRateLimits emits provider_rate_limit_total in provider order — the
// gate ③ flat proof. Line shape is byte-frozen (compose/sync Contains 단얫).
func (c *Counters) renderRateLimits(b *strings.Builder) {
	if len(c.providers) == 0 {
		return
	}
	providers := make([]string, 0, len(c.providers))
	for p := range c.providers {
		providers = append(providers, p)
	}
	sort.Strings(providers)
	writeHeader(b, "provider_rate_limit_total", "counter")
	for _, p := range providers {
		writeSampleLine(b, "provider_rate_limit_total", "", labelPairs([2]string{"provider", p}), itoa(c.rateLimits[p]))
	}
}

// renderAPIErrors emits provider_api_errors_total in {provider, op, code}
// order — only triples that fired.
func (c *Counters) renderAPIErrors(b *strings.Builder) {
	if len(c.apiErrors) == 0 {
		return
	}
	keys := make([]apiErrKey, 0, len(c.apiErrors))
	for k := range c.apiErrors {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].provider != keys[j].provider {
			return keys[i].provider < keys[j].provider
		}
		if keys[i].op != keys[j].op {
			return keys[i].op < keys[j].op
		}
		return keys[i].code < keys[j].code
	})
	writeHeader(b, "provider_api_errors_total", "counter")
	for _, k := range keys {
		labels := labelPairs(
			[2]string{"provider", k.provider},
			[2]string{"op", k.op},
			[2]string{"code", k.code},
		)
		writeSampleLine(b, "provider_api_errors_total", "", labels, itoa(c.apiErrors[k]))
	}
}

// renderSyncChanges emits inventory_sync_resource_changes_total in
// connection order.
func (c *Counters) renderSyncChanges(b *strings.Builder) {
	if len(c.syncChanges) == 0 {
		return
	}
	writeHeader(b, "inventory_sync_resource_changes_total", "counter")
	for _, conn := range sortedCounterKeys(c.syncChanges) {
		writeSampleLine(b, "inventory_sync_resource_changes_total", "", labelPairs([2]string{"connection", conn}), itoa(c.syncChanges[conn]))
	}
}

// renderSyncPartial emits inventory_sync_partial_total in connection order.
func (c *Counters) renderSyncPartial(b *strings.Builder) {
	if len(c.syncPartial) == 0 {
		return
	}
	writeHeader(b, "inventory_sync_partial_total", "counter")
	for _, conn := range sortedCounterKeys(c.syncPartial) {
		writeSampleLine(b, "inventory_sync_partial_total", "", labelPairs([2]string{"connection", conn}), itoa(c.syncPartial[conn]))
	}
}

// renderTaskFailures emits provider_task_failures_total in
// {operation, code} order.
func (c *Counters) renderTaskFailures(b *strings.Builder) {
	if len(c.taskFailures) == 0 {
		return
	}
	keys := make([]opCodeKey, 0, len(c.taskFailures))
	for k := range c.taskFailures {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].operation != keys[j].operation {
			return keys[i].operation < keys[j].operation
		}
		return keys[i].code < keys[j].code
	})
	writeHeader(b, "provider_task_failures_total", "counter")
	for _, k := range keys {
		labels := labelPairs([2]string{"operation", k.operation}, [2]string{"code", k.code})
		writeSampleLine(b, "provider_task_failures_total", "", labels, itoa(c.taskFailures[k]))
	}
}

// renderTaskRetries emits provider_task_retries_total in operation order.
func (c *Counters) renderTaskRetries(b *strings.Builder) {
	if len(c.taskRetries) == 0 {
		return
	}
	writeHeader(b, "provider_task_retries_total", "counter")
	for _, op := range sortedCounterKeys(c.taskRetries) {
		writeSampleLine(b, "provider_task_retries_total", "", labelPairs([2]string{"operation", op}), itoa(c.taskRetries[op]))
	}
}

// renderLeaseExpired emits the single worker_lease_expired_total line.
func (c *Counters) renderLeaseExpired(b *strings.Builder) {
	if c.leaseExpired == 0 {
		return
	}
	writeHeader(b, "worker_lease_expired_total", "counter")
	writeSampleLine(b, "worker_lease_expired_total", "", "", itoa(c.leaseExpired))
}

// renderStaleResources emits resource_stale_total{kind} in kind order.
func (c *Counters) renderStaleResources(b *strings.Builder) {
	if len(c.staleResources) == 0 {
		return
	}
	writeHeader(b, "resource_stale_total", "counter")
	for _, kind := range sortedCounterKeys(c.staleResources) {
		writeSampleLine(b, "resource_stale_total", "", labelPairs([2]string{"kind", kind}), itoa(c.staleResources[kind]))
	}
}

// renderSecretAccess emits secret_access_total in {purpose, backend} order.
func (c *Counters) renderSecretAccess(b *strings.Builder) {
	if len(c.secretAccess) == 0 {
		return
	}
	keys := make([]purposeBackendKey, 0, len(c.secretAccess))
	for k := range c.secretAccess {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].purpose != keys[j].purpose {
			return keys[i].purpose < keys[j].purpose
		}
		return keys[i].backend < keys[j].backend
	})
	writeHeader(b, "secret_access_total", "counter")
	for _, k := range keys {
		labels := labelPairs([2]string{"purpose", k.purpose}, [2]string{"backend", k.backend})
		writeSampleLine(b, "secret_access_total", "", labels, itoa(c.secretAccess[k]))
	}
}

// writeHeader appends the `# TYPE <name> <type>` exposition header line.
func writeHeader(b *strings.Builder, name, typ string) {
	b.WriteString("# TYPE ")
	b.WriteString(name)
	b.WriteByte(' ')
	b.WriteString(typ)
	b.WriteByte('\n')
}

// writeSampleLine appends one exposition value line:
// `<name><suffix>{labels} value` (braces omitted when unlabeled).
func writeSampleLine(b *strings.Builder, name, suffix, labels, value string) {
	b.WriteString(name)
	b.WriteString(suffix)
	if labels != "" {
		b.WriteByte('{')
		b.WriteString(labels)
		b.WriteByte('}')
	}
	b.WriteByte(' ')
	b.WriteString(value)
	b.WriteByte('\n')
}

// labelPairs renders sorted-order label pairs: `k="v",k2="v2"` with values
// escaped. label order is caller-fixed (deterministic render contract).
func labelPairs(pairs ...[2]string) string {
	parts := make([]string, 0, len(pairs))
	for _, kv := range pairs {
		parts = append(parts, kv[0]+`="`+escapeLabel(kv[1])+`"`)
	}
	return strings.Join(parts, ",")
}

// joinLabels appends an extra pair (e.g. le=) to a label string.
func joinLabels(labels, extra string) string {
	if labels == "" {
		return extra
	}
	return labels + "," + extra
}

// escapeLabel applies the Prometheus text format label-value escapes:
// backslash, double quote, newline.
func escapeLabel(v string) string {
	if !strings.ContainsAny(v, `"\`) && !strings.Contains(v, "\n") {
		return v
	}
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(v)
}

// sortedCounterKeys returns map[string]uint64 keys in ascending order.
func sortedCounterKeys(m map[string]uint64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// itoa — uint64 to decimal without strconv import churn in the hot path
// (readability over micro-optimisation; strconv would be equally fine).
func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
	}
	return string(digits[i:])
}

// formatFloat — shortest round-trip float formatting for histogram _sum and
// le bucket values ("0.005", "2.5", "+Inf"는 호출부 상수로 직접 기입).
func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
