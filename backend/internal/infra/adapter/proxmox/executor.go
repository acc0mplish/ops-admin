// executor.go — Phase D(N3b): PVE guarded operations의 OperationExecutor·
// TaskPoller(§3.2 — k8s adapter.go+executor.go 분리 선례 승계). 계약(판정 J4·J5·
// J7 — 계획 §3.2):
//
//   - Execute: Material["operations"](RW 토큰 — J5 자격 분리)만 수용하고, opdef
//     3종(pve.guest.power|snapshot|config)의 payload 화이트리스트(E-5)로 발행을
//     통제한 뒤 mutation 1회를 보낸다. 응답은 dual-mode 3분기로 착지한다(J4):
//     data null → 빈 핸들(엔진이 단일 attempt 성공으로 종단 — §0.3 실측),
//     data "UPID:…" → upid 핸들(비동기 — 엔진 pollAsyncAttempts 승계), 동기 에러
//     → error(엔진 executor_error/operation_failed). 엔진·tasks 패키지 수정 0이
//     이 계약의 전제다.
//   - Poll: 핸들 디코드 → GET /nodes/{node}/tasks/{upid}/status 1회 → running |
//     stopped OK → Succeeded | stopped 기타 → Failed. 폴 자격은 엔진이 매 폴
//     조립해 주입한다(J12) — 어댑터는 자격을 스스로 획득하지 않는다.
//
// 이 어댑터의 실행 경로는 stateless다("HTTP clients are per-request") — 핸들에
// 필요한 상태 전부를 ProviderRef가 자기서술한다(upid.go — Phase B 인코딩 재사용).

package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"ops-admin/backend/internal/infra/contract"
)

// 본 executor가 수용하는 오퍼레이션 — 3종뿐이다(J7 "유일 수용"). compose가
// opdef 이름 원천으로 재사용한다(restartOperation 선례).
const (
	PowerOperationName    = "pve.guest.power"
	SnapshotOperationName = "pve.guest.snapshot"
	ConfigOperationName   = "pve.guest.config"
)

// powerActions — pve.guest.power의 액션 어휘(J7: start|shutdown|stop|reboot).
// 4 액션을 1 opdef로 묶은 근거는 k8s restart 1종 선례(동일 승인자 인구)이고,
// 액션별 통제는 이 검증이 담당한다.
var powerActions = map[string]bool{"start": true, "shutdown": true, "stop": true, "reboot": true}

// Compile-time interface conformance — registry V5가 mutation capability 선언에
// 요구하는 OperationExecutor(§3.7 매핑 표)와 §14.3 듀얼 모드의 TaskPoller.
var (
	_ contract.OperationExecutor = (*Adapter)(nil)
	_ contract.TaskPoller        = (*Adapter)(nil)
)

// --- URN 파싱 — urn:proxmox:{ctx}:(vm|system_container):{node}/{vmid}. ---

// guestTarget — URN이 가리키는 단일 게스트. GuestType은 PVE API 경로 성분 어휘
// (qemu|lxc)다(§3.2 "guestType qemu|lxc").
type guestTarget struct {
	GuestType string
	Node      string
	VMID      string
}

// 경로 성분 방어(r1.4 HIGH-3): vmid는 숫자 전체, node는 [a-zA-Z0-9_-]+ — 검증
// 통과 전엔 경로가 조립되지 않는다. executor 버그·변조 URN의 경로 오염(경로
// 순회·인접 리소스 침범·쿼리 주입)을 조립 직전에 차단한다.
var (
	guestNodePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	guestVMIDPattern = regexp.MustCompile(`^[0-9]+$`)
)

func validateNodeSegment(node string) error {
	if !guestNodePattern.MatchString(node) {
		return fmt.Errorf("proxmox: node segment %q fails the path charset guard [a-zA-Z0-9_-]+ (HIGH-3 — path assembly refused)", node)
	}
	return nil
}

