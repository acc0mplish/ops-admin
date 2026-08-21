package dnsserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"gorm.io/gorm"
	"ops-admin/backend/model"
)

type Settings struct {
	Enabled        bool     `json:"enabled"`
	ListenAddress  string   `json:"listenAddress"`
	ListenPort     int      `json:"listenPort"`
	Upstreams      []string `json:"upstreams"`
	TimeoutSeconds int      `json:"timeoutSeconds"`
}

type Snapshot struct {
	Zones       []string
	Records     map[string]map[uint16][]dns.RR
	ZoneCount   int
	RecordCount int
}

type cacheEntry struct {
	message   *dns.Msg
	expiresAt time.Time
}

type Manager struct {
	db          *gorm.DB
	snapshot    atomic.Pointer[Snapshot]
	mu          sync.Mutex
	udp         *dns.Server
	tcp         *dns.Server
	running     bool
	lastError   string
	lastRefresh time.Time
	settings    Settings
	cacheMu     sync.RWMutex
	cache       map[string]cacheEntry
}

func NewManager(db *gorm.DB) *Manager {
	m := &Manager{db: db, cache: make(map[string]cacheEntry)}
	_ = m.Rebuild()
	setting, err := LoadSettings(db)
	if err == nil {
		m.settings = setting
		if setting.Enabled {
			if err := m.Start(setting); err != nil {
				m.lastError = err.Error()
				_ = db.Model(&model.InternalDNSSetting{}).Where("id = ?", 1).Updates(map[string]any{"enabled": false, "last_error": err.Error()}).Error
			}
		}
	}
	return m
}

func LoadSettings(db *gorm.DB) (Settings, error) {
	var item model.InternalDNSSetting
	err := db.First(&item, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		item = model.InternalDNSSetting{ID: 1, ListenAddress: "0.0.0.0", ListenPort: 53, UpstreamsJSON: `["223.5.5.5","119.29.29.29","180.76.76.76","114.114.114.114","223.112.112.112"]`, TimeoutSeconds: 2}
		if err = db.Create(&item).Error; err != nil {
			return Settings{}, err
		}
	} else if err != nil {
		return Settings{}, err
	}
	// The authoritative DNS service always binds the standard DNS port. Repair
	// legacy rows that may have stored a configurable port.
	if item.ListenPort != 53 {
		item.ListenPort = 53
		if err = db.Model(&model.InternalDNSSetting{}).Where("id = ?", item.ID).Update("listen_port", 53).Error; err != nil {
			return Settings{}, err
		}
	}
	var upstreams []string
	_ = json.Unmarshal([]byte(item.UpstreamsJSON), &upstreams)
	return normalizeSettings(Settings{Enabled: item.Enabled, ListenAddress: item.ListenAddress, ListenPort: item.ListenPort, Upstreams: upstreams, TimeoutSeconds: item.TimeoutSeconds}), nil
}

func normalizeSettings(value Settings) Settings {
	value.ListenAddress = strings.TrimSpace(value.ListenAddress)
	if value.ListenAddress == "" {
		value.ListenAddress = "0.0.0.0"
	}
	if value.ListenPort < 1 || value.ListenPort > 65535 {
		value.ListenPort = 53
	}
	if value.TimeoutSeconds < 1 || value.TimeoutSeconds > 30 {
		value.TimeoutSeconds = 2
	}
	clean := []string{}
	seen := map[string]struct{}{}
	for _, upstream := range value.Upstreams {
		upstream = strings.TrimSpace(upstream)
		if upstream == "" {
			continue
		}
		if _, _, err := net.SplitHostPort(upstream); err != nil {
			upstream = net.JoinHostPort(upstream, "53")
		}
		if _, ok := seen[upstream]; !ok {
			seen[upstream] = struct{}{}
			clean = append(clean, upstream)
		}
	}
	value.Upstreams = clean
	return value
}

func (m *Manager) Rebuild() error {
	snapshot, err := BuildSnapshot(m.db)
	if err != nil {
		return err
	}
	m.snapshot.Store(snapshot)
	m.mu.Lock()
	m.lastRefresh = time.Now()
	m.mu.Unlock()
	return nil
}

func (m *Manager) ReplaceSnapshot(snapshot *Snapshot) {
	if snapshot == nil {
		return
	}
	m.snapshot.Store(snapshot)
	m.mu.Lock()
	m.lastRefresh = time.Now()
	m.mu.Unlock()
}

