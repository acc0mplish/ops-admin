package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

// captureStderr swaps os.Stderr for a pipe, runs fn, and returns what was
// written — the closure notice is the primary observable of the closed CLI.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out := <-done
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	return out
}

// compare-inventory was closed in G1 (phase6-plan §J8·§12 #5): the §15 k8s
// pairing ended with the C53 verdict, and removing the legacy read source
// left the capture with nothing to pair against. The dispatch in main.go
// survives, so the command answers with a deprecation notice and a non-zero
// exit for every argument shape — no database access, no capture, no gate.
func TestRunCompareInventoryClosed(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"--cluster", "1"},
		{"--gate", "--cluster", "7", "--data", t.TempDir()},
	} {
		if code := runCompareInventory(args); code != 1 {
			t.Fatalf("args %v: exit code = %d, want 1 (CLI closed in G1)", args, code)
		}
	}
}

// The closure notice names the command and its replacement path so an
// operator hitting the old command learns why it is gone.
func TestRunCompareInventoryClosedNotice(t *testing.T) {
	notice := captureStderr(t, func() {
		if code := runCompareInventory([]string{"--cluster", "1"}); code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
	})
	if !strings.Contains(notice, "compare-inventory") {
		t.Fatalf("notice = %q, want the command name in the closure notice", notice)
	}
	if !strings.Contains(notice, "sync-inventory") {
		t.Fatalf("notice = %q, want the V2 replacement (sync-inventory) mentioned", notice)
	}
}
