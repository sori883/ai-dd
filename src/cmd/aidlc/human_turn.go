package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"

	"github.com/sori883/ai-dd/src/internal/audit"
	deliverypkg "github.com/sori883/ai-dd/src/internal/delivery"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/state"
)

var (
	humanTurnInputResolver = func(getwd func() (string, error), getenv func(string) string, explicitDir string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return deliveryInputResolver(getwd, getenv, explicitDir)
	}
	humanTurnRecord     func(context.Context, recordlock.Identity, *os.Root, *os.Root) error = audit.RecordHumanTurn
	humanTurnRootCloser                                                                      = func(root *os.Root) error { return deliveryRootCloser(root) }
)

// runHumanTurnHook is the fail-open UserPromptSubmit boundary. The payload is
// parsed only as a validity guard; no prompt text, session value, or choice is
// passed to the authority-bearing audit entry.
func runHumanTurnHook(
	read func() ([]byte, error),
	getwd func() (string, error),
	getenv func(string) string,
) (result error) {
	defer func() {
		if recover() != nil {
			result = nil
		}
	}()
	if read == nil {
		return nil
	}
	payload, err := read()
	if err != nil || !validHumanTurnPayload(payload) {
		return nil
	}
	if getenv != nil && getenv("AIDLC_UNATTENDED") == "1" {
		return nil
	}
	if humanTurnInputResolver == nil {
		return nil
	}
	input, projectRoot, recordRoot, resolveErr := humanTurnInputResolver(getwd, getenv, "")
	if resolveErr != nil || projectRoot == nil || recordRoot == nil {
		return nil
	}
	defer func() {
		if humanTurnRootCloser == nil {
			return
		}
		_ = humanTurnRootCloser(recordRoot)
		_ = humanTurnRootCloser(projectRoot)
	}()
	info, statErr := recordRoot.Lstat("aidlc-state.md")
	if statErr != nil || info == nil || !info.Mode().IsRegular() {
		return nil
	}
	document, stateErr := state.ReadDocument(recordRoot)
	if stateErr != nil || document.State.WorkflowStatus() != state.WorkflowStatusRunning {
		return nil
	}
	if currentStage := document.State.CurrentStage(); currentStage == "" || currentStage == "none" {
		return nil
	}
	if humanTurnRecord != nil {
		_ = humanTurnRecord(context.Background(), input.Identity, projectRoot, recordRoot)
	}
	return nil
}

func validHumanTurnPayload(payload []byte) bool {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return false
	}
	var value map[string]any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return false
	}
	return value != nil
}

func humanTurnHook(reader io.Reader, getwd func() (string, error), getenv func(string) string) error {
	if reader == nil {
		return nil
	}
	return runHumanTurnHook(func() ([]byte, error) { return io.ReadAll(reader) }, getwd, getenv)
}
