package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	deliverypkg "github.com/sori883/ai-dd/src/internal/delivery"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

var (
	deliveryInputResolver = resolveDeliveryInput
	deliveryRootCloser    = (*os.Root).Close
)

func deliveryNext(
	getwd func() (string, error),
	getenv func(string) string,
	next func(context.Context, deliverypkg.RunStageInput) (deliverypkg.DeliveryResult, error),
) func(string) ([]byte, error) {
	return func(explicitDir string) ([]byte, error) {
		return runDeliveryOperation(getwd, getenv, explicitDir, next)
	}
}

func deliveryContinue(
	getwd func() (string, error),
	getenv func(string) string,
	continueDelivery func(context.Context, deliverypkg.RunStageInput, string) (deliverypkg.DeliveryResult, error),
) func(string, string) ([]byte, error) {
	return func(token, explicitDir string) ([]byte, error) {
		return runDeliveryOperation(getwd, getenv, explicitDir, func(ctx context.Context, input deliverypkg.RunStageInput) (deliverypkg.DeliveryResult, error) {
			return continueDelivery(ctx, input, token)
		})
	}
}

func deliveryReadContext(
	getwd func() (string, error),
	getenv func(string) string,
	read func(context.Context, deliverypkg.RunStageInput) (deliverypkg.ContextReadResult, error),
) func(string) ([]byte, error) {
	return func(explicitDir string) ([]byte, error) {
		return runContextReadOperation(getwd, getenv, explicitDir, func(ctx context.Context, input deliverypkg.RunStageInput) ([]byte, error) {
			result, err := read(ctx, input)
			if err != nil {
				return nil, err
			}
			return json.Marshal(result)
		})
	}
}

func deliveryContinueContext(
	getwd func() (string, error),
	getenv func(string) string,
	continueRead func(context.Context, deliverypkg.RunStageInput, string) (deliverypkg.ContextReadResult, error),
) func(string, string) ([]byte, error) {
	return func(token, explicitDir string) ([]byte, error) {
		return runContextReadOperation(getwd, getenv, explicitDir, func(ctx context.Context, input deliverypkg.RunStageInput) ([]byte, error) {
			result, err := continueRead(ctx, input, token)
			if err != nil {
				return nil, err
			}
			return json.Marshal(result)
		})
	}
}

func runContextReadOperation(
	getwd func() (string, error),
	getenv func(string) string,
	explicitDir string,
	operation func(context.Context, deliverypkg.RunStageInput) ([]byte, error),
) (wire []byte, err error) {
	input, projectRoot, recordRoot, err := deliveryInputResolver(getwd, getenv, explicitDir)
	if err != nil {
		return nil, err
	}
	defer func() {
		recordCloseErr := deliveryRootCloser(recordRoot)
		projectCloseErr := deliveryRootCloser(projectRoot)
		closeErr := errors.Join(
			wrapDeliveryRootCloseError("record", recordCloseErr),
			wrapDeliveryRootCloseError("project", projectCloseErr),
		)
		if closeErr != nil {
			wire = nil
			if err == nil {
				err = closeErr
			} else {
				err = fmt.Errorf("context read operation failed during root cleanup: %v; %w", err, closeErr)
			}
		}
	}()
	if operation == nil {
		return nil, errors.New("context read operation is unavailable")
	}
	wire, err = operation(context.Background(), input)
	if err != nil {
		return nil, err
	}
	if len(wire) == 0 || !json.Valid(wire) {
		return nil, errors.New("context read operation returned invalid JSON")
	}
	return append([]byte(nil), wire...), nil
}

func runDeliveryOperation(
	getwd func() (string, error),
	getenv func(string) string,
	explicitDir string,
	operation func(context.Context, deliverypkg.RunStageInput) (deliverypkg.DeliveryResult, error),
) (wire []byte, err error) {
	input, projectRoot, recordRoot, err := deliveryInputResolver(getwd, getenv, explicitDir)
	if err != nil {
		return nil, err
	}
	defer func() {
		recordCloseErr := deliveryRootCloser(recordRoot)
		projectCloseErr := deliveryRootCloser(projectRoot)
		closeErr := errors.Join(
			wrapDeliveryRootCloseError("record", recordCloseErr),
			wrapDeliveryRootCloseError("project", projectCloseErr),
		)
		if closeErr != nil {
			wire = nil
			if err == nil {
				err = closeErr
			} else {
				// A cleanup failure makes the whole adapter operation internal,
				// even when the operation itself produced a workflow rejection.
				// Keep the cleanup cause unwrap-able without preserving the
				// WorkflowError classification through errors.Join.
				err = fmt.Errorf("delivery operation failed during root cleanup: %v; %w", err, closeErr)
			}
		}
	}()
	if operation == nil {
		return nil, errors.New("delivery operation is unavailable")
	}
	result, err := operation(context.Background(), input)
	if err != nil {
		return nil, err
	}
	if len(result.Wire) == 0 {
		return nil, errors.New("delivery operation returned an empty directive")
	}
	return append([]byte(nil), result.Wire...), nil
}