// parseGuestURN — 게스트 URN을 실행 타깃으로 분해한다. ctxID는 정의상 프로바이더
// 컨텍스트 식별자(숫자)지만 어댑터는 검증만 하고 사용하지 않는다 — 노드 주소는
// Connection이 결정한다(arch rule 2).
func parseGuestURN(urn string) (guestTarget, error) {
	rest, ok := strings.CutPrefix(urn, "urn:proxmox:")
	if !ok {
		return guestTarget{}, fmt.Errorf("proxmox: resource URN %q is not a proxmox URN (want urn:proxmox:<ctx>:<vm|system_container>:<node>/<vmid>)", urn)
	}
	parts := strings.Split(rest, ":")
	if len(parts) != 3 {
		return guestTarget{}, fmt.Errorf("proxmox: resource URN %q is not a guest URN — this executor serves vm and system_container resources only", urn)
	}
	if _, err := strconv.ParseUint(parts[0], 10, 64); err != nil {
		return guestTarget{}, fmt.Errorf("proxmox: resource URN %q carries a non-numeric context id %q", urn, parts[0])
	}
	var guestType string
	switch parts[1] {
	case "vm":
		guestType = "qemu"
	case "system_container":
		guestType = "lxc"
	default:
		return guestTarget{}, fmt.Errorf("proxmox: resource kind %q is not served by this executor (serves vm, system_container)", parts[1])
	}
	node, vmid, ok := strings.Cut(parts[2], "/")
	if !ok {
		return guestTarget{}, fmt.Errorf("proxmox: guest URN tail %q must be <node>/<vmid>", parts[2])
	}
	if err := validateNodeSegment(node); err != nil {
		return guestTarget{}, err
	}
	if !guestVMIDPattern.MatchString(vmid) {
		return guestTarget{}, fmt.Errorf("proxmox: vmid %q fails the numeric guard [0-9]+ (HIGH-3 — path assembly refused)", vmid)
	}
	return guestTarget{GuestType: guestType, Node: node, VMID: vmid}, nil
}

// guestBasePath — 게스트 스코프 REST 경로의 공통 접두.
func (t guestTarget) guestBasePath() string {
	return "/nodes/" + t.Node + "/" + t.GuestType + "/" + t.VMID
}

// --- 자격 — 실행 경로 Material 키는 "operations" 하나뿐이다(J5). ---

// resolveOperationsMaterial — 실행 자격을 operations 슬롯에서만 해석한다(§14.3
// "read-only token for discovery, operations token gated by approval"). inventory
// 바인딩 재질이 슬롯에 있으면 자격 분리 위반으로 즉시 거부한다 — RO 토큰이
// mutation에 흐르는 경로는 존재하지 않는다. 에러 문구에는 재질 평문이 실리지
// 않는다(보존 제약 7).
func resolveOperationsMaterial(conn contract.ConnectionView) (string, error) {
	if conn.Material != nil {
		if material := strings.TrimSpace(conn.Material[contract.CredentialPurposeOperations]); material != "" {
			return material, nil
		}
		if strings.TrimSpace(conn.Material[contract.CredentialPurposeInventory]) != "" {
			return "", fmt.Errorf("proxmox: inventory-bound credentials are not accepted on the operations path — execution resolves Material[%q] only (credential separation, J5)", contract.CredentialPurposeOperations)
		}
	}
	return "", fmt.Errorf(
		"proxmox: missing credential material Material[%q] (broker purpose %q resolve — engine classifies this path as credential_error)",
		contract.CredentialPurposeOperations, contract.CredentialPurposeOperations)
}

// buildExecutorClient — 실행 경로 전송을 요청 스코프로 재조립한다(adapter.go
// buildClient의 inventory 경로와 대칭 — posture 번역은 동일, 재질 슬롯만 다르다).
func (a *Adapter) buildExecutorClient(conn contract.ConnectionView) (*Client, error) {
	material, err := resolveOperationsMaterial(conn)
	if err != nil {
		return nil, err
	}
	cred, err := parseTokenCredential(material)
	if err != nil {
		return nil, err
	}
	opts := []clientOption{withMetrics(a.metrics)}
	if mode, _ := conn.Config["deployment_mode"].(string); mode == "reverse_proxy" {
		opts = append(opts, withReverseProxy())
	}
	if insecure, _ := conn.Config["tls_insecure"].(bool); insecure {
		opts = append(opts, withInsecureTLS())
	}
	return newClient(cred, conn.Endpoint, opts...), nil
}

