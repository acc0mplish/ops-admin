package service

import (
	"bytes"
	"context"
	"fmt"
	"ops-admin/backend/apperr"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

type AssetServiceDiagnosisTarget struct {
	ServiceID    uint   `json:"serviceId" form:"serviceId"`
	WorkloadType string `json:"workloadType" form:"workloadType"`
	WorkloadName string `json:"workloadName" form:"workloadName"`
	PodName      string `json:"podName" form:"podName"`
	Container    string `json:"container" form:"container"`
	PID          string `json:"pid" form:"pid"`
	Operation    string `json:"operation" form:"operation"`
	Pattern      string `json:"pattern" form:"pattern"`
	Event        string `json:"event" form:"event"`
	Seconds      int    `json:"seconds" form:"seconds"`
}

func (s *Service) validateAssetServiceDiagnosisTarget(target AssetServiceDiagnosisTarget) (uint, string, error) {
	service, err := s.GetAssetService(target.ServiceID)
	if err != nil {
		return 0, "", err
	}
	if !assetServiceContainsWorkload(service, target.WorkloadType, target.WorkloadName) {
		return 0, "", apperr.New("ASSET_SERVICE_WORKLOAD_MISMATCH", nil)
	}
	if Trimmed(target.PodName) == "" || Trimmed(target.Container) == "" {
		return 0, "", apperr.New("DIAGNOSIS_POD_CONTAINER_REQUIRED", nil)
	}
	if pid := Trimmed(target.PID); pid != "" {
		for _, char := range pid {
			if char < '0' || char > '9' {
				return 0, "", apperr.New("INVALID_PROCESS_ID", nil)
			}
		}
	}
	return service.K8sClusterID, service.Namespace, nil
}

func (s *Service) execAssetServiceDiagnosis(clusterID uint, namespace string, target AssetServiceDiagnosisTarget, command []string) (string, error) {
	cluster, err := s.GetK8sCluster(clusterID)
	if err != nil {
		return "", err
	}
	config, cleanup, err := s.k8sRESTConfigForCluster(cluster)
	if err != nil {
		return "", apperr.New("K8S_CLUSTER_CONNECTION_FAILED", nil)
	}
	defer cleanup()
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", apperr.New("K8S_CLUSTER_CONNECTION_FAILED", nil)
	}
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), target.PodName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	container := chooseK8sContainerName(pod, target.Container)
	if container == "" {
		return "", apperr.New("DIAGNOSIS_CONTAINER_NOT_FOUND", nil)
	}
	req := clientset.CoreV1().RESTClient().Post().Resource("pods").Name(target.PodName).Namespace(namespace).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{Container: container, Command: command, Stdout: true, Stderr: true}, scheme.ParameterCodec)
	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", err
	}
	var stdout, stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		return "", apperr.Wrap("DIAGNOSIS_EXECUTION_FAILED", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String())), nil)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (s *Service) GetAssetServiceDiagnosisProcesses(target AssetServiceDiagnosisTarget) (map[string]any, error) {
	clusterID, namespace, err := s.validateAssetServiceDiagnosisTarget(target)
	if err != nil {
		return nil, err
	}
	output, err := s.execAssetServiceDiagnosis(clusterID, namespace, target, []string{"sh", "-c", "ps -eo pid=,comm=,args= 2>/dev/null | grep -E '[j]ava' || true"})
	if err != nil {
		return nil, err
	}
	items := make([]map[string]string, 0)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		item := map[string]string{"pid": fields[0], "name": fields[1], "command": strings.Join(fields[1:], " ")}
		items = append(items, item)
	}
	return map[string]any{"processes": items}, nil
}

func (s *Service) GetAssetServiceDiagnosisEnvironment(target AssetServiceDiagnosisTarget) (map[string]any, error) {
	clusterID, namespace, err := s.validateAssetServiceDiagnosisTarget(target)
	if err != nil {
		return nil, err
	}
	pidCheck := "true"
	if Trimmed(target.PID) != "" {
		pidCheck = "test -r /proc/" + strings.TrimSpace(target.PID) + "/status"
	}
	output, err := s.execAssetServiceDiagnosis(clusterID, namespace, target, []string{"sh", "-c", "java -version 2>&1; test -r /opt/arthas-boot.jar; " + pidCheck})
	if err != nil {
		return map[string]any{"ready": false, "message": "Java, the Arthas JAR, or the target process is unavailable: " + err.Error()}, nil
	}
	return map[string]any{"ready": true, "message": "Java, the Arthas JAR, and the target process are ready for Arthas CLI diagnostics.", "detail": output}, nil
}

