package main

import (
	"context"
	"io"
	"os"

	"github.com/sori883/ai-dd/src/internal/audit"
	deliverypkg "github.com/sori883/ai-dd/src/internal/delivery"
	"github.com/sori883/ai-dd/src/internal/recordlock"
)

const humanTurnPayloadLimit = 64 * 1024

var (
	humanTurnInputResolver = func(getwd func() (string, error), getenv func(string) string, explicitDir string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return deliveryInputResolver(getwd, getenv, explicitDir)
	}
	humanTurnObserve          func(context.Context, recordlock.Identity, *os.Root, *os.Root) (audit.HumanTurnObservation, error) = audit.ObserveHumanTurn
	humanTurnRecordIfCurrent  func(context.Context, recordlock.Identity, *os.Root, *os.Root, audit.HumanTurnObservation) error   = audit.RecordHumanTurnIfCurrent
	humanTurnAfterObservation                                                                                                    = func() {}
	humanTurnRootCloser                                                                                                          = func(root *os.Root) error { return deliveryRootCloser(root) }
)

// runHumanTurnHook is the fail-open UserPromptSubmit boundary. It records an
// operational presence receipt when the caller has resolved an active record;
// origin authentication is outside this function and is not implied here.
// Stdin is never used as authority, prompt text, or choice data.
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
	if _, err := read(); err != nil {
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
	if humanTurnObserve == nil || humanTurnRecordIfCurrent == nil {
		return nil
	}
	observation, observeErr := humanTurnObserve(context.Background(), input.Identity, projectRoot, recordRoot)
	if observeErr != nil {
		return nil
	}
	if humanTurnAfterObservation != nil {
		humanTurnAfterObservation()
	}
	if err := humanTurnRecordIfCurrent(context.Background(), input.Identity, projectRoot, recordRoot, observation); err != nil {
		return nil
	}
	return nil
}

func humanTurnHook(reader io.Reader, getwd func() (string, error), getenv func(string) string) error {
	if reader == nil {
		return nil
	}
	return runHumanTurnHook(func() ([]byte, error) {
		return io.ReadAll(io.LimitReader(reader, humanTurnPayloadLimit))
	}, getwd, getenv)
}
