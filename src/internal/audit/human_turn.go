package audit

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"

	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/state"
)

var (
	// ErrHumanTurnObservationStale means the workflow changed between the
	// hook's observation and its append transaction.
	ErrHumanTurnObservationStale = errors.New("audit: HUMAN_TURN observation is stale")
	// ErrNoActiveWorkflow means that a hook observation found no running stage.
	ErrNoActiveWorkflow = errors.New("audit: no active workflow")
)

// HumanTurnObservation carries the private state and audit snapshot from the
// hook's first read to its lock-protected revalidation.
type HumanTurnObservation struct {
	identity        recordlock.Identity
	stateDigest     [32]byte
	stage           string
	auditGeneration [32]byte
}

// ObserveHumanTurn captures an active workflow snapshot for a later append.
// State and audit are read while holding the record lock so the observation is
// bound to one identity and one coherent ledger generation.
func ObserveHumanTurn(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root) (HumanTurnObservation, error) {
	if ctx == nil {
		return HumanTurnObservation{}, fmt.Errorf("audit: HUMAN_TURN observation context is nil: %w", fs.ErrInvalid)
	}
	if projectRoot == nil || recordRoot == nil {
		return HumanTurnObservation{}, fmt.Errorf("audit: HUMAN_TURN observation roots are required: %w", ErrInvalidRoot)
	}
	var observation HumanTurnObservation
	err := recordlock.With(ctx, identity, func(guard *recordlock.Guard) error {
		var err error
		observation, err = humanTurnObservationWithGuard(ctx, identity, guard, projectRoot, recordRoot)
		return err
	})
	if err != nil {
		return HumanTurnObservation{}, err
	}
	return observation, nil
}

// RecordHumanTurnIfCurrent appends only when the lock-protected workflow
// snapshot still matches observation.
func RecordHumanTurnIfCurrent(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, observation HumanTurnObservation) error {
	if ctx == nil {
		return fmt.Errorf("audit: HUMAN_TURN append context is nil: %w", fs.ErrInvalid)
	}
	if observation.identity != identity {
		return fmt.Errorf("audit: HUMAN_TURN identity advanced since observation: %w", ErrHumanTurnObservationStale)
	}
	return recordlock.With(ctx, identity, func(guard *recordlock.Guard) error {
		current, err := humanTurnObservationWithGuard(ctx, identity, guard, projectRoot, recordRoot)
		if errors.Is(err, ErrNoActiveWorkflow) {
			return ErrHumanTurnObservationStale
		}
		if err != nil {
			return err
		}
		if !sameHumanTurnObservation(observation, current) {
			return ErrHumanTurnObservationStale
		}
		return AppendForIdentity(ctx, identity, guard, projectRoot, recordRoot, []Event{{Event: "HUMAN_TURN"}})
	})
}

func humanTurnObservationWithGuard(ctx context.Context, identity recordlock.Identity, guard *recordlock.Guard, projectRoot, recordRoot *os.Root) (HumanTurnObservation, error) {
	if err := ValidateRecordBinding(ctx, identity, guard, projectRoot, recordRoot); err != nil {
		return HumanTurnObservation{}, err
	}
	document, err := state.ReadDocument(recordRoot)
	if err != nil {
		return HumanTurnObservation{}, fmt.Errorf("audit: HUMAN_TURN read state: %w", err)
	}
	stage := document.State.CurrentStage()
	if document.State.WorkflowStatus() != state.WorkflowStatusRunning || stage == "" || stage == "none" {
		return HumanTurnObservation{}, ErrNoActiveWorkflow
	}
	records, err := ReadEvents(ctx, identity, guard, projectRoot, recordRoot)
	if err != nil {
		return HumanTurnObservation{}, fmt.Errorf("audit: HUMAN_TURN read audit: %w", err)
	}
	return HumanTurnObservation{
		identity:        identity,
		stateDigest:     sha256.Sum256(document.Content),
		stage:           stage,
		auditGeneration: humanTurnAuditGeneration(records),
	}, nil
}

func sameHumanTurnObservation(want, got HumanTurnObservation) bool {
	return want.identity == got.identity && want.stateDigest == got.stateDigest &&
		want.stage == got.stage && want.auditGeneration == got.auditGeneration
}

func humanTurnAuditGeneration(records []AuditRecord) [32]byte {
	hash := sha256.New()
	for _, record := range records {
		writeHumanTurnDigestPart(hash, record.Event)
		writeHumanTurnDigestPart(hash, record.Shard)
		fmt.Fprintf(hash, "%d:%d:%d;", record.Position, record.Timestamp.Unix(), record.Timestamp.Nanosecond())
		keys := make([]string, 0, len(record.Fields))
		for key := range record.Fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			writeHumanTurnDigestPart(hash, key)
			writeHumanTurnDigestPart(hash, record.Fields[key])
		}
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return digest
}

func writeHumanTurnDigestPart(hash interface{ Write([]byte) (int, error) }, value string) {
	fmt.Fprintf(hash, "%d:", len(value))
	_, _ = hash.Write([]byte(value))
	_, _ = hash.Write([]byte{0})
}

// RecordHumanTurn appends one payload-free HUMAN_TURN presence event to the
// identity-bound audit ledger. The caller supplies no prompt or choice data.
func RecordHumanTurn(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root) error {
	if ctx == nil {
		return fmt.Errorf("audit: HUMAN_TURN context is nil: %w", fs.ErrInvalid)
	}
	return recordlock.With(ctx, identity, func(guard *recordlock.Guard) error {
		return AppendForIdentity(ctx, identity, guard, projectRoot, recordRoot, []Event{{Event: "HUMAN_TURN"}})
	})
}