func resolveDeliveryInput(
	getwd func() (string, error),
	getenv func(string) string,
	explicitDir string,
) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
	if getwd == nil {
		return deliverypkg.RunStageInput{}, nil, nil, errors.New("read working directory: callback is unavailable")
	}
	workingDir, err := getwd()
	if err != nil {
		return deliverypkg.RunStageInput{}, nil, nil, fmt.Errorf("read working directory: %w", err)
	}
	rootInput := workspace.RootInput{
		ExplicitDir: explicitDir,
		WorkingDir:  workingDir,
	}
	if getenv != nil {
		rootInput.AIDLCProjectDir = getenv("AIDLC_PROJECT_DIR")
		rootInput.ClaudeProjectDir = getenv("CLAUDE_PROJECT_DIR")
	}
	projectPath := workspace.ResolveRoot(rootInput)
	if !filepath.IsAbs(projectPath) {
		return deliverypkg.RunStageInput{}, nil, nil, fmt.Errorf("resolve project root %q: %w", projectPath, fs.ErrInvalid)
	}
	projectRoot, err := os.OpenRoot(projectPath)
	if err != nil {
		return deliverypkg.RunStageInput{}, nil, nil, fmt.Errorf("open project root %q: %w", projectPath, err)
	}
	closeProjectOnError := true
	defer func() {
		if closeProjectOnError {
			_ = projectRoot.Close()
		}
	}()

	activeSpace := workspace.ActiveSpace(projectRoot.FS())
	if err := validateDeliveryComponent(activeSpace, "active space"); err != nil {
		return deliverypkg.RunStageInput{}, nil, nil, err
	}
	intentsPath := filepath.ToSlash(filepath.Join("aidlc", "spaces", activeSpace, "intents"))
	intentsFS, err := fs.Sub(projectRoot.FS(), intentsPath)
	if err != nil {
		return deliverypkg.RunStageInput{}, nil, nil, fmt.Errorf("open active intents %q: %w", intentsPath, err)
	}
	activeIntent, found := workspace.ActiveIntent(intentsFS, "")
	if !found {
		return deliverypkg.RunStageInput{}, nil, nil, errors.New("active intent is not selected")
	}
	if err := validateDeliveryComponent(activeIntent, "active intent"); err != nil {
		return deliverypkg.RunStageInput{}, nil, nil, err
	}
	var intentUUID *string
	if intents, listErr := workspace.ListIntents(intentsFS, &activeIntent); listErr == nil {
		for _, candidate := range intents {
			if candidate.DirName == nil || *candidate.DirName != activeIntent {
				continue
			}
			value := candidate.UUID
			intentUUID = &value
			break
		}
	}
	identity, err := recordlock.NewIdentity(projectPath, activeSpace, activeIntent)
	if err != nil {
		return deliverypkg.RunStageInput{}, nil, nil, fmt.Errorf("create delivery identity: %w", err)
	}
	recordPath := filepath.ToSlash(filepath.Join("aidlc", "spaces", activeSpace, "intents", activeIntent))
	recordRoot, err := projectRoot.OpenRoot(recordPath)
	if err != nil {
		return deliverypkg.RunStageInput{}, nil, nil, fmt.Errorf("open record root %q: %w", recordPath, err)
	}
	closeProjectOnError = false
	return deliverypkg.RunStageInput{
		Identity:    identity,
		ProjectRoot: projectRoot,
		RecordRoot:  recordRoot,
		IntentUUID:  intentUUID,
	}, projectRoot, recordRoot, nil
}

func validateDeliveryComponent(value, label string) error {
	if value == "" || value == "." || strings.Contains(value, "/") || strings.Contains(value, `\`) || !fs.ValidPath(value) {
		return fmt.Errorf("%s %q is not a safe path component: %w", label, value, fs.ErrInvalid)
	}
	if _, err := filepath.Localize(value); err != nil {
		return fmt.Errorf("%s %q is not a native path component: %w", label, value, errors.Join(fs.ErrInvalid, err))
	}
	return nil
}

func wrapDeliveryRootCloseError(name string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("close %s root: %w", name, err)
}
