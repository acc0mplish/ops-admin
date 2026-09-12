package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/miekg/dns"
	"gorm.io/gorm"
	"ops-admin/backend/internal/domain/dnsserver"
	"ops-admin/backend/model"
)

func (s *Service) GetInternalDNSSettings() (map[string]any, error) {
	settings, err := dnsserver.LoadSettings(s.db)
	if err != nil {
		return nil, err
	}
	return map[string]any{"settings": settings, "status": s.dnsManager.Status()}, nil
}
func (s *Service) SaveInternalDNSSettings(payload InternalDNSSettingsPayload, actor DNSAuditActor) error {
	// DNS uses the standard port by product definition. Never trust a client
	// supplied port, including direct API calls that bypass the UI.
	payload.ListenPort = 53
	oldSettings, loadErr := dnsserver.LoadSettings(s.db)
	if loadErr != nil {
		return loadErr
	}
	settings := dnsserver.Settings{Enabled: payload.Enabled, ListenAddress: payload.ListenAddress, ListenPort: 53, Upstreams: payload.Upstreams, TimeoutSeconds: payload.TimeoutSeconds}
	data, _ := json.Marshal(payload.Upstreams)
	item := model.InternalDNSSetting{ID: 1, Enabled: payload.Enabled, ListenAddress: payload.ListenAddress, ListenPort: 53, UpstreamsJSON: string(data), TimeoutSeconds: payload.TimeoutSeconds}
	if err := s.dnsManager.Apply(settings); err != nil {
		_ = s.db.Model(&model.InternalDNSSetting{}).Where("id = ?", 1).Updates(map[string]any{"enabled": false, "last_error": err.Error()}).Error
		s.writeDNSAudit(actor, "Enable Internal DNS", "internal", "", "", "", "", "", err)
		return err
	}
	now := time.Now()
	updates := map[string]any{"enabled": item.Enabled, "listen_address": item.ListenAddress, "listen_port": item.ListenPort, "upstreams_json": item.UpstreamsJSON, "timeout_seconds": item.TimeoutSeconds, "last_error": ""}
	if item.Enabled {
		updates["last_started_at"] = &now
	}
	err := s.db.Model(&model.InternalDNSSetting{}).Where("id = ?", 1).Updates(updates).Error
	if err != nil {
		_ = s.dnsManager.Apply(oldSettings)
	}
	s.writeDNSAudit(actor, "Save Internal DNS Settings", "internal", "", "", "", "", "", err)
	return err
}

