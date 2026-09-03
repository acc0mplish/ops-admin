package apperr

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
)

var stableCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]+$`)

// FromLegacy converts a legacy user-facing error string into a stable API
// error code. The original text is retained only as the wrapped cause so the
// HTTP layer can keep diagnostics without returning implementation details to
// clients.
func FromLegacy(message string, status int) *Error {
	raw := strings.TrimSpace(message)
	code, params := classifyLegacy(raw, status)
	if raw == "" {
		return New(code, params)
	}
	return Wrap(code, errors.New(raw), params)
}

func classifyLegacy(message string, status int) (string, map[string]any) {
	if stableCodePattern.MatchString(message) {
		return message, nil
	}

	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "" {
		return defaultCodeForStatus(status), nil
	}

	switch {
	case containsAny(lower,
		"invalid startdate format",
		"invalid enddate format",
		"start date must be earlier than end date"):
		return "MONITOR_INVALID_DATE_RANGE", nil
	case lower == "datasource name is required":
		return "MONITOR_DATASOURCE_NAME_REQUIRED", nil
	case lower == "datasource url is required":
		return "MONITOR_DATASOURCE_URL_REQUIRED", nil
	case strings.HasPrefix(lower, "datasource is referenced by ") ||
		strings.Contains(lower, "datasource is bound to kubernetes cluster monitoring"):
		return "MONITOR_DATASOURCE_IN_USE", nil
	case lower == "query is required" ||
		lower == "promql is required" ||
		lower == "elasticsearch query is required" ||
		lower == "query text is required":
		return "MONITOR_QUERY_REQUIRED", nil
	case containsAny(lower,
		"chart queries support only prometheus or victoriametrics datasources",
		"trace queries support only jaeger datasources",
		"log queries support only elasticsearch or victorialogs datasources",
		"log query supports only elasticsearch datasources",
		"log fields support only elasticsearch or victorialogs datasources",
		"monitoring panels support only prometheus or victoriametrics",
		"log datasources do not support promql",
		"log alerts require an elasticsearch datasource",
		"victorialogs alerts require a victorialogs datasource",
		"metric alerts require a prometheus or victoriametrics datasource",
		"current datasource is not jaeger",
		"current datasource is not victorialogs",
		"current datasource is not elasticsearch",
		"use logsql in log explorer for victorialogs"):
		return "MONITOR_DATASOURCE_TYPE_UNSUPPORTED", nil
	case containsAny(lower,
		"query end time must be later than start time",
		"end time must be later than start time"):
		return "MONITOR_TIME_RANGE_INVALID", nil
	case lower == "datasource and trace id are required":
		return "MONITOR_TRACE_ID_REQUIRED", nil
	case lower == "invalid trace id format":
		return "MONITOR_TRACE_ID_INVALID", nil
	case lower == "trace was not found":
		return "MONITOR_TRACE_NOT_FOUND", nil
	case lower == "no matching datasource is available" ||
		lower == "monitoring datasource does not exist":
		return "MONITOR_DATASOURCE_UNAVAILABLE", nil
	case strings.Contains(lower, "failed to parse prometheus yaml"):
		return "INVALID_REQUEST", nil
	}

	if system := monitorUpstreamSystem(lower); system != "" && containsAny(lower,
		"api returned status",
		"query failed",
		"health check failed",
		"failed to retrieve",
		"returned no data",
		"failed to parse") {
		return "MONITOR_UPSTREAM_REQUEST_FAILED", map[string]any{"system": system}
	}

	switch {
	case strings.Contains(lower, "kubernetes cluster name already exists"):
		return "K8S_CLUSTER_ALREADY_EXISTS", nil
	case strings.Contains(lower, "k8s cluster not found"):
		return "K8S_CLUSTER_NOT_FOUND", nil
	case containsAny(lower,
		"failed to parse kubeconfig",
		"failed to parse cluster configuration",
		"invalid certificate authority") ||
		lower == "missing context" ||
		lower == "cluster not found" ||
		lower == "server not found":
		return "K8S_INVALID_CLUSTER_CONFIG", nil
	case containsAny(lower,
		"failed to create kubernetes api client",
		"failed to initialize kubernetes client",
		"failed to retrieve kubernetes cluster details",
		"failed to connect to api server",
		"connection succeeded but node listing failed"):
		return "K8S_CLUSTER_CONNECTION_FAILED", nil
	case strings.Contains(lower, "invalid yaml") ||
		strings.Contains(lower, "yaml validation failed"):
		return "K8S_INVALID_YAML", nil
	case lower == "resource not found" ||
		strings.Contains(lower, "resource path not found") ||
		strings.Contains(lower, "target resource does not exist"):
		return "K8S_RESOURCE_NOT_FOUND", nil
	case containsAny(lower,
		"traffic weight must be greater than or equal to 0",
		"traffic weights must total 100"):
		return "K8S_TRAFFIC_WEIGHT_INVALID", nil
	case lower == "please select workloads first":
		return "K8S_WORKLOAD_SELECTION_REQUIRED", nil
	case strings.Contains(lower, "no containers found") ||
		(strings.Contains(lower, "container ") && strings.Contains(lower, " was not found in workload")):
		return "K8S_CONTAINER_NOT_FOUND", nil
	case strings.HasSuffix(lower, " namespace and name are required"):
		return "K8S_RESOURCE_ID_REQUIRED", nil
	case strings.HasSuffix(lower, " namespace is required") ||
		lower == "namespace name is required" ||
		lower == "namespace is required for pvc":
		return "K8S_NAMESPACE_REQUIRED", nil
	case strings.Contains(lower, "immutable fields"):
		return "K8S_RESOURCE_IMMUTABLE", nil
	case strings.Contains(lower, "resource identity conflicts"):
		return "K8S_RESOURCE_CONFLICT", nil
	}

	switch {
	case lower == "unauthorized" && status == http.StatusUnauthorized:
		return "AUTH_SESSION_EXPIRED", nil
	case containsAny(lower,
		"permission denied",
		"forbidden",
		"not authorized",
		"unauthorized",
		"access denied"):
		return "AUTH_PERMISSION_DENIED", nil
	case containsAny(lower,
		"not found",
		"does not exist",
		"doesn't exist",
		"no matching"):
		return "RESOURCE_NOT_FOUND", nil
	case containsAny(lower, "already exists", "duplicate"):
		return "RESOURCE_ALREADY_EXISTS", nil
	case containsAny(lower,
		"required",
		"must be provided",
		"must not be empty",
		"cannot be empty"):
		return "REQUIRED_VALUE_MISSING", nil
	case containsAny(lower,
		"not supported",
		"unsupported",
		"support only",
		"supports only"):
		return "UNSUPPORTED_OPERATION", nil
	case containsAny(lower, "timed out", "timeout"):
		return "OPERATION_TIMEOUT", nil
	case containsAny(lower,
		"failed to connect",
		"unable to connect",
		"connection failed",
		"connection refused"):
		return "CONNECTION_FAILED", nil
	case containsAny(lower,
		"invalid",
		"malformed",
		"must be",
		"must match",
		"must total",
		"must use",
		"changed, please refresh"):
		return "INVALID_REQUEST", nil
	case containsAny(lower,
		"failed",
		"unable to",
		"could not",
		"cannot",
		"can't"):
		return "OPERATION_FAILED", nil
	default:
		return defaultCodeForStatus(status), nil
	}
}

func defaultCodeForStatus(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "AUTH_SESSION_EXPIRED"
	case http.StatusForbidden:
		return "AUTH_PERMISSION_DENIED"
	case http.StatusNotFound:
		return "RESOURCE_NOT_FOUND"
	case http.StatusConflict:
		return "RESOURCE_ALREADY_EXISTS"
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return "OPERATION_TIMEOUT"
	case http.StatusBadGateway, http.StatusServiceUnavailable:
		return "CONNECTION_FAILED"
	default:
		if status >= 400 && status < 500 {
			return "INVALID_REQUEST"
		}
		return "OPERATION_FAILED"
	}
}

func monitorUpstreamSystem(message string) string {
	switch {
	case strings.Contains(message, "victoriametrics"):
		return "VictoriaMetrics"
	case strings.Contains(message, "victorialogs"):
		return "VictoriaLogs"
	case strings.Contains(message, "prometheus"):
		return "Prometheus"
	case strings.Contains(message, "elasticsearch"):
		return "Elasticsearch"
	case strings.Contains(message, "jaeger"):
		return "Jaeger"
	default:
		return ""
	}
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
