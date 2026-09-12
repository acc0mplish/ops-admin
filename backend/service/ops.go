package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"ops-admin/backend/model"
)

type OpsScriptPayload struct {
	ID             uint                      `json:"id"`
	Name           string                    `json:"name"`
	ScriptType     string                    `json:"scriptType"`
	Interpreter    string                    `json:"interpreter"`
	Content        string                    `json:"content"`
	DefaultParams  string                    `json:"defaultParams"`
	Variables      []model.OpsScriptVariable `json:"variables"`
	TimeoutSeconds int                       `json:"timeoutSeconds"`
	Status         int                       `json:"status"`
	Description    string                    `json:"description"`
	ChangeSummary  string                    `json:"changeSummary"`
	Operator       string                    `json:"-"`
}

type OpsScriptStatusPayload struct {
	ID     uint `json:"id"`
	Status int  `json:"status"`
}

type OpsExecCommandPayload struct {
	Title          string `json:"title"`
	CommandText    string `json:"commandText"`
	Parameters     string `json:"parameters"`
	HostIDs        []uint `json:"hostIds"`
	GroupIDs       []uint `json:"groupIds"`
	Concurrency    int    `json:"concurrency"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	RiskConfirmed  bool   `json:"riskConfirmed"`
	Operator       string `json:"-"`
	SourceIP       string `json:"-"`
	Source         string `json:"-"`
	RetryOfTaskID  uint   `json:"-"`
}

type OpsExecScriptPayload struct {
	Title          string            `json:"title"`
	ScriptID       uint              `json:"scriptId"`
	Parameters     string            `json:"parameters"`
	Variables      map[string]string `json:"variables"`
	HostIDs        []uint            `json:"hostIds"`
	GroupIDs       []uint            `json:"groupIds"`
	Concurrency    int               `json:"concurrency"`
	TimeoutSeconds int               `json:"timeoutSeconds"`
	RiskConfirmed  bool              `json:"riskConfirmed"`
	Operator       string            `json:"-"`
	SourceIP       string            `json:"-"`
	Source         string            `json:"-"`
	RetryOfTaskID  uint              `json:"-"`
}

var opsScriptVariableName = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

type OpsFileDispatchPayload struct {
	Title          string `json:"title"`
	SourceType     string `json:"sourceType"`
	SourceHostID   uint   `json:"sourceHostId"`
	SourcePath     string `json:"sourcePath"`
	TargetPath     string `json:"targetPath"`
	HostIDs        []uint `json:"hostIds"`
	GroupIDs       []uint `json:"groupIds"`
	Concurrency    int    `json:"concurrency"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	Overwrite      bool   `json:"overwrite"`
	RiskConfirmed  bool   `json:"riskConfirmed"`
	Operator       string `json:"-"`
	SourceIP       string `json:"-"`
	Source         string `json:"-"`
}

type OpsExecRetryPayload struct {
	TaskID   uint   `json:"taskId"`
	Operator string `json:"-"`
	SourceIP string `json:"-"`
}

func normalizeOpsScriptType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "python", "python3":
		return "python"
	default:
		return "shell"
	}
}

func normalizeOpsInterpreter(v string, scriptType string) string {
	interpreter := strings.ToLower(strings.TrimSpace(v))
	if normalizeOpsScriptType(scriptType) == "python" {
		if slices.Contains([]string{"python", "python3"}, interpreter) {
			return interpreter
		}
		return "python"
	}
	if slices.Contains([]string{"bash", "sh"}, interpreter) {
		return interpreter
	}
	return "bash"
}

func normalizeOpsScriptTimeout(v int) int {
	if v <= 0 {
		return 300
	}
	if v < 30 {
		return 30
	}
	if v > 3600 {
		return 3600
	}
	return v
}

