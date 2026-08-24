package dnsserver

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func snapshotWithRecords(aValue string, includeCNAME bool) *Snapshot {
	a, _ := dns.NewRR(fmt.Sprintf("grafana.ops.internal. 300 IN A %s", aValue))
	records := map[string]map[uint16][]dns.RR{"grafana.ops.internal.": {dns.TypeA: {a}}}
	if includeCNAME {
		cname, _ := dns.NewRR("monitor.ops.internal. 300 IN CNAME grafana.ops.internal.")
		records["monitor.ops.internal."] = map[uint16][]dns.RR{dns.TypeCNAME: {cname}}
	}
	return &Snapshot{Zones: []string{"ops.internal."}, Records: records, ZoneCount: 1, RecordCount: len(records)}
}

func query(name string, qtype uint16) *dns.Msg {
	message := new(dns.Msg)
	message.SetQuestion(dns.Fqdn(name), qtype)
	return message
}

func TestResolveInternalAAndCNAME(t *testing.T) {
	manager := &Manager{cache: map[string]cacheEntry{}}
	manager.snapshot.Store(snapshotWithRecords("192.168.10.20", true))
	a := manager.Resolve(context.Background(), query("grafana.ops.internal", dns.TypeA))
	if len(a.Answer) != 1 || a.Rcode != dns.RcodeSuccess {
		t.Fatalf("unexpected A response: %#v", a)
	}
	cname := manager.Resolve(context.Background(), query("monitor.ops.internal", dns.TypeA))
	if len(cname.Answer) != 1 || cname.Answer[0].Header().Rrtype != dns.TypeCNAME {
		t.Fatalf("unexpected CNAME response: %#v", cname)
	}
}
func TestInternalMissReturnsNXDOMAINWithoutForward(t *testing.T) {
	manager := &Manager{cache: map[string]cacheEntry{}, settings: Settings{Upstreams: []string{"127.0.0.1:1"}, TimeoutSeconds: 1}}
	manager.snapshot.Store(snapshotWithRecords("192.168.10.20", false))
	response := manager.Resolve(context.Background(), query("missing.ops.internal", dns.TypeA))
	if response.Rcode != dns.RcodeNameError {
		t.Fatalf("rcode=%d want NXDOMAIN", response.Rcode)
	}
}

func TestExistingInternalNameMissingTypeReturnsNODATA(t *testing.T) {
	manager := &Manager{cache: map[string]cacheEntry{}}
	manager.snapshot.Store(snapshotWithRecords("192.168.10.20", false))
	response := manager.Resolve(context.Background(), query("grafana.ops.internal", dns.TypeAAAA))
	if response.Rcode != dns.RcodeSuccess || len(response.Answer) != 0 {
		t.Fatalf("response=%#v want NOERROR/NODATA", response)
	}
}
func TestSnapshotSwapMakesChangesImmediate(t *testing.T) {
	manager := &Manager{cache: map[string]cacheEntry{}}
	manager.ReplaceSnapshot(snapshotWithRecords("192.168.10.20", false))
	first := manager.Resolve(context.Background(), query("grafana.ops.internal", dns.TypeA))
	manager.ReplaceSnapshot(snapshotWithRecords("192.168.10.30", false))
	second := manager.Resolve(context.Background(), query("grafana.ops.internal", dns.TypeA))
	if first.Answer[0].String() == second.Answer[0].String() {
		t.Fatal("record update did not take effect")
	}
	manager.ReplaceSnapshot(&Snapshot{Zones: []string{"ops.internal."}, Records: map[string]map[uint16][]dns.RR{}})
	deleted := manager.Resolve(context.Background(), query("grafana.ops.internal", dns.TypeA))
	if deleted.Rcode != dns.RcodeNameError {
		t.Fatal("deleted record still resolves")
	}
}

func startFakeUpstream(t *testing.T) (string, func()) {
	t.Helper()
	tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := tcpListener.Addr().(*net.TCPAddr).Port
	packet, err := net.ListenPacket("udp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		tcpListener.Close()
		t.Fatal(err)
	}
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		response := new(dns.Msg)
		response.SetReply(r)
		rr, _ := dns.NewRR(r.Question[0].Name + " 30 IN A 203.0.113.8")
		response.Answer = []dns.RR{rr}
		_ = w.WriteMsg(response)
	})
	udp := &dns.Server{PacketConn: packet, Handler: handler}
	tcp := &dns.Server{Listener: tcpListener, Handler: handler}
	go udp.ActivateAndServe()
	go tcp.ActivateAndServe()
	return fmt.Sprintf("127.0.0.1:%d", port), func() { _ = udp.Shutdown(); _ = tcp.Shutdown() }
}