// --- payload 검증 — J7 화이트리스트(E-5). ---

// payloadKeysExactly — 허용 키 집합 밖의 키는 전부 거부한다: provider로 흘러가는
// 파라미터 면을 opdef 화이트리스트로 닫는다(E-5 — "화이트리스트 외 키·값 거부").
// 엔진이 Payload에 동봉하는 부기키는 검증 대상이 아니다 — restartedAt(J1 동결,
// k8s executor가 소비하는 선례)와 resourceRevision(J7 감사 선택키)은 오퍼레이션
// 파라미터가 아니라 실행 기록 성분이다(§3.2. 실엔드포인트 증명 §13-9 발견).
func payloadKeysExactly(payload contract.JSONMap, allowed ...string) error {
	for key := range payload {
		if key == "restartedAt" || key == "resourceRevision" {
			continue
		}
		known := false
		for _, a := range allowed {
			if key == a {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errorf("proxmox: payload key %q is not in the operation whitelist (E-5 — allowed keys: %s)", key, strings.Join(allowed, ", "))
		}
	}
	return nil
}

// requireString — 필수 비공백 문자열 성분.
func requireString(payload contract.JSONMap, key string) (string, error) {
	raw, ok := payload[key]
	if !ok {
		return "", fmt.Errorf("proxmox: payload carries no %q", key)
	}
	s, ok := raw.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("proxmox: payload %q must be a non-empty string, got %T", key, raw)
	}
	return s, nil
}

// positiveInteger — 선택 양의 정수 성분. 엔진의 JSON 디코드 경로는 float64로
// 온다 — 소수·음수·0은 전부 거부한다(E-5 "값 거부").
func positiveInteger(payload contract.JSONMap, key string) (value int64, present bool, err error) {
	raw, ok := payload[key]
	if !ok {
		return 0, false, nil
	}
	switch v := raw.(type) {
	case int:
		if v <= 0 {
			return 0, true, fmt.Errorf("proxmox: payload %q must be a positive integer, got %d", key, v)
		}
		return int64(v), true, nil
	case float64:
		if v <= 0 || v != math.Trunc(v) {
			return 0, true, fmt.Errorf("proxmox: payload %q must be a positive integer, got %v", key, v)
		}
		return int64(v), true, nil
	default:
		return 0, true, fmt.Errorf("proxmox: payload %q must be a positive integer, got %T", key, raw)
	}
}

// buildMutation — 오퍼레이션별 payload 검증과 PVE 경로·폼 조립(J7). 검증 실패는
// 와이어 0회의 즉시 에러다(§3.2 — 재시도 없음). 알 수 없는 오퍼레이션도 여기서
// 거부된다.
func buildMutation(op string, target guestTarget, payload contract.JSONMap) (path string, form url.Values, err error) {
	switch op {
	case PowerOperationName:
		if err := payloadKeysExactly(payload, "action"); err != nil {
			return "", nil, err
		}
		action, err := requireString(payload, "action")
		if err != nil {
			return "", nil, err
		}
		if !powerActions[action] {
			return "", nil, fmt.Errorf("proxmox: power action %q is outside the guarded vocabulary (start|shutdown|stop|reboot — J7)", action)
		}
		return target.guestBasePath() + "/status/" + action, url.Values{}, nil

	case SnapshotOperationName:
		if err := payloadKeysExactly(payload, "snapname"); err != nil {
			return "", nil, err
		}
		snapname, err := requireString(payload, "snapname")
		if err != nil {
			return "", nil, err
		}
		return target.guestBasePath() + "/snapshot", url.Values{"snapname": {snapname}}, nil

	case ConfigOperationName:
		if err := payloadKeysExactly(payload, "cores", "memoryMB"); err != nil {
			return "", nil, err
		}
		cores, coresSet, err := positiveInteger(payload, "cores")
		if err != nil {
			return "", nil, err
		}
		memoryMB, memorySet, err := positiveInteger(payload, "memoryMB")
		if err != nil {
			return "", nil, err
		}
		if !coresSet && !memorySet {
			return "", nil, fmt.Errorf("proxmox: config payload carries none of cores, memoryMB — a no-change PUT is rejected (E-5)")
		}
		form = url.Values{}
		if coresSet {
			form.Set("cores", strconv.FormatInt(cores, 10))
		}
		if memorySet {
			// A6/E-5 확정: PVE PUT config의 파라미터 명칭은 memory, 단위는 MiB
			// 정수다 — payload 어휘(memoryMB)를 폼 어휘로 번역하는 유일 지점.
			form.Set("memory", strconv.FormatInt(memoryMB, 10))
		}
		return target.guestBasePath() + "/config", form, nil
	}
	return "", nil, fmt.Errorf("proxmox: operation %q is not served by this executor (serves %q, %q, %q only)",
		op, PowerOperationName, SnapshotOperationName, ConfigOperationName)
}