func (s *Service) ListInternalZones(keyword string) ([]model.InternalDNSZone, error) {
	query := s.db.Model(&model.InternalDNSZone{}).Select("domain_internal_dns_zone.*, COUNT(domain_internal_dns_record.id) AS record_count").Joins("LEFT JOIN domain_internal_dns_record ON domain_internal_dns_record.zone_id=domain_internal_dns_zone.id").Group("domain_internal_dns_zone.id")
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("domain_internal_dns_zone.name LIKE ?", "%"+keyword+"%")
	}
	list := make([]model.InternalDNSZone, 0)
	err := query.Order("domain_internal_dns_zone.name asc").Scan(&list).Error
	return list, err
}
func (s *Service) SaveInternalZone(payload InternalZonePayload, actor DNSAuditActor) error {
	name, err := normalizeZone(payload.Name)
	if err != nil {
		return err
	}
	if payload.Status == 0 {
		payload.Status = 1
	}
	var snapshot *dnsserver.Snapshot
	var old model.InternalDNSZone
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if payload.ID == 0 {
			old = model.InternalDNSZone{}
			if err := tx.Create(&model.InternalDNSZone{Name: name, Description: strings.TrimSpace(payload.Description), Status: payload.Status}).Error; err != nil {
				return err
			}
		} else {
			if err := tx.First(&old, payload.ID).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.InternalDNSZone{}).Where("id = ?", payload.ID).Updates(map[string]any{"name": name, "description": strings.TrimSpace(payload.Description), "status": payload.Status}).Error; err != nil {
				return err
			}
		}
		snapshot, err = dnsserver.BuildSnapshot(tx)
		return err
	})
	if err == nil {
		s.dnsManager.ReplaceSnapshot(snapshot)
	}
	s.writeDNSAudit(actor, map[bool]string{true: "Create Internal DNS Zone", false: "Update Internal DNS Zone"}[payload.ID == 0], "internal", name, name, "", old.Name, name, err)
	return err
}
func (s *Service) DeleteInternalZone(id uint, actor DNSAuditActor) error {
	var zone model.InternalDNSZone
	if err := s.db.First(&zone, id).Error; err != nil {
		return err
	}
	var snapshot *dnsserver.Snapshot
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("zone_id = ?", id).Delete(&model.InternalDNSRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&zone).Error; err != nil {
			return err
		}
		var buildErr error
		snapshot, buildErr = dnsserver.BuildSnapshot(tx)
		return buildErr
	})
	if err == nil {
		s.dnsManager.ReplaceSnapshot(snapshot)
	}
	s.writeDNSAudit(actor, "Delete Internal DNS Zone", "internal", zone.Name, zone.Name, "", zone.Name, "", err)
	return err
}
func (s *Service) ListInternalRecords(zoneID uint, keyword string) ([]model.InternalDNSRecord, error) {
	query := s.db.Where("zone_id = ?", zoneID)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("host LIKE ? OR value LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	list := make([]model.InternalDNSRecord, 0)
	err := query.Order("host asc,type asc").Find(&list).Error
	return list, err
}
func (s *Service) SaveInternalRecord(payload InternalRecordPayload, actor DNSAuditActor) error {
	var zone model.InternalDNSZone
	if err := s.db.First(&zone, payload.ZoneID).Error; err != nil {
		return errors.New("DNS zone does not exist")
	}
	payload.Host = strings.ToLower(strings.TrimSpace(payload.Host))
	payload.Type = strings.ToUpper(strings.TrimSpace(payload.Type))
	payload.Value = strings.TrimSpace(payload.Value)
	if payload.TTL == 0 {
		payload.TTL = 300
	}
	if payload.Status == 0 {
		payload.Status = 1
	}
	if err := validateDNSHost(payload.Host); err != nil {
		return err
	}
	if payload.Type == "A" {
		ip := net.ParseIP(payload.Value)
		if ip == nil || ip.To4() == nil {
			return errors.New("A record value must be a valid IPv4 address")
		}
	} else if payload.Type == "CNAME" {
		normalized, err := normalizeFQDN(payload.Value)
		if err != nil {
			return err
		}
		payload.Value = normalized
	} else {
		return errors.New("the current internal DNS phase supports only A and CNAME records")
	}
	var old model.InternalDNSRecord
	var snapshot *dnsserver.Snapshot
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := validateRecordConflict(tx, payload); err != nil {
			return err
		}
		if payload.Type == "CNAME" {
			if err := validateCNAMELoop(tx, payload, zone.Name); err != nil {
				return err
			}
		}
		item := model.InternalDNSRecord{ID: payload.ID, ZoneID: payload.ZoneID, Host: payload.Host, Type: payload.Type, Value: payload.Value, TTL: payload.TTL, Status: payload.Status}
		if payload.ID == 0 {
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Where("id = ? AND zone_id = ?", payload.ID, payload.ZoneID).First(&old).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errors.New("DNS record does not exist or does not belong to the current zone")
				}
				return err
			}
			if err := tx.Model(&model.InternalDNSRecord{}).Where("id = ? AND zone_id = ?", payload.ID, payload.ZoneID).Updates(map[string]any{"host": item.Host, "type": item.Type, "value": item.Value, "ttl": item.TTL, "status": item.Status}).Error; err != nil {
				return err
			}
		}
		var buildErr error
		snapshot, buildErr = dnsserver.BuildSnapshot(tx)
		return buildErr
	})
	if err == nil {
		s.dnsManager.ReplaceSnapshot(snapshot)
	}
	domain := internalFQDN(payload.Host, zone.Name)
	s.writeDNSAudit(actor, map[bool]string{true: "Create Internal DNS Record", false: "Update Internal DNS Record"}[payload.ID == 0], "internal", zone.Name, domain, payload.Type, old.Value, payload.Value, err)
	return err
}
func (s *Service) DeleteInternalRecord(id uint, actor DNSAuditActor) error {
	var item model.InternalDNSRecord
	if err := s.db.First(&item, id).Error; err != nil {
		return err
	}
	var zone model.InternalDNSZone
	_ = s.db.First(&zone, item.ZoneID).Error
	var snapshot *dnsserver.Snapshot
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}
		var buildErr error
		snapshot, buildErr = dnsserver.BuildSnapshot(tx)
		return buildErr
	})
	if err == nil {
		s.dnsManager.ReplaceSnapshot(snapshot)
	}
	s.writeDNSAudit(actor, "Delete Internal DNS Record", "internal", zone.Name, internalFQDN(item.Host, zone.Name), item.Type, item.Value, "", err)
	return err
}
func (s *Service) BatchInternalRecords(payload InternalRecordBatchPayload, actor DNSAuditActor) (int, error) {
	var zone model.InternalDNSZone
	if err := s.db.First(&zone, payload.ZoneID).Error; err != nil {
		return 0, errors.New("DNS zone does not exist")
	}
	action := strings.ToLower(strings.TrimSpace(payload.Action))
	actionNames := map[string]string{"create": "Batch Create Internal DNS Records", "update": "Batch Update Internal DNS Records", "delete": "Batch Delete Internal DNS Records", "enable": "Batch Enable Internal DNS Records", "disable": "Batch Disable Internal DNS Records"}
	actionName, ok := actionNames[action]
	if !ok {
		return 0, errors.New("unsupported batch operation")
	}
	var snapshot *dnsserver.Snapshot
	affected := 0
	err := s.db.Transaction(func(tx *gorm.DB) error {
		switch action {
		case "create", "update":
			if len(payload.Records) == 0 {
				return errors.New("provide at least one DNS record")
			}
			for index, record := range payload.Records {
				record.ZoneID = payload.ZoneID
				normalized, normalizeErr := normalizeInternalRecordPayload(record)
				if normalizeErr != nil {
					return fmt.Errorf("record %d: %w", index+1, normalizeErr)
				}
				if action == "create" {
					normalized.ID = 0
				} else if normalized.ID == 0 {
					return fmt.Errorf("record %d is missing an ID", index+1)
				} else {
					var existing model.InternalDNSRecord
					if findErr := tx.Where("id = ? AND zone_id = ?", normalized.ID, payload.ZoneID).First(&existing).Error; findErr != nil {
						return fmt.Errorf("record %d does not exist", index+1)
					}
				}
				if validateErr := validateRecordConflict(tx, normalized); validateErr != nil {
					return fmt.Errorf("record %d: %w", index+1, validateErr)
				}
				if normalized.Type == "CNAME" {
					if loopErr := validateCNAMELoop(tx, normalized, zone.Name); loopErr != nil {
						return fmt.Errorf("record %d: %w", index+1, loopErr)
					}
				}
				item := model.InternalDNSRecord{ID: normalized.ID, ZoneID: normalized.ZoneID, Host: normalized.Host, Type: normalized.Type, Value: normalized.Value, TTL: normalized.TTL, Status: normalized.Status}
				if action == "create" {
					if createErr := tx.Create(&item).Error; createErr != nil {
						return fmt.Errorf("failed to save record %d: %w", index+1, createErr)
					}
				} else if updateErr := tx.Model(&model.InternalDNSRecord{}).Where("id = ? AND zone_id = ?", item.ID, payload.ZoneID).Updates(map[string]any{"host": item.Host, "type": item.Type, "value": item.Value, "ttl": item.TTL, "status": item.Status}).Error; updateErr != nil {
					return fmt.Errorf("failed to save record %d: %w", index+1, updateErr)
				}
				affected++
			}
		case "delete", "enable", "disable":
			ids := uniqueUintIDs(payload.IDs)
			if len(ids) == 0 {
				return errors.New("select at least one DNS record")
			}
			var existing []model.InternalDNSRecord
			if findErr := tx.Where("zone_id = ? AND id IN ?", payload.ZoneID, ids).Find(&existing).Error; findErr != nil {
				return findErr
			}
			if len(existing) != len(ids) {
				return errors.New("some DNS records do not exist or do not belong to the current zone")
			}
			query := tx.Where("zone_id = ? AND id IN ?", payload.ZoneID, ids)
			if action == "delete" {
				if deleteErr := query.Delete(&model.InternalDNSRecord{}).Error; deleteErr != nil {
					return deleteErr
				}
			} else {
				status := 1
				if action == "disable" {
					status = 2
				}
				if updateErr := query.Model(&model.InternalDNSRecord{}).Update("status", status).Error; updateErr != nil {
					return updateErr
				}
			}
			affected = len(ids)
		}
		var buildErr error
		snapshot, buildErr = dnsserver.BuildSnapshot(tx)
		return buildErr
	})
	if err == nil {
		s.dnsManager.ReplaceSnapshot(snapshot)
	}
	s.writeDNSAudit(actor, actionName, "internal", zone.Name, zone.Name, "", fmt.Sprintf("%d records", affected), fmt.Sprintf("%d records", affected), err)
	return affected, err
}