func BuildSnapshot(db *gorm.DB) (*Snapshot, error) {
	var zones []model.InternalDNSZone
	if err := db.Where("status = ?", 1).Find(&zones).Error; err != nil {
		return nil, err
	}
	zoneByID := map[uint]string{}
	zoneNames := make([]string, 0, len(zones))
	for _, zone := range zones {
		name := canonical(zone.Name)
		zoneByID[zone.ID] = name
		zoneNames = append(zoneNames, name)
	}
	sort.Slice(zoneNames, func(i, j int) bool { return len(zoneNames[i]) > len(zoneNames[j]) })
	var records []model.InternalDNSRecord
	if len(zoneByID) > 0 {
		ids := make([]uint, 0, len(zoneByID))
		for id := range zoneByID {
			ids = append(ids, id)
		}
		if err := db.Where("zone_id IN ? AND status = ?", ids, 1).Find(&records).Error; err != nil {
			return nil, err
		}
	}
	result := &Snapshot{Zones: zoneNames, Records: map[string]map[uint16][]dns.RR{}, ZoneCount: len(zones)}
	for _, record := range records {
		zone, ok := zoneByID[record.ZoneID]
		if !ok {
			continue
		}
		name := recordName(record.Host, zone)
		var rr dns.RR
		var err error
		switch strings.ToUpper(record.Type) {
		case "A":
			rr, err = dns.NewRR(fmt.Sprintf("%s %d IN A %s", name, record.TTL, record.Value))
		case "CNAME":
			rr, err = dns.NewRR(fmt.Sprintf("%s %d IN CNAME %s", name, record.TTL, canonical(record.Value)))
		}
		if err != nil || rr == nil {
			continue
		}
		if result.Records[name] == nil {
			result.Records[name] = map[uint16][]dns.RR{}
		}
		result.Records[name][rr.Header().Rrtype] = append(result.Records[name][rr.Header().Rrtype], rr)
		result.RecordCount++
	}
	return result, nil
}

func (m *Manager) Start(settings Settings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	settings = normalizeSettings(settings)
	if !settings.Enabled {
		return nil
	}
	if err := m.stopLocked(); err != nil {
		return err
	}
	address := net.JoinHostPort(settings.ListenAddress, strconv.Itoa(settings.ListenPort))
	packet, err := net.ListenPacket("udp", address)
	if err != nil {
		return fmt.Errorf("UDP %s 启动失败: %w", address, err)
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		_ = packet.Close()
		return fmt.Errorf("TCP %s 启动失败: %w", address, err)
	}
	handler := dns.HandlerFunc(m.handle)
	m.udp = &dns.Server{PacketConn: packet, Handler: handler}
	m.tcp = &dns.Server{Listener: listener, Handler: handler}
	m.settings = settings
	m.running = true
	m.lastError = ""
	go func() {
		if err := m.udp.ActivateAndServe(); err != nil && !strings.Contains(strings.ToLower(err.Error()), "closed") {
			m.setRuntimeError(err)
		}
	}()
	go func() {
		if err := m.tcp.ActivateAndServe(); err != nil && !strings.Contains(strings.ToLower(err.Error()), "closed") {
			m.setRuntimeError(err)
		}
	}()
	return nil
}

func (m *Manager) Apply(settings Settings) error {
	settings = normalizeSettings(settings)
	if !settings.Enabled {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.settings = settings
		return m.stopLocked()
	}
	return m.Start(settings)
}
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLockedContext(ctx)
}
func (m *Manager) stopLocked() error { return m.stopLockedContext(context.Background()) }
func (m *Manager) stopLockedContext(ctx context.Context) error {
	var firstErr error
	if m.udp != nil {
		if err := m.udp.ShutdownContext(ctx); err != nil {
			firstErr = err
		}
		m.udp = nil
	}
	if m.tcp != nil {
		if err := m.tcp.ShutdownContext(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		m.tcp = nil
	}
	m.running = false
	return firstErr
}
func (m *Manager) setRuntimeError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastError = err.Error()
	m.running = false
}

