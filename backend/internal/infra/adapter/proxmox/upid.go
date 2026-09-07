package proxmox

import (
	"fmt"
	"strings"
)

// upid.go — PVE UPID(Unix Process ID) 파싱과 폴 핸들 인코딩 (계획 N2·판정 J4).
//
// UPID는 PVE 비동기 태스크의 provider-네이티브 참조다:
//
//	UPID:<node>:<pid>:<pstart>:<starttime>:<type>:<id>:<user>:
//
// node 성분은 재폴 대상 노드 결정(§14.3 증류 계약 1 — "GET /nodes/{node}/tasks/
// {upid}/status")에 필수고, 나머지 성분은 원문 보존만 한다(재해석 금지 — PVE가
// 유일한 해석자). 핸들 인코딩 `upid|<connUID>|<node>|<rawUPID>`는 k8s rollout
// 핸들(rolloutHandleMarker)의 관례 승계로, connUID 자기서술(J12)과 raw UPID
// 통재 저장(재폴 시 파싱 중복 제거 — node는 어차피 필요해 성분으로 재노출)을
// 한 형태에 담는다. executor 소비면은 Phase D가 소유한다.
//
// 핸들은 task_attempt.handle_ref에 지속된다 — connUID는 공개 식별자(비밀 아님)
// 이고 자격 물질은 실리지 않는다(보존 제약 7).

// upidHandleMarker — ProviderRef 형식의 첫 세그먼트.
const upidHandleMarker = "upid"

// UPID — 파싱된 UPID. Raw에 원문이 보존되고 성분은 열람 편의 재노출이다.
type UPID struct {
	Raw       string
	Node      string
	PID       string
	PStart    string
	StartTime string
	Type      string
	ID        string
	User      string
}

// parseUPID — UPID 원문을 성분으로 분해한다. node는 필수(재폴 경로의 대상),
// 나머지 성분은 비어 있어도 원문 보존을 우선한다. 구조 변형(접두 대소문자·
// 성분 수·후행 콜론 부재)은 거부한다 — 재구성(성분 join)이 원문과 일치하지
// 않는 입력은 PVE가 발행한 형태가 아니다.
func parseUPID(raw string) (UPID, error) {
	parts := strings.Split(raw, ":")
	// "UPID:node:pid:pstart:starttime:type:id:user:" → 9조각(끝이 빈 조각).
	if len(parts) != 9 || parts[len(parts)-1] != "" {
		return UPID{}, fmt.Errorf("proxmox: upid %q does not match UPID:<node>:<pid>:<pstart>:<starttime>:<type>:<id>:<user>:", raw)
	}
	if parts[0] != "UPID" {
		return UPID{}, fmt.Errorf("proxmox: upid %q lacks the UPID: prefix", raw)
	}
	if parts[1] == "" {
		return UPID{}, fmt.Errorf("proxmox: upid %q carries no node component", raw)
	}
	u := UPID{
		Raw:       raw,
		Node:      parts[1],
		PID:       parts[2],
		PStart:    parts[3],
		StartTime: parts[4],
		Type:      parts[5],
		ID:        parts[6],
		User:      parts[7],
	}
	// 변형 거부의 마지막 자물쇠: 원문이 정규 재구성과 일치해야 한다(정규화로
	// 지워질 수 있는 입력 — 이중 콜론 등 — 은 PVE 발행물이 아니다).
	if rebuilt := rebuildUPID(u); rebuilt != raw {
		return UPID{}, fmt.Errorf("proxmox: upid %q is not in canonical form (rebuilds to %q)", raw, rebuilt)
	}
	return u, nil
}

// rebuildUPID — 성분을 정규 형태로 재조립한다(파싱 역연산).
func rebuildUPID(u UPID) string {
	return strings.Join([]string{"UPID", u.Node, u.PID, u.PStart, u.StartTime, u.Type, u.ID, u.User, ""}, ":")
}

// upidHandle — decodeUPIDRef의 결과.
type upidHandle struct {
	ConnectionUID string
	UPID          UPID
}

// encodeUPIDRef — `upid|<connUID>|<node>|<rawUPID>` 인코딩.
func encodeUPIDRef(connUID string, u UPID) string {
	return strings.Join([]string{upidHandleMarker, connUID, u.Node, u.Raw}, "|")
}

// decodeUPIDRef — 핸들 디코딩. node 세그먼트와 raw UPID 파싱 성분의 정합까지
// 단얫한다(핸들 위조·전송 중 손상의 조기 포착 — k8s decodeRolloutRef의
// 대상 정합 단얫 승계).
func decodeUPIDRef(ref string) (upidHandle, error) {
	parts := strings.Split(ref, "|")
	if len(parts) != 4 {
		return upidHandle{}, fmt.Errorf("proxmox: malformed upid handle %q (want upid|<connUID>|<node>|<rawUPID>)", ref)
	}
	if parts[0] != upidHandleMarker {
		return upidHandle{}, fmt.Errorf("proxmox: handle %q is not an upid handle", ref)
	}
	if parts[1] == "" {
		return upidHandle{}, fmt.Errorf("proxmox: upid handle %q carries no connection uid", ref)
	}
	u, err := parseUPID(parts[3])
	if err != nil {
		return upidHandle{}, fmt.Errorf("proxmox: upid handle %q carries an unusable raw UPID: %w", ref, err)
	}
	if u.Node != parts[2] {
		return upidHandle{}, fmt.Errorf("proxmox: upid handle %q node segment %q does not match the raw UPID node %q", ref, parts[2], u.Node)
	}
	return upidHandle{ConnectionUID: parts[1], UPID: u}, nil
}
