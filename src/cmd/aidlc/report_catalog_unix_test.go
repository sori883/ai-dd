//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	deliverypkg "github.com/sori883/ai-dd/src/internal/delivery"
	"github.com/sori883/ai-dd/src/internal/orchestrator"
)

func TestReportAdapterRejectsCatalogFIFOWithoutBlocking(t *testing.T) {
	fixture := newReportAdapterFixture(t)
	previousResolver := reportInputResolver
	t.Cleanup(func() { reportInputResolver = previousResolver })
	reportInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return fixture.input, fixture.projectRoot, fixture.recordRoot, nil
	}
	stagePath := filepath.Join(fixture.input.Identity.ProjectRoot(), ".codex", "tools", "data", "stage-graph.json")
	if err := fixture.projectRoot.Remove(filepath.ToSlash(filepath.Join(".codex", "tools", "data", "stage-graph.json"))); err != nil {
		t.Fatalf("Remove(stage graph): %v", err)
	}
	if err := syscall.Mkfifo(stagePath, 0o600); err != nil {
		t.Fatalf("Mkfifo(stage graph): %v", err)
	}
	callbackCalls := 0
	callback := reportAdapter(nil, nil, func(context.Context, orchestrator.ReportInput) (orchestrator.ReportResult, error) {
		callbackCalls++
		return orchestrator.ReportResult{Kind: orchestrator.ReportKindAwaitingApproval, Slug: "intent-capture"}, nil
	})
	done := make(chan struct{})
	var wire []byte
	var reportErr error
	go func() {
		wire, reportErr = callback("intent-capture", "awaiting-approval", "", "", "")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("report adapter blocked while inspecting catalog FIFO")
	}
	if wire != nil || reportErr == nil {
		t.Fatalf("reportAdapter(FIFO) = (%q, %v), want internal error", wire, reportErr)
	}
	if callbackCalls != 0 {
		t.Fatalf("report callback calls = %d, want 0 for FIFO catalog", callbackCalls)
	}
	if errors.Is(reportErr, orchestrator.ErrInvalidReport) || orchestrator.IsWorkflowError(reportErr) {
		t.Fatalf("reportAdapter(FIFO) error = %v, want internal classification", reportErr)
	}
}
