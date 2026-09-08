package metrics

import (
	"sort"
	"strings"
)

// SetHealth records one health sweep result for a connection — gauge 0/1
// (§18.2 provider_health{connection}).
func (c *Counters) SetHealth(connection string, healthy bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.health[connection] = healthy
}

// RemoveHealth drops a connection's health series — 커넥션 행이 DB에서 사라진
// 뒤에도 마지막 값 라인이 프로세스 수명 내내 잔존하는 것을 막는다(④리뷰 LOW —
// gauge.go). health sweep이 관측 집합 대조로 호출한다.
func (c *Counters) RemoveHealth(connection string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.health, connection)
}

// SetQueueDepth records the queued-task COUNT snapshot for
// worker_queue_depth (무라벨 gauge). The poller tick is the only caller.
func (c *Counters) SetQueueDepth(depth uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queueDepth = depth
	c.queueDepthSet = true
}

// renderHealth emits provider_health{connection} in connection order —
// healthy=1, unhealthy=0.
func (c *Counters) renderHealth(b *strings.Builder) {
	if len(c.health) == 0 {
		return
	}
	conns := make([]string, 0, len(c.health))
	for conn := range c.health {
		conns = append(conns, conn)
	}
	sort.Strings(conns)
	writeHeader(b, "provider_health", "gauge")
	for _, conn := range conns {
		value := "0"
		if c.health[conn] {
			value = "1"
		}
		writeSampleLine(b, "provider_health", "", labelPairs([2]string{"connection", conn}), value)
	}
}

// renderQueueDepth emits the single worker_queue_depth line — only after an
// explicit Set (family absence until then keeps the empty render contract).
func (c *Counters) renderQueueDepth(b *strings.Builder) {
	if !c.queueDepthSet {
		return
	}
	writeHeader(b, "worker_queue_depth", "gauge")
	writeSampleLine(b, "worker_queue_depth", "", "", itoa(c.queueDepth))
}
