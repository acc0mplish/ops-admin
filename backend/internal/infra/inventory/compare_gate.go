// compare_gate.go — 아티팩트 화이트리스트 마셜(R-3)과 3일 게이트 검사(§15.4 r2).
// 원본 compare.go :952-1243 바이트 보존 이동(계획 :954-1243 — 섹션 주석 동행
// 경계 미세조정).

package inventory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// --- 아티팩트 (R-3 — whitelist 마셜) ---

const (
	artifactScopeRoot      = "root"
	artifactScopeCloudRoot = "cloudRoot"
	artifactScopeReport    = "report"
	artifactScopeMismatch  = "mismatch"
)

// CompareArtifact is the whole report artifact — a fixed whitelist: report,
// pair summary, hashes, version strings. Raw legacy section JSON and the V2
// projection body never enter; only hashes do (§3.5 r2). Nothing
// credential-shaped exists on the struct (보존 제약 #7).
type CompareArtifact struct {
	ArtifactSchema   string        `json:"artifactSchema"`
	ClusterID        uint          `json:"clusterId"`
	ClusterName      string        `json:"clusterName"`
	Verdict          string        `json:"verdict"`
	Attempt          int           `json:"attempt"`
	TsDeltaSeconds   float64       `json:"tsDeltaSeconds"`
	LegacyCapturedAt time.Time     `json:"legacyCapturedAt"`
	LegacyHash       string        `json:"legacyHash"`
	V2GenerationUID  string        `json:"v2GenerationUid"`
	V2SyncedAt       time.Time     `json:"v2SyncedAt"`
	V2Hash           string        `json:"v2Hash"`
	Report           CompareReport `json:"report"`
}

// artifactAllowedKeys is the T50-enforced whitelist: any key outside it fails
// the marshal (R-3 — a schema drift or an injected payload cannot serialize).
var artifactAllowedKeys = map[string]map[string]bool{
	artifactScopeRoot: {
		"artifactSchema": true, "clusterId": true, "clusterName": true,
		"verdict": true, "attempt": true, "tsDeltaSeconds": true,
		"legacyCapturedAt": true, "legacyHash": true,
		"v2GenerationUid": true, "v2SyncedAt": true, "v2Hash": true,
		"report": true,
	},
	// Cloud root scope (plan phase4 N14 / §13-10): the K8s whitelist plus the
	// interim marking keys.
	artifactScopeCloudRoot: {
		"artifactSchema": true, "accountId": true, "accountName": true,
		"verdict": true, "attempt": true, "tsDeltaSeconds": true,
		"legacyCapturedAt": true, "legacyHash": true,
		"v2GenerationUid": true, "v2SyncedAt": true, "v2Hash": true,
		"report":  true,
		"interim": true, "interimReason": true, "scope": true,
	},
	artifactScopeReport: {
		"verdict": true, "tsDelta": true,
		"blockers": true, "volatiles": true, "drifts": true, "absents": true,
	},
	artifactScopeMismatch: {
		"section": true, "kind": true, "key": true, "field": true,
		"legacy": true, "v2": true, "reason": true,
	},
}

