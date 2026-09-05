package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	deliverypkg "github.com/sori883/ai-dd/src/internal/delivery"
	"github.com/sori883/ai-dd/src/internal/recordlock"
)

func TestDeliveryReadContextAdapterMarshalsResultAndClosesRoots(t *testing.T) {
	previousResolver := deliveryInputResolver
	t.Cleanup(func() { deliveryInputResolver = previousResolver })
	projectPath := t.TempDir()
	recordPath := t.TempDir()
	projectRoot, err := os.OpenRoot(projectPath)
	if err != nil {
		t.Fatalf("OpenRoot(project): %v", err)
	}
	recordRoot, err := os.OpenRoot(recordPath)
	if err != nil {
		_ = projectRoot.Close()
		t.Fatalf("OpenRoot(record): %v", err)
	}
	identity, err := recordlock.NewIdentity(projectPath, "team", "build")
	if err != nil {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
		t.Fatalf("NewIdentity(): %v", err)
	}
	deliveryInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return deliverypkg.RunStageInput{Identity: identity, ProjectRoot: projectRoot, RecordRoot: recordRoot}, projectRoot, recordRoot, nil
	}
	callback := deliveryReadContext(nil, nil, func(context.Context, deliverypkg.RunStageInput) (deliverypkg.ContextReadResult, error) {
		return deliverypkg.ContextReadResult{Kind: deliverypkg.ContextReadKindChunk, Stage: "intent-capture", Slot: deliverypkg.ContextReadSlotStage, Index: 1, Part: 1, Parts: 1, Text: "ok", Complete: true}, nil
	})
	wire, err := callback("/tmp/project")
	if err != nil {
		t.Fatalf("deliveryReadContext() error = %v", err)
	}
	if !json.Valid(wire) || len(wire) == 0 {
		t.Fatalf("deliveryReadContext() wire = %q, want JSON", wire)
	}
	if string(wire) != `{"kind":"context-chunk","stage":"intent-capture","slot":"stage-file","index":1,"part":1,"parts":1,"content_sha256":"","text":"ok","complete":true}` {
		t.Errorf("deliveryReadContext() wire = %q, want canonical context JSON", wire)
	}
}

func TestDeliveryReadContextAdapterTreatsRootCloseFailureAsInternal(t *testing.T) {
	previousResolver := deliveryInputResolver
	previousCloser := deliveryRootCloser
	t.Cleanup(func() {
		deliveryInputResolver = previousResolver
		deliveryRootCloser = previousCloser
	})
	projectPath := t.TempDir()
	recordPath := t.TempDir()
	projectRoot, err := os.OpenRoot(projectPath)
	if err != nil {
		t.Fatalf("OpenRoot(project): %v", err)
	}
	recordRoot, err := os.OpenRoot(recordPath)
	if err != nil {
		_ = projectRoot.Close()
		t.Fatalf("OpenRoot(record): %v", err)
	}
	identity, err := recordlock.NewIdentity(projectPath, "team", "build")
	if err != nil {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
		t.Fatalf("NewIdentity(): %v", err)
	}
	deliveryInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return deliverypkg.RunStageInput{Identity: identity, ProjectRoot: projectRoot, RecordRoot: recordRoot}, projectRoot, recordRoot, nil
	}
	closeErr := errors.New("injected context root close failure")
	deliveryRootCloser = func(root *os.Root) error {
		if root == recordRoot {
			return closeErr
		}
		return root.Close()
	}
	callback := deliveryReadContext(nil, nil, func(context.Context, deliverypkg.RunStageInput) (deliverypkg.ContextReadResult, error) {
		return deliverypkg.ContextReadResult{Kind: deliverypkg.ContextReadKindChunk, Complete: true}, nil
	})
	if _, err := callback(""); err == nil || !errors.Is(err, closeErr) {
		t.Fatalf("deliveryReadContext(close failure) error = %v, want injected close error", err)
	}
	_ = recordRoot.Close()
}
