package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"ops-admin/backend/model"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

type prometheusRuleDocument struct {
	Groups []struct {
		Name     string `yaml:"name"`
		Interval string `yaml:"interval"`
		Rules    []struct {
			Alert       string         `yaml:"alert"`
			Record      string         `yaml:"record"`
			Expr        yaml.Node      `yaml:"expr"`
			For         string         `yaml:"for"`
			Labels      map[string]any `yaml:"labels"`
			Annotations map[string]any `yaml:"annotations"`
		} `yaml:"rules"`
	} `yaml:"groups"`
}

type prometheusRuleExportDocument struct {
	Groups []prometheusRuleExportGroup `yaml:"groups"`
}

type prometheusRuleExportGroup struct {
	Name     string                     `yaml:"name"`
	Interval string                     `yaml:"interval,omitempty"`
	Rules    []prometheusRuleExportItem `yaml:"rules"`
}

type prometheusRuleExportItem struct {
	Alert       string         `yaml:"alert"`
	Expr        string         `yaml:"expr"`
	For         string         `yaml:"for,omitempty"`
	Labels      map[string]any `yaml:"labels,omitempty"`
	Annotations map[string]any `yaml:"annotations,omitempty"`
}

func parsePrometheusTemplateDuration(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parts := regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)(ms|s|m|h|d|w|y)`).FindAllStringSubmatch(value, -1)
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid duration %q", value)
	}
	consumed := ""
	total := float64(0)
	seconds := map[string]float64{"ms": .001, "s": 1, "m": 60, "h": 3600, "d": 86400, "w": 604800, "y": 31536000}
	for _, part := range parts {
		consumed += part[0]
		number, err := strconv.ParseFloat(part[1], 64)
		if err != nil {
			return 0, err
		}
		total += number * seconds[strings.ToLower(part[2])]
	}
	if !strings.EqualFold(consumed, value) {
		return 0, fmt.Errorf("invalid duration %q", value)
	}
	return int(total), nil
}

func jsonObjectText(value map[string]any) string {
	if value == nil {
		return "{}"
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func importedSeverity(labels map[string]any) string {
	value := strings.ToLower(strings.TrimSpace(fmt.Sprint(labels["severity"])))
	switch value {
	case "p0", "disaster", "emergency", "page":
		return "P0"
	case "p1", "critical", "fatal":
		return "P1"
	case "p3", "info", "notice":
		return "P3"
	default:
		return "P2"
	}
}

func splitPrometheusTemplateExpression(expression string) (string, string, float64) {
	expression = strings.TrimSpace(expression)
	matcher := regexp.MustCompile(`(?s)^(.+?)\s*(==|!=|>=|<=|>|<)\s*(?:bool\s+)?(-?(?:\d+(?:\.\d*)?|\.\d+))\s*$`)
	parts := matcher.FindStringSubmatch(expression)
	if len(parts) != 4 {
		return expression, ">", 0
	}
	threshold, err := strconv.ParseFloat(parts[3], 64)
	if err != nil {
		return expression, ">", 0
	}
	return strings.TrimSpace(parts[1]), parts[2], threshold
}

func ParsePrometheusAlertTemplates(content []byte) ([]MonitorAlertTemplateImportItem, error) {
	var document prometheusRuleDocument
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("failed to parse Prometheus YAML: %w", err)
	}
	if len(document.Groups) == 0 {
		return nil, errors.New("no groups were found; select a Prometheus rule YAML file")
	}
	items := make([]MonitorAlertTemplateImportItem, 0)
	for _, group := range document.Groups {
		interval, err := parsePrometheusTemplateDuration(group.Interval)
		if err != nil {
			return nil, fmt.Errorf("invalid interval for rule group %s: %w", group.Name, err)
		}
		if interval < 15 {
			interval = 60
		}
		for _, rule := range group.Rules {
			if strings.TrimSpace(rule.Alert) == "" {
				continue // recording rules are not alert templates
			}
			expression := strings.TrimSpace(rule.Expr.Value)
			if expression == "" {
				return nil, fmt.Errorf("rule %s is missing expr", rule.Alert)
			}
			duration, err := parsePrometheusTemplateDuration(rule.For)
			if err != nil {
				return nil, fmt.Errorf("invalid for duration for rule %s: %w", rule.Alert, err)
			}
			queryText, comparator, threshold := splitPrometheusTemplateExpression(expression)
			description := strings.TrimSpace(fmt.Sprint(rule.Annotations["description"]))
			if description == "<nil>" || description == "" {
				description = strings.TrimSpace(fmt.Sprint(rule.Annotations["summary"]))
			}
			if description == "<nil>" {
				description = ""
			}
			items = append(items, MonitorAlertTemplateImportItem{
				Name: strings.TrimSpace(rule.Alert), PrometheusGroup: strings.TrimSpace(group.Name), OriginalExpression: expression,
				QueryText: queryText, Comparator: comparator, Threshold: threshold, ForSeconds: duration,
				EvalIntervalSeconds: interval, Severity: importedSeverity(rule.Labels), LabelsJSON: jsonObjectText(rule.Labels),
				AnnotationsJSON: jsonObjectText(rule.Annotations), Description: description,
			})
		}
	}
	if len(items) == 0 {
		return nil, errors.New("the file contains no alert rules; recording rules are not imported as alert templates")
	}
	return items, nil
}

func (s *Service) ImportPrometheusAlertTemplates(payload MonitorAlertTemplateImportPayload) (map[string]any, error) {
	if payload.GroupID == 0 {
		return nil, errors.New("select a target template group")
	}
	if len(payload.Items) == 0 {
		return nil, errors.New("select at least one Prometheus alert rule")
	}
	if len(payload.Items) > 500 {
		return nil, errors.New("at most 500 alert rules can be imported at once")
	}
	category, collector, err := s.monitorAlertTemplateGroupMeta(payload.GroupID)
	if err != nil {
		return nil, err
	}
	strategy := strings.ToLower(strings.TrimSpace(payload.DuplicateStrategy))
	if strategy != "rename" {
		strategy = "skip"
	}
	created, skipped := 0, 0
	err = s.db.Transaction(func(tx *gorm.DB) error {
		for _, source := range payload.Items {
			name := Trimmed(source.Name)
			queryText := strings.TrimSpace(source.QueryText)
			if name == "" || queryText == "" {
				return errors.New("imported item name and query are required")
			}
			var count int64
			if err := tx.Model(&model.MonitorAlertTemplate{}).Where("group_id = ? AND name = ?", payload.GroupID, name).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 && strategy == "skip" {
				skipped++
				continue
			}
			if count > 0 {
				base := name
				for index := 2; count > 0; index++ {
					name = fmt.Sprintf("%s (Import %d)", base, index)
					if err := tx.Model(&model.MonitorAlertTemplate{}).Where("group_id = ? AND name = ?", payload.GroupID, name).Count(&count).Error; err != nil {
						return err
					}
				}
			}
			labels := strings.TrimSpace(source.LabelsJSON)
			annotations := strings.TrimSpace(source.AnnotationsJSON)
			if !json.Valid([]byte(labels)) || !json.Valid([]byte(annotations)) {
				return fmt.Errorf("labels or annotations for rule %s are not valid JSON", name)
			}
			item := model.MonitorAlertTemplate{
				GroupID: payload.GroupID, Name: name, Category: category, Collector: collector, ObjectType: "",
				DatasourceType: "prometheus", QueryText: queryText, Comparator: firstNonEmpty(source.Comparator, ">"), Threshold: source.Threshold,
				ForSeconds: source.ForSeconds, EvalIntervalSeconds: source.EvalIntervalSeconds, Severity: firstNonEmpty(source.Severity, "P2"),
				LabelsJSON: labels, AnnotationsJSON: annotations, Description: Trimmed(source.Description), Source: "custom", Status: 1,
			}
			if item.ForSeconds < 0 {
				item.ForSeconds = 0
			}
			if item.EvalIntervalSeconds < 15 {
				item.EvalIntervalSeconds = 60
			}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
			created++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"created": created, "skipped": skipped, "total": len(payload.Items)}, nil
}

func prometheusExportDuration(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	if seconds%3600 == 0 {
		return fmt.Sprintf("%dh", seconds/3600)
	}
	if seconds%60 == 0 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf("%ds", seconds)
}

func prometheusExportObject(raw string) map[string]any {
	values := map[string]any{}
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &values)
	}
	return values
}

// ExportPrometheusAlertTemplates emits standard Prometheus Rule YAML, which can be
// pasted back into the template import dialog of another Ops Admin instance.
func (s *Service) ExportPrometheusAlertTemplates(ids []uint) ([]byte, error) {
	ids, err := normalizeMonitorBatchIDs(ids)
	if err != nil {
		return nil, errors.New("select at least one alert template")
	}
	if len(ids) > 500 {
		return nil, errors.New("at most 500 alert templates can be exported at once")
	}

	var templates []model.MonitorAlertTemplate
	if err := s.db.Where("id IN ?", ids).Find(&templates).Error; err != nil {
		return nil, err
	}
	if len(templates) != len(ids) {
		return nil, errors.New("some alert templates do not exist or were deleted")
	}
	for _, item := range templates {
		if item.DatasourceType != "prometheus" && item.DatasourceType != "victoriametrics" {
			return nil, fmt.Errorf("template %q is not Prometheus or VictoriaMetrics and cannot be exported as Prometheus rule YAML", item.Name)
		}
	}

	var groups []model.MonitorAlertTemplateGroup
	if err := s.db.Find(&groups).Error; err != nil {
		return nil, err
	}
	groupByID := make(map[uint]model.MonitorAlertTemplateGroup, len(groups))
	for _, group := range groups {
		groupByID[group.ID] = group
	}
	groupPath := func(id uint) string {
		parts := make([]string, 0, 2)
		for group, ok := groupByID[id]; ok; group, ok = groupByID[group.ParentID] {
			parts = append([]string{group.Name}, parts...)
			if group.ParentID == 0 {
				break
			}
		}
		return strings.Join(parts, " / ")
	}

	sort.Slice(templates, func(i, j int) bool {
		if templates[i].GroupID == templates[j].GroupID {
			return templates[i].Name < templates[j].Name
		}
		return templates[i].GroupID < templates[j].GroupID
	})
	document := prometheusRuleExportDocument{}
	groupIndexes := map[string]int{}
	for _, item := range templates {
		interval := item.EvalIntervalSeconds
		if interval < 15 {
			interval = 60
		}
		name := groupPath(item.GroupID)
		if name == "" {
			name = "ops-admin-export"
		}
		key := fmt.Sprintf("%d/%d", item.GroupID, interval)
		index, ok := groupIndexes[key]
		if !ok {
			index = len(document.Groups)
			groupIndexes[key] = index
			document.Groups = append(document.Groups, prometheusRuleExportGroup{Name: name, Interval: prometheusExportDuration(interval)})
		}
		labels := prometheusExportObject(item.LabelsJSON)
		if monitorSourceString(labels["severity"]) == "" {
			labels["severity"] = strings.ToLower(item.Severity)
		}
		annotations := prometheusExportObject(item.AnnotationsJSON)
		if monitorSourceString(annotations["description"]) == "" && strings.TrimSpace(item.Description) != "" {
			annotations["description"] = item.Description
		}
		expression := strings.TrimSpace(item.QueryText)
		if item.Comparator != "" {
			expression += " " + item.Comparator + " " + strconv.FormatFloat(item.Threshold, 'f', -1, 64)
		}
		document.Groups[index].Rules = append(document.Groups[index].Rules, prometheusRuleExportItem{
			Alert: item.Name, Expr: expression, For: prometheusExportDuration(item.ForSeconds), Labels: labels, Annotations: annotations,
		})
	}
	content, err := yaml.Marshal(document)
	if err != nil {
		return nil, err
	}
	return append([]byte("# Exported by Ops Admin alert template library.\n# Paste this content into Alert Templates > Paste Prometheus Template.\n\n"), content...), nil
}

func (s *Service) ListMonitorAlertTemplates(pageNum, pageSize int, keyword, category, datasourceType, source string, groupID uint) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.MonitorAlertTemplate{})
	if value := strings.TrimSpace(keyword); value != "" {
		like := "%" + value + "%"
		query = query.Where("name LIKE ? OR description LIKE ? OR collector LIKE ? OR query_text LIKE ?", like, like, like, like)
	}
	if value := strings.TrimSpace(category); value != "" {
		query = query.Where("category = ?", value)
	}
	if value := strings.TrimSpace(datasourceType); value != "" {
		query = query.Where("datasource_type = ?", value)
	}
	if value := strings.TrimSpace(source); value != "" {
		query = query.Where("source = ?", value)
	}
	if groupID > 0 {
		var groups []model.MonitorAlertTemplateGroup
		if err := s.db.Find(&groups).Error; err != nil {
			return nil, err
		}
		children := make(map[uint][]uint)
		for _, item := range groups {
			children[item.ParentID] = append(children[item.ParentID], item.ID)
		}
		ids := []uint{groupID}
		for index := 0; index < len(ids); index++ {
			ids = append(ids, children[ids[index]]...)
		}
		query = query.Where("group_id IN ?", ids)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.MonitorAlertTemplate
	if err := query.Order("source ASC, category ASC, id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) ListMonitorAlertTemplateGroups() ([]map[string]any, error) {
	var items []model.MonitorAlertTemplateGroup
	if err := s.db.Order("parent_id ASC, sort ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	var counts []struct {
		GroupID uint
		Count   int64
	}
	_ = s.db.Model(&model.MonitorAlertTemplate{}).Select("group_id, COUNT(*) AS count").Group("group_id").Scan(&counts).Error
	countMap := map[uint]int64{}
	for _, item := range counts {
		countMap[item.GroupID] = item.Count
	}
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		rows = append(rows, map[string]any{"id": item.ID, "parentId": item.ParentID, "name": item.Name, "count": countMap[item.ID]})
	}
	return rows, nil
}

func (s *Service) SaveMonitorAlertTemplateGroup(payload MonitorAlertTemplateGroupPayload) (*model.MonitorAlertTemplateGroup, error) {
	name := Trimmed(payload.Name)
	if name == "" {
		return nil, errors.New("group name is required")
	}
	if payload.ID > 0 {
		var current model.MonitorAlertTemplateGroup
		if err := s.db.First(&current, payload.ID).Error; err != nil {
			return nil, err
		}
		var duplicate int64
		if err := s.db.Model(&model.MonitorAlertTemplateGroup{}).Where("parent_id = ? AND name = ? AND id <> ?", current.ParentID, name, payload.ID).Count(&duplicate).Error; err != nil {
			return nil, err
		}
		if duplicate > 0 {
			return nil, errors.New("a group with the same name already exists at this level")
		}
		if err := s.db.Model(&model.MonitorAlertTemplateGroup{}).Where("id = ?", payload.ID).Updates(map[string]any{"name": name}).Error; err != nil {
			return nil, err
		}
		var item model.MonitorAlertTemplateGroup
		err := s.db.First(&item, payload.ID).Error
		return &item, err
	}
	if payload.ParentID > 0 {
		var parent model.MonitorAlertTemplateGroup
		if err := s.db.First(&parent, payload.ParentID).Error; err != nil {
			return nil, errors.New("parent group does not exist")
		}
		if parent.ParentID > 0 {
			return nil, errors.New("template groups support two levels only")
		}
	}
	var duplicate int64
	if err := s.db.Model(&model.MonitorAlertTemplateGroup{}).Where("parent_id = ? AND name = ?", payload.ParentID, name).Count(&duplicate).Error; err != nil {
		return nil, err
	}
	if duplicate > 0 {
		return nil, errors.New("a group with the same name already exists at this level")
	}
	item := model.MonitorAlertTemplateGroup{ParentID: payload.ParentID, Name: name}
	if err := s.db.Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) DeleteMonitorAlertTemplateGroup(id uint) error {
	var childCount, templateCount int64
	_ = s.db.Model(&model.MonitorAlertTemplateGroup{}).Where("parent_id = ?", id).Count(&childCount).Error
	_ = s.db.Model(&model.MonitorAlertTemplate{}).Where("group_id = ?", id).Count(&templateCount).Error
	if childCount > 0 || templateCount > 0 {
		return errors.New("group contains child groups or templates and cannot be deleted")
	}
	return s.db.Delete(&model.MonitorAlertTemplateGroup{}, id).Error
}

func (s *Service) monitorAlertTemplateGroupMeta(groupID uint) (string, string, error) {
	var groups []model.MonitorAlertTemplateGroup
	if err := s.db.Find(&groups).Error; err != nil {
		return "", "", err
	}
	byID := make(map[uint]model.MonitorAlertTemplateGroup, len(groups))
	for _, item := range groups {
		byID[item.ID] = item
	}
	path := []model.MonitorAlertTemplateGroup{}
	for currentID := groupID; currentID > 0; {
		item, ok := byID[currentID]
		if !ok {
			return "", "", errors.New("template group does not exist")
		}
		path = append([]model.MonitorAlertTemplateGroup{item}, path...)
		currentID = item.ParentID
	}
	if len(path) < 2 {
		return "", "", errors.New("select a collector child group under the component category")
	}
	return path[0].Name, path[1].Name, nil
}

func (s *Service) GetMonitorAlertTemplate(id uint) (*model.MonitorAlertTemplate, error) {
	var item model.MonitorAlertTemplate
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) SaveMonitorAlertTemplate(payload MonitorAlertTemplatePayload) (*model.MonitorAlertTemplate, error) {
	if strings.TrimSpace(payload.Name) == "" {
		return nil, errors.New("template name is required")
	}
	if payload.GroupID == 0 {
		return nil, errors.New("template group is required")
	}
	category, collector, err := s.monitorAlertTemplateGroupMeta(payload.GroupID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(payload.QueryText) == "" {
		return nil, errors.New("query text is required")
	}
	updates := map[string]any{
		"group_id": payload.GroupID,
		"name":     Trimmed(payload.Name), "category": category, "collector": collector,
		"object_type": "", "datasource_type": Trimmed(payload.DatasourceType), "query_text": strings.TrimSpace(payload.QueryText),
		"comparator": firstNonEmpty(Trimmed(payload.Comparator), ">"), "threshold": payload.Threshold, "for_seconds": payload.ForSeconds,
		"eval_interval_seconds": payload.EvalIntervalSeconds, "severity": firstNonEmpty(Trimmed(payload.Severity), "P2"),
		"labels_json": strings.TrimSpace(payload.LabelsJSON), "annotations_json": strings.TrimSpace(payload.AnnotationsJSON),
		"description": Trimmed(payload.Description), "status": normalizeMonitorStatus(payload.Status),
	}
	if payload.ID > 0 {
		current, err := s.GetMonitorAlertTemplate(payload.ID)
		if err != nil {
			return nil, err
		}
		if current.Source == "platform" {
			return nil, errors.New("platform template is read-only; copy it before editing")
		}
		if err := s.db.Model(&model.MonitorAlertTemplate{}).Where("id = ?", payload.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
		return s.GetMonitorAlertTemplate(payload.ID)
	}
	item := model.MonitorAlertTemplate{
		GroupID: payload.GroupID,
		Name:    updates["name"].(string), Category: updates["category"].(string), Collector: updates["collector"].(string),
		ObjectType: updates["object_type"].(string), DatasourceType: updates["datasource_type"].(string), QueryText: updates["query_text"].(string),
		Comparator: updates["comparator"].(string), Threshold: payload.Threshold, ForSeconds: payload.ForSeconds,
		EvalIntervalSeconds: payload.EvalIntervalSeconds, Severity: updates["severity"].(string), LabelsJSON: strings.TrimSpace(payload.LabelsJSON),
		AnnotationsJSON: strings.TrimSpace(payload.AnnotationsJSON), Description: updates["description"].(string), Status: normalizeMonitorStatus(payload.Status), Source: "custom",
	}
	if item.ForSeconds < 0 {
		item.ForSeconds = 0
	}
	if item.EvalIntervalSeconds < 15 {
		item.EvalIntervalSeconds = 60
	}
	if err := s.db.Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) DeleteMonitorAlertTemplate(id uint) error {
	item, err := s.GetMonitorAlertTemplate(id)
	if err != nil {
		return err
	}
	if item.Source == "platform" {
		return errors.New("platform template cannot be deleted")
	}
	return s.db.Delete(&model.MonitorAlertTemplate{}, id).Error
}