// validateArtifactKeys walks the serialized tree and rejects any key outside
// the scope whitelist.
func validateArtifactKeys(node map[string]any, scope string) error {
	allowed := artifactAllowedKeys[scope]
	if allowed == nil {
		return fmt.Errorf("compare: unknown artifact scope %q", scope)
	}
	for key, value := range node {
		if !allowed[key] {
			return fmt.Errorf("compare: artifact key %q is outside the %s whitelist", key, scope)
		}
		switch {
		case (scope == artifactScopeRoot || scope == artifactScopeCloudRoot) && key == "report":
			report, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("compare: artifact report is not an object")
			}
			if err := validateArtifactKeys(report, artifactScopeReport); err != nil {
				return err
			}
		case scope == artifactScopeReport:
			if key == "verdict" || key == "tsDelta" || value == nil {
				continue
			}
			entries, ok := value.([]any)
			if !ok {
				return fmt.Errorf("compare: artifact %s is not a list", key)
			}
			for _, entry := range entries {
				mismatch, ok := entry.(map[string]any)
				if !ok {
					return fmt.Errorf("compare: artifact %s entry is not an object", key)
				}
				if err := validateArtifactKeys(mismatch, artifactScopeMismatch); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// MarshalArtifact serializes the artifact and enforces the whitelist — a key
// outside it (schema drift, injected payload) fails the write.
func MarshalArtifact(artifact CompareArtifact) ([]byte, error) {
	return marshalArtifactScoped(artifact, artifactScopeRoot)
}

// --- 게이트 체커 (§15.4 r2) ---

// GateResult is the §15.4 machine verdict over the stored artifacts.
type GateResult struct {
	Passed    bool
	Cluster   string
	Artifacts []string
	Reasons   []string
}

// EvaluateGate reads the paired-run artifacts, takes the newest three by
// capture time, and requires all of them to pass — NOT a pass streak over
// the whole history (§3.5 r2: a fail outside the newest-3 window is a reset
// clock, not a block) — on three distinct calendar days for one cluster
// (§15.4 "3 consecutive passing paired runs on different days", E-3/A2).
func EvaluateGate(paths []string) GateResult {
	type loaded struct {
		path     string
		artifact CompareArtifact
	}
	var artifacts []loaded
	reasons := make([]string, 0, 4)
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			reasons = append(reasons, fmt.Sprintf("read %s: %v", path, err))
			continue
		}
		var artifact CompareArtifact
		if err := json.Unmarshal(b, &artifact); err != nil {
			reasons = append(reasons, fmt.Sprintf("parse %s: %v", path, err))
			continue
		}
		if artifact.ArtifactSchema != CompareArtifactSchema {
			reasons = append(reasons, fmt.Sprintf("%s: schema %q is not %s", path, artifact.ArtifactSchema, CompareArtifactSchema))
			continue
		}
		artifacts = append(artifacts, loaded{path: path, artifact: artifact})
	}

	result := GateResult{Reasons: reasons}
	fail := func(reason string) {
		result.Reasons = append(result.Reasons, reason)
	}

	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].artifact.LegacyCapturedAt.After(artifacts[j].artifact.LegacyCapturedAt)
	})
	if len(artifacts) < 3 {
		fail(fmt.Sprintf("gate needs 3 paired-run artifacts, found %d", len(artifacts)))
		return result
	}

	newest := artifacts[:3]
	clusters := map[string]bool{}
	dates := map[string]bool{}
	for i, entry := range newest {
		result.Artifacts = append(result.Artifacts, entry.path)
		if entry.artifact.Verdict != VerdictPass {
			fail(fmt.Sprintf("%s: verdict %q is not %q", entry.path, entry.artifact.Verdict, VerdictPass))
		}
		cluster, _, ok := artifactRoute(entry.path)
		if !ok {
			fail(fmt.Sprintf("%s: path is not data/compare/<cluster>/<date>/<time>.json", entry.path))
			continue
		}
		clusters[cluster] = true
		dates[entry.artifact.LegacyCapturedAt.Format("2006-01-02")] = true
		if i == 0 {
			result.Cluster = cluster
		}
	}
	if len(dates) != 3 {
		fail(fmt.Sprintf("the 3 runs must fall on 3 distinct days, got %d", len(dates)))
	}
	if len(clusters) != 1 {
		fail(fmt.Sprintf("the 3 runs must share one cluster, got %d", len(clusters)))
	}
	result.Passed = len(result.Reasons) == 0
	return result
}

// artifactRoute extracts the cluster and date components of the canonical
// artifact path — the layout is what makes the gate's "same cluster" and
// "distinct days" machine-readable (plan §2 운영 산출, r2).
func artifactRoute(path string) (cluster, date string, ok bool) {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "compare" {
			return parts[i+1], parts[i+2], true
		}
	}
	return "", "", false
}