func (s *Service) DownloadAssetServiceArthas(target AssetServiceDiagnosisTarget) (map[string]any, error) {
	clusterID, namespace, err := s.validateAssetServiceDiagnosisTarget(target)
	if err != nil {
		return nil, err
	}
	_, err = s.execAssetServiceDiagnosis(clusterID, namespace, target, []string{"sh", "-c", "set -e; if command -v curl >/dev/null 2>&1; then curl -fsSL https://arthas.aliyun.com/arthas-boot.jar -o /opt/arthas-boot.jar; elif command -v wget >/dev/null 2>&1; then wget -qO /opt/arthas-boot.jar https://arthas.aliyun.com/arthas-boot.jar; else exit 12; fi"})
	if err != nil {
		return nil, apperr.Wrap("ARTHAS_DOWNLOAD_FAILED", err, nil)
	}
	return s.GetAssetServiceDiagnosisEnvironment(target)
}

func (s *Service) UploadAssetServiceArthas(target AssetServiceDiagnosisTarget, content []byte) (map[string]any, error) {
	if len(content) == 0 {
		return nil, apperr.New("ARTHAS_FILE_EMPTY", nil)
	}
	if len(content) > 80*1024*1024 {
		return nil, apperr.New("ARTHAS_FILE_TOO_LARGE", map[string]any{"maxMb": 80})
	}
	clusterID, namespace, err := s.validateAssetServiceDiagnosisTarget(target)
	if err != nil {
		return nil, err
	}
	cluster, err := s.GetK8sCluster(clusterID)
	if err != nil {
		return nil, err
	}
	config, cleanup, err := s.k8sRESTConfigForCluster(cluster)
	if err != nil {
		return nil, apperr.New("K8S_CLUSTER_CONNECTION_FAILED", nil)
	}
	defer cleanup()
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, apperr.New("K8S_CLUSTER_CONNECTION_FAILED", nil)
	}
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), target.PodName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	container := chooseK8sContainerName(pod, target.Container)
	if container == "" {
		return nil, apperr.New("DIAGNOSIS_CONTAINER_NOT_FOUND", nil)
	}
	req := clientset.CoreV1().RESTClient().Post().Resource("pods").Name(target.PodName).Namespace(namespace).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{Container: container, Command: []string{"sh", "-c", "cat > /opt/arthas-boot.jar && test -s /opt/arthas-boot.jar"}, Stdin: true, Stdout: true, Stderr: true}, scheme.ParameterCodec)
	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{Stdin: bytes.NewReader(content), Stdout: &stdout, Stderr: &stderr}); err != nil {
		return nil, apperr.Wrap("ARTHAS_UPLOAD_FAILED", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String())), nil)
	}
	return s.GetAssetServiceDiagnosisEnvironment(target)
}

var arthasANSIControl = regexp.MustCompile("(?:\\x1b|�)\\[[0-?]*[ -/]*[@-~]")
var arthasThreadHeader = regexp.MustCompile(`(?m)^"([^"]+)"\s+Id=(\d+)\s+cpuUsage=([^\s]+)\s+deltaTime=[^\s]+\s+time=([^\s]+)\s+([A-Z_]+)`)
var arthasFlameData = regexp.MustCompile(`(?m)^[ \t]*(?:f|u|n)\(\d+\s*,`)

func cleanArthasOutput(output string) string {
	return strings.TrimSpace(arthasANSIControl.ReplaceAllString(output, ""))
}

func arthasOutputRows(output string) []map[string]string {
	rows := make([]map[string]string, 0)
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "[") || strings.HasPrefix(line, ",---") || strings.HasPrefix(line, "/ ") || strings.HasPrefix(line, "|") || strings.HasPrefix(line, "`") || strings.HasPrefix(line, "Process ends after") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		rows = append(rows, map[string]string{"name": fields[0], "value": strings.Join(fields[1:], " ")})
	}
	return rows
}

func arthasRowValue(rows []map[string]string, name string) string {
	for _, row := range rows {
		if row["name"] == name {
			return row["value"]
		}
	}
	return "-"
}