func normalizeInternalRecordPayload(payload InternalRecordPayload) (InternalRecordPayload, error) {
	payload.Host = strings.ToLower(strings.TrimSpace(payload.Host))
	payload.Type = strings.ToUpper(strings.TrimSpace(payload.Type))
	payload.Value = strings.TrimSpace(payload.Value)
	if payload.TTL == 0 {
		payload.TTL = 300
	}
	if payload.Status == 0 {
		payload.Status = 1
	}
	if err := validateDNSHost(payload.Host); err != nil {
		return payload, err
	}
	if payload.Type == "A" {
		ip := net.ParseIP(payload.Value)
		if ip == nil || ip.To4() == nil {
			return payload, errors.New("A record value must be a valid IPv4 address")
		}
	} else if payload.Type == "CNAME" {
		normalized, err := normalizeFQDN(payload.Value)
		if err != nil {
			return payload, err
		}
		payload.Value = normalized
	} else {
		return payload, errors.New("the current internal DNS phase supports only A and CNAME records")
	}
	return payload, nil
}

func uniqueUintIDs(values []uint) []uint {
	seen := make(map[uint]struct{}, len(values))
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
func (s *Service) TestDNSResolution(domainName, recordType string) (map[string]any, error) {
	recordType = strings.ToUpper(strings.TrimSpace(recordType))
	qtype, ok := map[string]uint16{"A": dns.TypeA, "CNAME": dns.TypeCNAME}[recordType]
	if !ok {
		return nil, errors.New("test type supports only A or CNAME")
	}
	request := new(dns.Msg)
	request.SetQuestion(dns.Fqdn(domainName), qtype)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	response := s.dnsManager.Resolve(ctx, request)
	elapsed := float64(time.Since(start).Microseconds()) / 1000
	if response.Rcode != dns.RcodeSuccess {
		return map[string]any{"status": "failed", "rcode": dns.RcodeToString[response.Rcode], "responseTimeMs": elapsed}, nil
	}
	answers := []map[string]any{}
	for _, rr := range response.Answer {
		value := ""
		switch typed := rr.(type) {
		case *dns.A:
			value = typed.A.String()
		case *dns.CNAME:
			value = typed.Target
		default:
			value = rr.String()
		}
		answers = append(answers, map[string]any{"value": value, "ttl": rr.Header().Ttl, "type": dns.TypeToString[rr.Header().Rrtype]})
	}
	status := s.dnsManager.Status()
	return map[string]any{"status": "success", "dnsServer": status["listenAddress"], "answers": answers, "responseTimeMs": elapsed}, nil
}

func (s *Service) ListDNSAuditLogs(pageNum, pageSize int) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	if err := s.db.Model(&model.DNSAuditLog{}).Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.DNSAuditLog
	if err := s.db.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}