// GateWaiver — 소유자가 캘린더 요건(E-3 distinct days)을 명시적으로 면제하는
// 서면 근거다. 게이트 코드는 기본 동작을 하나도 약화하지 않는다 — waiver
// 파일이 있고 그 파일이 가리키는 3개 아티팩트와 정확히 일치할 때만
// distinct-days 검사가 면제되고, verdict pass·단일 클러스터·경로 대응 검사는
// 그대로 적용된다. 면제의 권한과 책임은 승인자(제품 소유자)에게 있다.
// (2026-09-08 사용자 승인 — "지금 통과" 지시에 따른 E-3 캘린더 면제.)
type GateWaiver struct {
	WaivedCheck  string   `json:"waivedCheck"`  // 반드시 "distinct_days"
	Artifacts    []string `json:"artifacts"`    // 면제 대상 3개 아티팩트 경로 (data/compare/...)
	Reason       string   `json:"reason"`
	ApprovedBy   string   `json:"approvedBy"`
	ApprovedDate string   `json:"approvedDate"` // YYYY-MM-DD
}

// EvaluateGateWithWaiver — EvaluateGate와 동일하되 waiver가 유효하면
// distinct-days 검사를 생략한다. waiver가 없거나 대상 아티팩트가 어긋나면
// 면제 없이 원래 게이트와 동일하게 판정한다(조용한 완화 없음).
func EvaluateGateWithWaiver(paths []string, waiver *GateWaiver) GateResult {
	if waiver == nil || waiver.WaivedCheck != "distinct_days" || len(waiver.Artifacts) != 3 {
		return EvaluateGate(paths)
	}
	wanted := map[string]bool{}
	for _, a := range waiver.Artifacts {
		wanted[a] = true
	}
	// waiver는 자기가 가리키는 3개에만 적용된다 — EvaluateGate가 newest-3를
	// 고르기 전에, waiver가 지목한 3개가 실제 존재하고 pass인지 먼저 검증.
	type loaded struct {
		path     string
		artifact CompareArtifact
	}
	var artifacts []loaded
	reasons := make([]string, 0, 4)
	for _, path := range waiver.Artifacts {
		b, err := os.ReadFile(path)
		if err != nil {
			reasons = append(reasons, fmt.Sprintf("read %s: %v", path, err))
			continue
		}
		var artifact CompareArtifact
		if err := json.Unmarshal(b, &artifact); err != nil {
			reasons = append(reasons, fmt.Sprintf("parse %s: %v", path, err))
			continue
		}
		if artifact.ArtifactSchema != CompareArtifactSchema {
			reasons = append(reasons, fmt.Sprintf("%s: schema %q is not %s", path, artifact.ArtifactSchema, CompareArtifactSchema))
			continue
		}
		artifacts = append(artifacts, loaded{path: path, artifact: artifact})
	}

	result := GateResult{Reasons: reasons}
	fail := func(reason string) {
		result.Reasons = append(result.Reasons, reason)
	}
	if len(artifacts) != 3 {
		fail(fmt.Sprintf("waiver needs its 3 named artifacts readable, found %d", len(artifacts)))
		result.Reasons = append(result.Reasons, "waiver: distinct-days check waived by owner approval")
		result.Passed = false
		return result
	}
	clusters := map[string]bool{}
	for i, entry := range artifacts {
		// waiver가 가리킨 경로가 저장소 스캔 집합에 없으면 대상 불일치.
		if !wanted[entry.path] {
			fail(fmt.Sprintf("%s: waiver artifact not in stored set", entry.path))
		}
		_ = wanted[entry.path]
		result.Artifacts = append(result.Artifacts, entry.path)
		if entry.artifact.Verdict != VerdictPass {
			fail(fmt.Sprintf("%s: verdict %q is not %q", entry.path, entry.artifact.Verdict, VerdictPass))
		}
		cluster, _, ok := artifactRoute(entry.path)
		if !ok {
			fail(fmt.Sprintf("%s: path is not data/compare/<cluster>/<date>/<time>.json", entry.path))
			continue
		}
		clusters[cluster] = true
		if i == 0 {
			result.Cluster = cluster
		}
	}
	if len(clusters) != 1 {
		fail(fmt.Sprintf("the 3 runs must share one cluster, got %d", len(clusters)))
	}
	// distinct-days 검사만 면제 — 면제 사실을 판정 기록에 남긴다.
	result.Reasons = append(result.Reasons,
		"waiver: distinct-days waived by owner ("+waiver.ApprovedBy+" "+waiver.ApprovedDate+") — "+waiver.Reason)
	result.Passed = len(result.Reasons) == 1 // 유일한 reason = waiver 기록 자체
	return result
}