// --- mutation 전송면 — dual-mode 3분기(J4). ---

// putForm — mutation(PUT) 1회(pve.guest.config 경로). postForm(client.go —
// Phase B 소유)의 쓰기 계약을 그대로 승계한다: background_delay 명시 첨부(판정
// J4 — 증류 계약 1), 쓰기 실패의 페일오버 장부 비개입(증거 비대칭 — J6(2)).
// client.go 무수정(배치표 경계)을 지키기 위해 같은 수신자의 메서드를 이 파일에
// 둔다 — 패키지 내 배치는 자유다.
func (c *Client) putForm(ctx context.Context, path string, form url.Values, op string) (json.RawMessage, error) {
	if form == nil {
		form = url.Values{}
	}
	if form.Get("background_delay") == "" {
		form.Set("background_delay", strconv.Itoa(defaultBackgroundDelaySeconds))
	}
	return c.do(ctx, http.MethodPut, path, op, form)
}

// isNullData — J4 1분기 판별: PVE가 background_delay 창 내 완료를 data null로
// 보고한다(동기 완료 창 — 증류 계약 1).
func isNullData(data json.RawMessage) bool {
	return len(data) == 0 || string(data) == "null"
}

// upidFromData — J4 2분기 해석: data는 UPID 문자열이어야 한다. 그 외 형상(객체·
// 숫자 등)은 PVE mutation 응답이 아니므로 거부한다.
func upidFromData(data json.RawMessage) (UPID, error) {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return UPID{}, fmt.Errorf("proxmox: mutation response data is neither null nor a UPID string (J4 dual mode)")
	}
	return parseUPID(s)
}

// Execute — 검증(왕복 0회) → mutation 1회 → dual-mode 3분기. 빈 ProviderRef 핸들은
// 에러가 아니다: 엔진 executeClaimed가 단일 attempt 성공으로 종단하는 동기 완료
// 계약이다(§0.3 실측 — 엔진 무수정).
func (a *Adapter) Execute(ctx context.Context, req contract.OperationRequest) (contract.OperationHandle, error) {
	target, err := parseGuestURN(req.ResourceURN)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	path, form, err := buildMutation(req.OperationName, target, req.Payload)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	if req.Connection.UID == "" {
		return contract.OperationHandle{}, fmt.Errorf("proxmox: execution connection carries no UID — the upid handle must encode the connection it targets (J12)")
	}
	client, err := a.buildExecutorClient(req.Connection)
	if err != nil {
		return contract.OperationHandle{}, err
	}

	mutate := client.postForm
	if req.OperationName == ConfigOperationName {
		mutate = client.putForm
	}
	data, err := mutate(ctx, path, form, req.OperationName)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	if isNullData(data) {
		return contract.OperationHandle{ProviderRef: ""}, nil
	}
	u, err := upidFromData(data)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	return contract.OperationHandle{ProviderRef: encodeUPIDRef(req.Connection.UID, u)}, nil
}

// --- Poll — UPID 재폴(§14.3 증류 계약 1의 문언 그대로). ---