func arthasJVMSummary(output string) map[string]string {
	rows := arthasOutputRows(output)
	return map[string]string{
		"vmName":        arthasRowValue(rows, "VM-NAME"),
		"vmVersion":     arthasRowValue(rows, "VM-VERSION"),
		"startTime":     arthasRowValue(rows, "JVM-START-TIME"),
		"loadedClasses": arthasRowValue(rows, "LOADED-CLASS-COUNT"),
		"threads":       arthasRowValue(rows, "COUNT"),
		"processors":    arthasRowValue(rows, "PROCESSORS-COUNT"),
	}
}

func removeArthasClassPath(output string) string {
	lines := strings.Split(output, "\n")
	filtered := make([]string, 0, len(lines))
	skippingContinuation := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "CLASS-PATH") {
			skippingContinuation = true
			continue
		}
		if skippingContinuation && (trimmed == "" || len(strings.Fields(line)) < 2) {
			continue
		}
		skippingContinuation = false
		filtered = append(filtered, line)
	}
	return strings.TrimSpace(strings.Join(filtered, "\n"))
}

func arthasThreadRows(output string) []map[string]string {
	matches := arthasThreadHeader.FindAllStringSubmatch(output, -1)
	threads := make([]map[string]string, 0, len(matches))
	for _, match := range matches {
		threads = append(threads, map[string]string{
			"id":    match[2],
			"name":  match[1],
			"state": match[5],
			"cpu":   match[3],
			"time":  match[4],
		})
	}
	return threads
}

func arthasMemoryRows(output string) []map[string]string {
	rows := make([]map[string]string, 0)
	inMemoryTable := false
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if strings.Contains(line, "used") && strings.Contains(line, "total") && strings.Contains(line, "usage") {
			inMemoryTable = true
			continue
		}
		if !inMemoryTable || line == "" || strings.HasPrefix(line, "[") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 || !strings.HasSuffix(fields[len(fields)-1], "%") {
			continue
		}
		last := len(fields)
		rows = append(rows, map[string]string{
			"name":  strings.Join(fields[:last-4], " "),
			"used":  fields[last-4],
			"total": fields[last-3],
			"max":   fields[last-2],
			"usage": fields[last-1],
		})
	}
	return rows
}

func arthasMemoryRow(rows []map[string]string, name string) map[string]string {
	for _, row := range rows {
		if row["name"] == name {
			return row
		}
	}
	return map[string]string{"used": "-", "total": "-", "max": "-", "usage": "-"}
}

func arthasEnvironmentRows(output string) []map[string]string {
	rows := make([]map[string]string, 0)
	inEnvironmentTable := false
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "KEY") && strings.Contains(line, "VALUE") {
			inEnvironmentTable = true
			continue
		}
		if !inEnvironmentTable || line == "" || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "[") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		rows = append(rows, map[string]string{"key": fields[0], "value": strings.Join(fields[1:], " ")})
	}
	return rows
}

func arthasPropertyRows(output string) []map[string]string {
	rows := arthasEnvironmentRows(output)
	filtered := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		if row["key"] != "java.class.path" {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func arthasPropertySummary(rows []map[string]string) map[string]string {
	values := map[string]string{}
	for _, row := range rows {
		values[row["key"]] = row["value"]
	}
	get := func(key string) string {
		if values[key] == "" {
			return "-"
		}
		return values[key]
	}
	return map[string]string{
		"application": get("APPLICATION_NAME"),
		"command":     get("sun.java.command"),
		"pid":         get("PID"),
		"workDir":     get("user.dir"),
		"javaHome":    get("java.home"),
		"tempDir":     get("java.io.tmpdir"),
		"timezone":    get("user.timezone"),
		"encoding":    get("file.encoding"),
	}
}

func removeArthasProperty(output, property string) string {
	lines := strings.Split(output, "\n")
	filtered := make([]string, 0, len(lines))
	skippingContinuation := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, property) {
			skippingContinuation = true
			continue
		}
		if skippingContinuation && (trimmed == "" || len(strings.Fields(line)) < 2) {
			continue
		}
		skippingContinuation = false
		filtered = append(filtered, line)
	}
	return strings.TrimSpace(strings.Join(filtered, "\n"))
}

