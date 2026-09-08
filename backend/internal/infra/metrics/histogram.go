package metrics

import (
	"sort"
	"strings"
	"time"
)

// §18.2 버킷 상수(계획 §1.2) — 오름차순, +Inf는 렌더 시 공통으로 첨가.
var (
	apiLatencyBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	durationBuckets   = []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300}
)

// histSample is one label-combination series of a histogram family. counts
// are cumulative per bucket (counts[i] = observations ≤ buckets[i]).
type histSample struct {
	buckets []float64
	counts  []uint64
	sum     float64
	count   uint64
}

func newHistSample(buckets []float64) *histSample {
	return &histSample{buckets: buckets, counts: make([]uint64, len(buckets))}
}

func (h *histSample) observe(v float64) {
	for i, ub := range h.buckets {
		if v <= ub {
			h.counts[i]++
		}
	}
	h.sum += v
	h.count++
}

// ObserveAPILatency records one adapter REST wrapper call duration for
// provider_api_latency_seconds{provider,op}. The adapter choke points are the
// only callers (start-of-call timestamp + deferred observe).
func (c *Counters) ObserveAPILatency(provider, op string, d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.providers[provider] = true
	k := providerOpKey{provider: provider, op: op}
	h := c.apiLatency[k]
	if h == nil {
		h = newHistSample(apiLatencyBuckets)
		c.apiLatency[k] = h
	}
	h.observe(d.Seconds())
}

// ObserveSyncDuration records one sync run duration for
// inventory_sync_duration_seconds{connection,mode}.
func (c *Counters) ObserveSyncDuration(connection, mode string, d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := connModeKey{connection: connection, mode: mode}
	h := c.syncDuration[k]
	if h == nil {
		h = newHistSample(durationBuckets)
		c.syncDuration[k] = h
	}
	h.observe(d.Seconds())
}

// ObserveTaskDuration records one task claim→terminal duration for
// provider_task_duration_seconds{operation,status} (가정 A4: 모든 종단 커밋).
func (c *Counters) ObserveTaskDuration(operation, status string, d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := opStatusKey{operation: operation, status: status}
	h := c.taskDuration[k]
	if h == nil {
		h = newHistSample(durationBuckets)
		c.taskDuration[k] = h
	}
	h.observe(d.Seconds())
}

// renderAPILatency emits provider_api_latency_seconds in
// {provider asc, op asc} series order.
func (c *Counters) renderAPILatency(b *strings.Builder) {
	if len(c.apiLatency) == 0 {
		return
	}
	keys := make([]providerOpKey, 0, len(c.apiLatency))
	for k := range c.apiLatency {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].provider != keys[j].provider {
			return keys[i].provider < keys[j].provider
		}
		return keys[i].op < keys[j].op
	})
	writeHeader(b, "provider_api_latency_seconds", "histogram")
	for _, k := range keys {
		labels := labelPairs([2]string{"provider", k.provider}, [2]string{"op", k.op})
		writeHistogram(b, "provider_api_latency_seconds", labels, c.apiLatency[k])
	}
}

// renderSyncDuration emits inventory_sync_duration_seconds in
// {connection asc, mode asc} series order.
func (c *Counters) renderSyncDuration(b *strings.Builder) {
	if len(c.syncDuration) == 0 {
		return
	}
	keys := make([]connModeKey, 0, len(c.syncDuration))
	for k := range c.syncDuration {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].connection != keys[j].connection {
			return keys[i].connection < keys[j].connection
		}
		return keys[i].mode < keys[j].mode
	})
	writeHeader(b, "inventory_sync_duration_seconds", "histogram")
	for _, k := range keys {
		labels := labelPairs([2]string{"connection", k.connection}, [2]string{"mode", k.mode})
		writeHistogram(b, "inventory_sync_duration_seconds", labels, c.syncDuration[k])
	}
}

// renderTaskDuration emits provider_task_duration_seconds in
// {operation asc, status asc} series order.
func (c *Counters) renderTaskDuration(b *strings.Builder) {
	if len(c.taskDuration) == 0 {
		return
	}
	keys := make([]opStatusKey, 0, len(c.taskDuration))
	for k := range c.taskDuration {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].operation != keys[j].operation {
			return keys[i].operation < keys[j].operation
		}
		return keys[i].status < keys[j].status
	})
	writeHeader(b, "provider_task_duration_seconds", "histogram")
	for _, k := range keys {
		labels := labelPairs([2]string{"operation", k.operation}, [2]string{"status", k.status})
		writeHistogram(b, "provider_task_duration_seconds", labels, c.taskDuration[k])
	}
}

// writeHistogram renders one series of a histogram family whose `# TYPE`
// header the caller already emitted: _bucket lines (le 오름차순, 마지막
// +Inf — cumulative counts), then _sum, _count.
func writeHistogram(b *strings.Builder, name, labels string, h *histSample) {
	for i, ub := range h.buckets {
		le := labelPairs([2]string{"le", formatFloat(ub)})
		writeSampleLine(b, name, "_bucket", joinLabels(labels, le), itoa(h.counts[i]))
	}
	inf := labelPairs([2]string{"le", "+Inf"})
	writeSampleLine(b, name, "_bucket", joinLabels(labels, inf), itoa(h.count))
	writeSampleLine(b, name, "_sum", labels, formatFloat(h.sum))
	writeSampleLine(b, name, "_count", labels, itoa(h.count))
}
