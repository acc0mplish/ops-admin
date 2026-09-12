package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/model"
)

func aiAssetQueryLimit(value any) int {
	limit := int(anyUint(value))
	if limit < 1 {
		return 20
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func (s *Service) queryAIAssetHosts(args map[string]any) (map[string]any, error) {
	keyword := strings.TrimSpace(anyString(args["keyword"]))
	environment := strings.TrimSpace(anyString(args["environment"]))
	limit := aiAssetQueryLimit(args["limit"])
	query := s.db.Model(&model.AssetHost{}).Preload("Group").Preload("HostGroups")
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("host_name LIKE ? OR alias LIKE ? OR private_ip LIKE ? OR public_ip LIKE ? OR ssh_ip LIKE ?", like, like, like, like, like)
	}
	if environment != "" {
		query = query.Where("environment = ?", normalizeEnvCode(environment))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var hosts []model.AssetHost
	if err := query.Order("id DESC").Limit(limit).Find(&hosts).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(hosts))
	for _, host := range hosts {
		groupNames := make([]string, 0, len(host.HostGroups)+1)
		if host.Group.ID > 0 {
			groupNames = append(groupNames, host.Group.Name)
		}
		for _, group := range host.HostGroups {
			if group.Name != "" && !aiStringSliceContains(groupNames, group.Name) {
				groupNames = append(groupNames, group.Name)
			}
		}
		items = append(items, map[string]any{
			"id": host.ID, "name": host.HostName, "alias": host.Alias,
			"privateIp": host.PrivateIP, "publicIp": host.PublicIP, "sshPort": host.SSHPort,
			"os": host.OS, "arch": host.Arch, "cpu": host.CPU, "memory": host.Memory, "disk": host.Disk,
			"environment": host.Environment, "groups": groupNames,
			"enabled": host.Status == 1, "online": host.AliveStatus == 1, "lastCheckTime": host.LastCheckTime,
		})
	}
	return map[string]any{"resourceType": "server", "total": total, "returned": len(items), "items": items}, nil
}

func (s *Service) queryAIAssetDatabases(dbType string, args map[string]any) (map[string]any, error) {
	keyword := strings.TrimSpace(anyString(args["keyword"]))
	environment := strings.TrimSpace(anyString(args["environment"]))
	limit := aiAssetQueryLimit(args["limit"])
	normalizedType := normalizeDatabaseType(dbType)
	query := s.db.Model(&model.AssetDatabase{}).Where("db_type = ?", normalizedType)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR host LIKE ? OR db_name LIKE ?", like, like, like)
	}
	if environment != "" {
		query = query.Where("env = ?", normalizeEnvCode(environment))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var databases []model.AssetDatabase
	if err := query.Order("id DESC").Limit(limit).Find(&databases).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(databases))
	for _, database := range databases {
		items = append(items, map[string]any{
			"id": database.ID, "name": database.Name, "type": normalizedType,
			"host": database.Host, "port": database.Port, "database": database.DBName,
			"environment": database.Env, "tags": database.Tags, "version": database.Version,
			"accessMode": database.AccessMode, "connectionMode": database.ConnectionMode,
			"enabled": database.Status == 1, "connected": database.ConnectStatus == 1,
			"connectStatus": database.ConnectStatus, "lastCheckTime": database.LastCheckTime,
		})
	}
	return map[string]any{"resourceType": "database", "databaseType": normalizedType, "total": total, "returned": len(items), "items": items}, nil
}