func TestForwarderFailoverAndCache(t *testing.T) {
	upstream, stop := startFakeUpstream(t)
	defer stop()
	manager := &Manager{cache: map[string]cacheEntry{}, settings: Settings{Upstreams: []string{"127.0.0.1:1", upstream}, TimeoutSeconds: 1}}
	manager.snapshot.Store(&Snapshot{})
	response := manager.Resolve(context.Background(), query("www.example.com", dns.TypeA))
	if response.Rcode != dns.RcodeSuccess || len(response.Answer) != 1 {
		t.Fatalf("failover response=%#v", response)
	}
	stop()
	cached := manager.Resolve(context.Background(), query("www.example.com", dns.TypeA))
	if cached.Rcode != dns.RcodeSuccess || len(cached.Answer) != 1 {
		t.Fatal("cached forward response unavailable")
	}
}
func TestForwarderTimeoutReturnsSERVFAIL(t *testing.T) {
	manager := &Manager{cache: map[string]cacheEntry{}, settings: Settings{Upstreams: []string{"127.0.0.1:1"}, TimeoutSeconds: 1}}
	manager.snapshot.Store(&Snapshot{})
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	response := manager.Resolve(ctx, query("www.example.com", dns.TypeA))
	if response.Rcode != dns.RcodeServerFailure {
		t.Fatalf("rcode=%d want SERVFAIL", response.Rcode)
	}
}

func freePort(t *testing.T) int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
func TestUDPAndTCPServerLifecycle(t *testing.T) {
	port := freePort(t)
	manager := &Manager{cache: map[string]cacheEntry{}}
	manager.snapshot.Store(snapshotWithRecords("192.168.10.20", false))
	settings := Settings{Enabled: true, ListenAddress: "127.0.0.1", ListenPort: port, TimeoutSeconds: 1}
	if err := manager.Start(settings); err != nil {
		t.Fatal(err)
	}
	defer manager.Stop(context.Background())
	for _, network := range []string{"udp", "tcp"} {
		client := &dns.Client{Net: network, Timeout: time.Second}
		response, _, err := client.Exchange(query("grafana.ops.internal", dns.TypeA), fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil || len(response.Answer) != 1 {
			t.Fatalf("%s query failed: %v %#v", network, err, response)
		}
	}
	if !manager.Status()["running"].(bool) {
		t.Fatal("server not running")
	}
	if err := manager.Apply(Settings{Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if manager.Status()["running"].(bool) {
		t.Fatal("server still running after disable")
	}
}

func TestFailedReplacementKeepsExistingDNSServerRunning(t *testing.T) {
	activePort := freePort(t)
	manager := &Manager{cache: map[string]cacheEntry{}}
	manager.snapshot.Store(snapshotWithRecords("192.168.10.20", false))
	if err := manager.Start(Settings{Enabled: true, ListenAddress: "127.0.0.1", ListenPort: activePort, TimeoutSeconds: 1}); err != nil {
		t.Fatal(err)
	}
	defer manager.Stop(context.Background())

	occupied, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	occupiedPort := occupied.LocalAddr().(*net.UDPAddr).Port
	if err := manager.Start(Settings{Enabled: true, ListenAddress: "127.0.0.1", ListenPort: occupiedPort, TimeoutSeconds: 1}); err == nil {
		t.Fatal("replacement unexpectedly succeeded on an occupied port")
	}
	if !manager.Status()["running"].(bool) {
		t.Fatal("healthy DNS server was stopped after replacement failed")
	}
	client := &dns.Client{Net: "udp", Timeout: time.Second}
	response, _, err := client.Exchange(query("grafana.ops.internal", dns.TypeA), fmt.Sprintf("127.0.0.1:%d", activePort))
	if err != nil || len(response.Answer) != 1 {
		t.Fatalf("original server unavailable after failed replacement: %v %#v", err, response)
	}
}