func (s *Service) writeDNSAudit(actor DNSAuditActor, action, providerName, zone, domainName, recordType, oldValue, newValue string, err error) {
	item := model.DNSAuditLog{AdminID: actor.AdminID, Username: actor.Username, IPAddress: actor.IP, Action: action, Provider: providerName, Zone: zone, Domain: domainName, RecordType: recordType, OldValue: oldValue, NewValue: newValue, Success: err == nil}
	if err != nil {
		item.Error = err.Error()
	}
	_ = s.db.Create(&item).Error
}

var dnsLabelPattern = regexp.MustCompile(`^[a-zA-Z0-9_*-](?:[a-zA-Z0-9_*-]{0,61}[a-zA-Z0-9_*-])?$`)

func validateDNSHost(value string) error {
	if value == "@" {
		return nil
	}
	if value == "" {
		return errors.New("host record is required")
	}
	for _, part := range strings.Split(value, ".") {
		if !dnsLabelPattern.MatchString(part) {
			return fmt.Errorf("invalid host record format: %q", value)
		}
	}
	return nil
}
func normalizeZone(value string) (string, error) {
	value = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
	if value == "" || !strings.Contains(value, ".") {
		return "", errors.New("zone must be a fully qualified domain name, for example ops.internal")
	}
	if err := validateDNSHost(value); err != nil {
		return "", err
	}
	return value, nil
}
func normalizeFQDN(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	trimmed := strings.TrimSuffix(value, ".")
	if !strings.Contains(trimmed, ".") {
		return "", errors.New("CNAME record value must be a fully qualified domain name")
	}
	if err := validateDNSHost(trimmed); err != nil {
		return "", err
	}
	return dns.Fqdn(trimmed), nil
}
func internalFQDN(host, zone string) string {
	if host == "@" || host == "" {
		return dns.Fqdn(zone)
	}
	return dns.Fqdn(host + "." + zone)
}
func validateRecordConflict(tx *gorm.DB, payload InternalRecordPayload) error {
	opposite := "CNAME"
	if payload.Type == "CNAME" {
		opposite = "A"
	}
	var count int64
	query := tx.Model(&model.InternalDNSRecord{}).Where("zone_id = ? AND host = ? AND type = ?", payload.ZoneID, payload.Host, opposite)
	if payload.ID > 0 {
		query = query.Where("id <> ?", payload.ID)
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("CNAME and A records cannot coexist for the same name")
	}
	return nil
}
func validateCNAMELoop(tx *gorm.DB, payload InternalRecordPayload, zone string) error {
	source := internalFQDN(payload.Host, zone)
	target := dns.Fqdn(strings.ToLower(payload.Value))
	if source == target {
		return errors.New("CNAME cannot point to itself")
	}
	var records []model.InternalDNSRecord
	if err := tx.Where("zone_id = ? AND type = 'CNAME'", payload.ZoneID).Find(&records).Error; err != nil {
		return err
	}
	edges := map[string]string{source: target}
	for _, record := range records {
		if record.ID != payload.ID {
			edges[internalFQDN(record.Host, zone)] = dns.Fqdn(strings.ToLower(record.Value))
		}
	}
	if cnameHasLoop(edges, source) {
		return errors.New("CNAME configuration creates a cycle")
	}
	return nil
}
func cnameHasLoop(edges map[string]string, source string) bool {
	seen := map[string]struct{}{}
	current := source
	for i := 0; i <= len(edges); i++ {
		if _, ok := seen[current]; ok {
			return true
		}
		seen[current] = struct{}{}
		next, ok := edges[current]
		if !ok {
			return false
		}
		current = next
	}
	return true
}
func maskDNSKey(value string) string {
	runes := []rune(value)
	if len(runes) <= 8 {
		return "****"
	}
	return string(runes[:4]) + "…" + string(runes[len(runes)-4:])
}