func aiStringSliceContains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func anyString(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
func anyUint(value any) uint {
	switch item := value.(type) {
	case float64:
		return uint(item)
	case int:
		return uint(item)
	case uint:
		return item
	case json.Number:
		v, _ := item.Int64()
		return uint(v)
	default:
		var result uint
		_, _ = fmt.Sscan(fmt.Sprint(value), &result)
		return result
	}
}

func anyFloat(value any) float64 {
	if value == nil {
		return 0
	}
	switch item := value.(type) {
	case float64:
		return item
	case float32:
		return float64(item)
	case int:
		return float64(item)
	case json.Number:
		parsed, _ := item.Float64()
		return parsed
	default:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
		return parsed
	}
}

func anyBool(value any) bool {
	switch item := value.(type) {
	case bool:
		return item
	case string:
		return strings.EqualFold(strings.TrimSpace(item), "true") || strings.TrimSpace(item) == "1"
	default:
		return false
	}
}

func (s *Service) ConfirmIntegrationAIToolAction(userID uint, username string, id uint) (map[string]any, error) {
	var action model.IntegrationAIToolAction
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&action).Error; err != nil {
		return nil, err
	}
	if action.Status != "pending" {
		return nil, errors.New("action has already been processed")
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(action.ArgumentsJSON), &args); err != nil {
		return nil, err
	}
	payload := model.K8sWorkloadActionPayload{ClusterID: anyUint(args["clusterId"]), Namespace: anyString(args["namespace"]), WorkloadType: anyString(args["workloadType"]), WorkloadName: anyString(args["workloadName"]), Replicas: int(anyUint(args["replicas"]))}
	var result map[string]any
	var err error
	switch action.ToolKey {
	case "k8s_restart_workload":
		result, err = s.RestartK8sWorkload(payload)
	case "k8s_scale_workload":
		result, err = s.ScaleK8sWorkload(payload)
	case "monitor_alert_rule_draft":
		result, err = s.createAIMonitorAlertRuleDraft(args)
	default:
		err = errors.New("unsupported pending confirmation action")
	}
	now := time.Now()
	action.ConfirmedAt, action.ConfirmedBy = &now, username
	if err != nil {
		action.Status = "failed"
		action.ResultJSON = err.Error()
	} else {
		action.Status = "completed"
		raw, _ := json.Marshal(result)
		action.ResultJSON = string(raw)
	}
	_ = s.db.Save(&action).Error
	if err != nil {
		return nil, err
	}
	return map[string]any{"action": action, "result": result}, nil
}

func (s *Service) createAIMonitorAlertRuleDraft(args map[string]any) (map[string]any, error) {
	name := strings.TrimSpace(anyString(args["name"]))
	promQL := strings.TrimSpace(anyString(args["promql"]))
	datasourceID := anyUint(args["datasourceId"])
	if name == "" || promQL == "" || datasourceID == 0 {
		return nil, errors.New("rule name, datasource, and PromQL are required")
	}
	payload := MonitorAlertRulePayload{
		Name: name, AlertType: "metric", DatasourceScope: "specific", DatasourceID: datasourceID, PromQL: promQL,
		Comparator: firstNonEmpty(strings.TrimSpace(anyString(args["comparator"])), ">"), Threshold: anyFloat(args["threshold"]),
		ForSeconds: int(anyUint(args["forSeconds"])), Severity: firstNonEmpty(strings.TrimSpace(anyString(args["severity"])), "P2"),
		Env: strings.TrimSpace(anyString(args["env"])), Description: strings.TrimSpace(anyString(args["description"])),
		LabelsJSON: "{}", AnnotationsJSON: "{}", NotifyEnabled: false, NotifyRecoveryEnabled: true, Status: 2,
	}
	if err := s.SaveMonitorAlertRule(payload); err != nil {
		return nil, err
	}
	var rule model.MonitorAlertRule
	if err := s.db.Where("name = ?", name).Order("id DESC").First(&rule).Error; err != nil {
		return nil, err
	}
	return map[string]any{"id": rule.ID, "name": rule.Name, "status": "draft_disabled", "message": "Alert Rule Draft를 비활성 상태로 저장했으며 Notification은 활성화하지 않았습니다. Alert Rule Page에서 검토한 뒤 활성화하십시오."}, nil
}

func (s *Service) RejectIntegrationAIToolAction(userID, id uint) error {
	return s.db.Model(&model.IntegrationAIToolAction{}).Where("id = ? AND user_id = ? AND status = ?", id, userID, "pending").Update("status", "rejected").Error
}
