package audit

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/sori883/ai-dd/src/internal/recordlock"
)

// RecordHumanTurn appends one authority-only HUMAN_TURN event to the
// identity-bound audit ledger. The caller supplies no prompt or choice data.
func RecordHumanTurn(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root) error {
	if ctx == nil {
		return fmt.Errorf("audit: HUMAN_TURN context is nil: %w", fs.ErrInvalid)
	}
	return recordlock.With(ctx, identity, func(guard *recordlock.Guard) error {
		return AppendForIdentity(ctx, identity, guard, projectRoot, recordRoot, []Event{{Event: "HUMAN_TURN"}})
	})
}