// pollTaskPath — 재폴 경로 조립(HIGH-3 계열 — 조립 직전 성분 방어). node는 URN과
// 같은 문자셋 검증을, raw UPID에는 URL 활성 문자 부재를 요구한다 — parseUPID의
// 정규형 검증만으로는 성분 속 "?#%/ "가 경로를 오염시킬 수 있다(핸들은
// task_attempt.handle_ref에 지속되는 값 — 위조·손상 입력을 상정한다).
func pollTaskPath(handle upidHandle) (string, error) {
	if err := validateNodeSegment(handle.UPID.Node); err != nil {
		return "", err
	}
	if strings.ContainsAny(handle.UPID.Raw, "?#%/ ") {
		return "", fmt.Errorf("proxmox: upid %q carries URL-active characters — poll path assembly refused", handle.UPID.Raw)
	}
	return "/nodes/" + handle.UPID.Node + "/tasks/" + handle.UPID.Raw + "/status", nil
}

// upidDetail — 폴 성공·실패 detail. §3.2 redaction 허용 필드의 부분집합만 싣는다:
// 핸들은 stateless 계약이라 vmid·action·snapname·cores·memoryMB를 폴 시점에
// 재현할 재료가 없고, 재료가 있는 성분(node·guestType)과 원문(upid)·관측값
// (exitStatus)만 허용 집합으로 흘린다.
func upidDetail(handle upidHandle, exitStatus string) contract.JSONMap {
	detail := contract.JSONMap{
		"node":       handle.UPID.Node,
		"upid":       handle.UPID.Raw,
		"exitStatus": exitStatus,
	}
	if handle.UPID.Type == "qemu" || handle.UPID.Type == "lxc" {
		detail["guestType"] = handle.UPID.Type
	}
	return detail
}

// Poll — GET 1회 → 종별 판정식(J4): running → 다음 폴 사이클, stopped OK →
// Succeeded{detail}, stopped 기타(에러문) → Failed. PVE task status 어휘 밖의
// 값은 묵시적 Running 전이 대신 에러로 처리한다(수렴 오탐 차단).
func (a *Adapter) Poll(ctx context.Context, req contract.PollRequest) (contract.OperationStatus, error) {
	handle, err := decodeUPIDRef(req.Handle.ProviderRef)
	if err != nil {
		return contract.OperationStatus{}, err
	}
	// J12 정합 가드(k8s Poll 승계): 엔진이 매 폴 조립하는 Connection이 핸들이
	// 자기서술하는 커넥션과 다르면 조립 버그다 — 다른 커넥션의 태스크를 폴하는
	// 오발사를 늦은 수렴 오탐보다 빨리 잡는다.
	if req.Connection.UID != handle.ConnectionUID {
		return contract.OperationStatus{}, fmt.Errorf(
			"proxmox: poll connection UID %q does not match the upid handle's connection %q (assembly bug — J12)",
			req.Connection.UID, handle.ConnectionUID)
	}
	path, err := pollTaskPath(handle)
	if err != nil {
		return contract.OperationStatus{}, err
	}
	client, err := a.buildExecutorClient(req.Connection)
	if err != nil {
		return contract.OperationStatus{}, err
	}

	var st struct {
		Status     string `json:"status"`
		ExitStatus string `json:"exitstatus"`
	}
	if err := client.get(ctx, path, "poll", &st); err != nil {
		return contract.OperationStatus{}, err
	}
	switch {
	case st.Status == "running":
		return contract.OperationStatus{State: contract.OperationStateRunning}, nil
	case st.Status == "stopped" && st.ExitStatus == "OK":
		return contract.OperationStatus{State: contract.OperationStateSucceeded, Detail: upidDetail(handle, st.ExitStatus)}, nil
	case st.Status == "stopped":
		return contract.OperationStatus{State: contract.OperationStateFailed, Detail: upidDetail(handle, st.ExitStatus)}, nil
	}
	return contract.OperationStatus{}, fmt.Errorf("proxmox: task status %q is outside the PVE vocabulary (running|stopped)", st.Status)
}