func (m *Manager) Status() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := m.snapshot.Load()
	result := map[string]any{"enabled": m.settings.Enabled, "running": m.running, "udpRunning": m.running && m.udp != nil, "tcpRunning": m.running && m.tcp != nil, "listenAddress": net.JoinHostPort(m.settings.ListenAddress, strconv.Itoa(m.settings.ListenPort)), "upstreams": m.settings.Upstreams, "lastError": m.lastError, "lastCacheRefresh": m.lastRefresh}
	if snapshot != nil {
		result["zoneCount"] = snapshot.ZoneCount
		result["recordCount"] = snapshot.RecordCount
	}
	return result
}

func (m *Manager) handle(writer dns.ResponseWriter, request *dns.Msg) {
	response := m.Resolve(context.Background(), request)
	_ = writer.WriteMsg(response)
}
func (m *Manager) Resolve(ctx context.Context, request *dns.Msg) *dns.Msg {
	response := new(dns.Msg)
	response.SetReply(request)
	response.Authoritative = true
	if len(request.Question) == 0 {
		return response
	}
	question := request.Question[0]
	name := canonical(question.Name)
	snapshot := m.snapshot.Load()
	if snapshot != nil {
		for _, zone := range snapshot.Zones {
			if name == zone || strings.HasSuffix(name, "."+zone) {
				records := snapshot.Records[name]
				if records != nil {
					answers := records[question.Qtype]
					if len(answers) == 0 && question.Qtype != dns.TypeCNAME {
						answers = records[dns.TypeCNAME]
					}
					for _, rr := range answers {
						response.Answer = append(response.Answer, dns.Copy(rr))
					}
					if len(response.Answer) > 0 {
						return response
					}
				}
				response.Rcode = dns.RcodeNameError
				return response
			}
		}
	}
	forwarded, err := m.forward(ctx, request)
	if err != nil {
		response.Rcode = dns.RcodeServerFailure
		return response
	}
	return forwarded
}

func (m *Manager) forward(ctx context.Context, request *dns.Msg) (*dns.Msg, error) {
	if len(request.Question) == 0 {
		return nil, fmt.Errorf("empty DNS question")
	}
	key := strings.ToLower(request.Question[0].Name) + "|" + strconv.Itoa(int(request.Question[0].Qtype))
	m.cacheMu.RLock()
	cached, ok := m.cache[key]
	m.cacheMu.RUnlock()
	if ok && time.Now().Before(cached.expiresAt) {
		copy := cached.message.Copy()
		copy.Id = request.Id
		return copy, nil
	}
	m.mu.Lock()
	settings := m.settings
	m.mu.Unlock()
	if len(settings.Upstreams) == 0 {
		return nil, fmt.Errorf("未配置上游 DNS")
	}
	timeout := time.Duration(settings.TimeoutSeconds) * time.Second
	for _, upstream := range settings.Upstreams {
		query := request.Copy()
		client := &dns.Client{Net: "udp", Timeout: timeout}
		response, _, err := client.ExchangeContext(ctx, query, upstream)
		if err == nil && response != nil && response.Truncated {
			client.Net = "tcp"
			response, _, err = client.ExchangeContext(ctx, query, upstream)
		}
		if err != nil || response == nil {
			continue
		}
		ttl := responseTTL(response)
		copy := response.Copy()
		copy.Id = request.Id
		m.cacheMu.Lock()
		if len(m.cache) >= 5000 {
			m.cache = make(map[string]cacheEntry)
		}
		m.cache[key] = cacheEntry{message: copy.Copy(), expiresAt: time.Now().Add(ttl)}
		m.cacheMu.Unlock()
		return copy, nil
	}
	return nil, fmt.Errorf("所有上游 DNS 查询均失败")
}

func responseTTL(message *dns.Msg) time.Duration {
	ttl := uint32(300)
	found := false
	for _, section := range [][]dns.RR{message.Answer, message.Ns, message.Extra} {
		for _, rr := range section {
			if !found || rr.Header().Ttl < ttl {
				ttl = rr.Header().Ttl
				found = true
			}
		}
	}
	if ttl == 0 {
		ttl = 1
	}
	if ttl > 3600 {
		ttl = 3600
	}
	return time.Duration(ttl) * time.Second
}
func canonical(value string) string { return strings.ToLower(dns.Fqdn(strings.TrimSpace(value))) }
func recordName(host, zone string) string {
	host = strings.TrimSpace(host)
	if host == "" || host == "@" {
		return zone
	}
	if strings.HasSuffix(host, ".") {
		return canonical(host)
	}
	return canonical(host + "." + strings.TrimSuffix(zone, "."))
}