func (s *Service) runArthasCLI(clusterID uint, namespace string, target AssetServiceDiagnosisTarget, command string) (string, error) {
	shell := "java -jar /opt/arthas-boot.jar -c " + shellQuote(command) + " " + shellQuote(Trimmed(target.PID))
	return s.execAssetServiceDiagnosis(clusterID, namespace, target, []string{"sh", "-c", shell})
}

func (s *Service) runArthasCLIWide(clusterID uint, namespace string, target AssetServiceDiagnosisTarget, command string) (string, error) {
	// sysenv values and Kubernetes-generated variable names are frequently
	// wider than Arthas's default terminal size. Keep the command output intact.
	shell := "java -jar /opt/arthas-boot.jar --width 240 -c " + shellQuote(command) + " " + shellQuote(Trimmed(target.PID))
	return s.execAssetServiceDiagnosis(clusterID, namespace, target, []string{"sh", "-c", shell})
}

func isDashboardThreadRow(line string) bool {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return false
	}
	for _, char := range fields[0] {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// parseDashboardSummary reads only values actually present in `dashboard -n 1`.
func parseDashboardSummary(output string) map[string]any {
	summary := map[string]any{"hotThreads": "-", "heapUsed": "-", "heapTotal": "-", "nonHeapUsed": "-", "nonHeapTotal": "-"}
	threadRows := 0
	section := ""
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "ID "):
			section = "threads"
			continue
		case strings.HasPrefix(line, "Memory"):
			section = "memory"
			continue
		case strings.HasPrefix(line, "Runtime"):
			section = "runtime"
			continue
		}
		if section == "threads" && isDashboardThreadRow(line) {
			threadRows++
		}
		if section != "memory" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		switch fields[0] {
		case "heap":
			summary["heapUsed"], summary["heapTotal"] = fields[1], fields[2]
		case "nonheap":
			summary["nonHeapUsed"], summary["nonHeapTotal"] = fields[1], fields[2]
		}
	}
	if threadRows > 0 {
		summary["hotThreads"] = fmt.Sprintf("%d", threadRows)
	}
	return summary
}

func (s *Service) getAssetServiceJVMDashboard(clusterID uint, namespace string, target AssetServiceDiagnosisTarget) (map[string]any, error) {
	output, err := s.runArthasCLI(clusterID, namespace, target, "dashboard -n 1")
	if err != nil {
		return nil, err
	}
	output = cleanArthasOutput(output)
	return map[string]any{"dashboard": parseDashboardSummary(output), "raw": output}, nil
}

// getAssetServiceFlamegraph generates a uniquely named temporary report in the
// container, returns its HTML for the browser preview, and then removes it.
func (s *Service) getAssetServiceFlamegraph(clusterID uint, namespace string, target AssetServiceDiagnosisTarget) (map[string]any, error) {
	seconds := target.Seconds
	if seconds != 10 && seconds != 30 && seconds != 60 && seconds != 120 {
		return nil, apperr.New("INVALID_FLAMEGRAPH_DURATION", nil)
	}
	event := Trimmed(target.Event)
	if event == "" {
		event = "cpu"
	}
	if event != "cpu" && event != "alloc" {
		return nil, apperr.New("INVALID_FLAMEGRAPH_EVENT", nil)
	}
	profilerEvent := event
	file := fmt.Sprintf("/tmp/arthas-flame-%s-%s-%d-%d.html", event, Trimmed(target.PID), seconds, time.Now().UnixNano())
	if _, err := s.runArthasCLI(clusterID, namespace, target, "profiler start --event "+profilerEvent); err != nil {
		return nil, apperr.Wrap("FLAMEGRAPH_START_FAILED", err, nil)
	}
	defer func() {
		_, _ = s.execAssetServiceDiagnosis(clusterID, namespace, target, []string{"sh", "-c", "rm -f " + shellQuote(file)})
	}()
	time.Sleep(time.Duration(seconds) * time.Second)
	stopOutput, stopErr := s.runArthasCLI(clusterID, namespace, target, "profiler stop --format html --file "+file)
	if stopErr != nil {
		return nil, apperr.Wrap("FLAMEGRAPH_STOP_FAILED", stopErr, nil)
	}
	html, err := s.execAssetServiceDiagnosis(clusterID, namespace, target, []string{"sh", "-c", "cat " + shellQuote(file)})
	if err != nil {
		return nil, apperr.Wrap("FLAMEGRAPH_READ_FAILED", err, nil)
	}
	if !strings.Contains(strings.ToLower(html), "<html") {
		return nil, apperr.New("FLAMEGRAPH_INVALID_OUTPUT", nil)
	}
	if !arthasFlameData.MatchString(html) {
		if event == "cpu" {
			return nil, apperr.New("FLAMEGRAPH_CPU_NO_SAMPLES", nil)
		}
		return nil, apperr.New("FLAMEGRAPH_ALLOC_NO_SAMPLES", nil)
	}
	return map[string]any{"flameHtml": html, "event": event, "engine": profilerEvent, "raw": cleanArthasOutput(stopOutput)}, nil
}