func normalizeOpsScriptVariables(values []model.OpsScriptVariable) ([]model.OpsScriptVariable, error) {
	if len(values) > 30 {
		return nil, errors.New("at most 30 execution variables may be defined")
	}
	result := make([]model.OpsScriptVariable, 0, len(values))
	seen := map[string]bool{}
	for _, item := range values {
		item.Name = strings.ToUpper(strings.TrimSpace(item.Name))
		item.Description = Trimmed(item.Description)
		if !opsScriptVariableName.MatchString(item.Name) {
			return nil, fmt.Errorf("invalid variable name %q; use uppercase letters, digits, and underscores, starting with a letter", item.Name)
		}
		if seen[item.Name] {
			return nil, fmt.Errorf("duplicate variable %s", item.Name)
		}
		if len(item.DefaultValue) > 4096 || len(item.Description) > 255 {
			return nil, fmt.Errorf("default value or description for variable %s is too long", item.Name)
		}
		if item.Secret {
			item.DefaultValue = ""
		}
		seen[item.Name] = true
		result = append(result, item)
	}
	return result, nil
}

func resolveOpsScriptVariables(definitions []model.OpsScriptVariable, supplied map[string]string) (map[string]string, error) {
	resolved := make(map[string]string, len(definitions))
	allowed := make(map[string]bool, len(definitions))
	for _, definition := range definitions {
		allowed[definition.Name] = true
		value, exists := supplied[definition.Name]
		if !exists {
			value = definition.DefaultValue
		}
		if definition.Required && strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("execution variable %s is required", definition.Name)
		}
		if len(value) > 4096 {
			return nil, fmt.Errorf("value for execution variable %s is too long", definition.Name)
		}
		resolved[definition.Name] = value
	}
	for name := range supplied {
		if !allowed[name] {
			return nil, fmt.Errorf("variable %s is not defined by the script", name)
		}
	}
	return resolved, nil
}

func normalizeOpsConcurrency(v int) int {
	if v <= 0 {
		return 5
	}
	if v > 10 {
		return 10
	}
	return v
}

func normalizeOpsTimeout(v int) int {
	if v <= 0 {
		return 10
	}
	if v < 10 {
		return 10
	}
	if v > 3600 {
		return 3600
	}
	return v
}

func opsRiskLevel(content string) string {
	value := strings.ToLower(strings.TrimSpace(content))
	highRisk := []string{"rm -rf", "mkfs", "shutdown", "reboot", "init 0", "init 6", "dd if=", "iptables -f", "systemctl stop", "userdel", "drop database", "truncate table"}
	for _, keyword := range highRisk {
		if strings.Contains(value, keyword) {
			return "high"
		}
	}
	return "normal"
}

func requireOpsRiskConfirmation(content string, confirmed bool) (string, error) {
	level := opsRiskLevel(content)
	if level == "high" && !confirmed {
		return level, errors.New("high-risk operation detected; confirm the target scope and command before execution")
	}
	return level, nil
}

func opsTaskSource(value string) string {
	if strings.TrimSpace(value) == "" {
		return "quick_exec"
	}
	return strings.TrimSpace(value)
}

func opsTargetSnapshot(hosts []model.AssetHost) string {
	items := make([]map[string]any, 0, len(hosts))
	for _, host := range hosts {
		items = append(items, map[string]any{
			"id": host.ID, "hostName": host.HostName, "sshIp": host.SSHIP,
			"environment": host.Environment,
		})
	}
	data, _ := json.Marshal(items)
	return string(data)
}

func opsHasProductionTarget(hosts []model.AssetHost) bool {
	for _, host := range hosts {
		environment := strings.ToLower(strings.TrimSpace(host.Environment))
		if environment == "prod" || environment == "production" || strings.Contains(environment, "\u751f\u4ea7") {
			return true
		}
	}
	return false
}

func requireOpsProductionConfirmation(hosts []model.AssetHost, confirmed bool) error {
	if opsHasProductionTarget(hosts) && !confirmed {
		return errors.New("targets include production hosts; confirm the target scope before execution")
	}
	return nil
}

func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}
