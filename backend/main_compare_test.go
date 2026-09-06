package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/model"
)

// The v1 DTO → engine capture conversion copies only the compared sections
// and never carries credential-shaped fields.
func TestLegacyCaptureFromDetail(t *testing.T) {
	detail := model.K8sClusterDetail{
		Cluster: model.K8sClusterView{ID: 7, Name: "kind-seed", Version: "v1.31.0"},
		Nodes:   []model.K8sNodeItem{{Name: "kind-control", Role: "control-plane", Status: "Ready", CPU: "8", Memory: "32660 MB"}},
		Pods:    []model.K8sPodItem{{Name: "coredns-abc", Namespace: "kube-system", Status: "Running", Restarts: 3}},
		Workloads: []model.K8sWorkloadItem{
			{Name: "coredns", Type: "Deployment", Namespace: "kube-system", Ready: "2/2"},
			{Name: "seed", Type: "Job", Namespace: "batch", Ready: "1/1"},
		},
		ConfigStorage: model.K8sConfigStorageSection{
			Storage: []model.K8sStorageItem{{Name: "pvc-1", Kind: "PV", Capacity: "10Gi", StorageClass: "standard", AccessModes: "RWO"}},
		},
	}
	capture := legacyCaptureFromDetail(detail, time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC))

	if capture.CapturedAt.IsZero() {
		t.Fatalf("capture timestamp not set")
	}
	if len(capture.Nodes) != 1 || capture.Nodes[0].Name != "kind-control" {
		t.Fatalf("nodes not copied: %+v", capture.Nodes)
	}
	if len(capture.Workloads) != 2 || capture.Workloads[1].Type != "Job" {
		t.Fatalf("workloads not copied: %+v", capture.Workloads)
	}
	if len(capture.Storage) != 1 || capture.Storage[0].Kind != "PV" {
		t.Fatalf("storage not copied: %+v", capture.Storage)
	}
	if capture.Pods[0].Restarts != 3 {
		t.Fatalf("pod restarts not copied: %+v", capture.Pods)
	}
}

// --cluster is mandatory for the capture path.
func TestRunCompareInventoryRequiresCluster(t *testing.T) {
	if code := runCompareInventory([]string{}); code != 1 {
		t.Fatalf("exit code = %d, want 1 for missing --cluster", code)
	}
}

// --gate evaluates the artifacts under the data directory: an empty
// directory fails the gate (exit 1) without touching the database.
func TestRunCompareInventoryGateEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	if code := runCompareInventory([]string{"--gate", "--cluster", "7", "--data", dir}); code != 1 {
		t.Fatalf("exit code = %d, want 1 for an empty artifact directory", code)
	}
}

// --gate passes (exit 0) on three pass artifacts over three distinct days of
// one cluster.
func TestRunCompareInventoryGatePassesOnThreeDistinctDays(t *testing.T) {
	dir := t.TempDir()
	clusterDir := filepath.Join(dir, "compare", "7")
	for _, d := range []int{1, 2, 3} {
		at := time.Date(2026, 9, d, 8, 0, 0, 0, time.UTC)
		artifact := inventory.CompareArtifact{
			ArtifactSchema: inventory.CompareArtifactSchema, ClusterName: "kind-seed",
			Verdict: inventory.VerdictPass, LegacyCapturedAt: at, V2SyncedAt: at,
		}
		b, err := inventory.MarshalArtifact(artifact)
		if err != nil {
			t.Fatalf("MarshalArtifact: %v", err)
		}
		path := filepath.Join(clusterDir, at.Format("2006-01-02"), at.Format("150405")+".json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if code := runCompareInventory([]string{"--gate", "--cluster", "7", "--data", dir}); code != 0 {
		t.Fatalf("exit code = %d, want 0 for a cleared gate", code)
	}
}