// RunAssetServiceDiagnosis only accepts a small read-only Arthas command set.
func (s *Service) RunAssetServiceDiagnosis(target AssetServiceDiagnosisTarget) (map[string]any, error) {
	clusterID, namespace, err := s.validateAssetServiceDiagnosisTarget(target)
	if err != nil {
		return nil, err
	}
	if Trimmed(target.PID) == "" {
		return nil, apperr.New("PROCESS_ID_REQUIRED", nil)
	}
	if target.Operation == "dashboard" {
		data, err := s.getAssetServiceJVMDashboard(clusterID, namespace, target)
		if err != nil {
			return nil, apperr.Wrap("ARTHAS_DASHBOARD_FAILED", err, nil)
		}
		return map[string]any{"operation": target.Operation, "data": data}, nil
	}
	if target.Operation == "flame" {
		data, err := s.getAssetServiceFlamegraph(clusterID, namespace, target)
		if err != nil {
			return nil, apperr.Wrap("ARTHAS_FLAMEGRAPH_FAILED", err, nil)
		}
		return map[string]any{"operation": target.Operation, "data": data}, nil
	}
	var command string
	switch target.Operation {
	case "thread":
		command = "thread -n 10"
	case "jvm":
		command = "jvm"
	case "memory":
		command = "memory"
	case "env":
		command = "sysenv"
	case "sysprop":
		command = "sysprop"
	case "class":
		pattern := Trimmed(target.Pattern)
		if pattern == "" {
			return nil, apperr.New("CLASS_PATTERN_REQUIRED", nil)
		}
		for _, char := range pattern {
			if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || strings.ContainsRune(".*$_-", char)) {
				return nil, apperr.New("INVALID_CLASS_PATTERN", nil)
			}
		}
		command = "sc -d " + pattern
	default:
		return nil, apperr.New("UNSUPPORTED_DIAGNOSIS_OPERATION", nil)
	}
	var output string
	if target.Operation == "env" || target.Operation == "sysprop" {
		output, err = s.runArthasCLIWide(clusterID, namespace, target, command)
	} else {
		output, err = s.runArthasCLI(clusterID, namespace, target, command)
	}
	if err != nil {
		return nil, apperr.Wrap("ARTHAS_CLI_FAILED", err, nil)
	}
	output = cleanArthasOutput(output)
	rawOutput := output
	if target.Operation == "jvm" {
		rawOutput = removeArthasClassPath(output)
	}
	if target.Operation == "sysprop" {
		rawOutput = removeArthasProperty(output, "java.class.path")
	}
	data := map[string]any{"rows": arthasOutputRows(output), "raw": rawOutput}
	if target.Operation == "thread" {
		data["threads"] = arthasThreadRows(output)
	}
	if target.Operation == "memory" {
		memoryRows := arthasMemoryRows(output)
		data["memory"] = map[string]any{"rows": memoryRows, "heap": arthasMemoryRow(memoryRows, "heap"), "nonHeap": arthasMemoryRow(memoryRows, "nonheap")}
	}
	if target.Operation == "env" {
		data["envs"] = arthasEnvironmentRows(output)
	}
	if target.Operation == "sysprop" {
		properties := arthasPropertyRows(output)
		data["properties"] = properties
		data["propertySummary"] = arthasPropertySummary(properties)
	}
	if target.Operation == "jvm" {
		data["jvm"] = arthasJVMSummary(output)
	}
	return map[string]any{"operation": target.Operation, "data": data}, nil
}
