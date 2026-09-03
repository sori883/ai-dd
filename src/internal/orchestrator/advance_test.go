package orchestrator

import (
	"errors"
	"strconv"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/state"
)

func TestDeriveNextStageUsesSavedActionsAndGraphOrder(t *testing.T) {
	t.Parallel()

	current := parseAdvanceState(t, `- [x] first — EXECUTE
- [ ] skipped-by-state — SKIP
- [ ] second — EXECUTE
- [ ] third — EXECUTE`, "first", "skipped-by-state")
	catalog := loadAdvanceCatalog(t)

	next, ok, err := deriveNextStage(current, catalog, "first")
	if err != nil {
		t.Fatalf("deriveNextStage() error = %v", err)
	}
	if !ok {
		t.Fatal("deriveNextStage() ok = false, want true")
	}
	if next.Slug != "second" {
		t.Fatalf("deriveNextStage() slug = %q, want second", next.Slug)
	}
}

func TestDeriveNextStageRejectsStateGraphMismatch(t *testing.T) {
	t.Parallel()

	current := parseAdvanceState(t, `- [x] first — EXECUTE
- [ ] skipped-by-state — SKIP
- [ ] second — EXECUTE
- [ ] phantom — EXECUTE`, "first", "phantom")
	_, _, err := deriveNextStage(current, loadAdvanceCatalog(t), "first")
	if !errors.Is(err, ErrStateCatalogMismatch) {
		t.Fatalf("deriveNextStage() error = %v, want ErrStateCatalogMismatch", err)
	}
}

func TestDeriveNextStageRejectsEarlierPendingExecute(t *testing.T) {
	t.Parallel()

	current := parseAdvanceStateWithTotal(t, `- [x] first — EXECUTE
- [ ] skipped-by-state — EXECUTE
- [x] second — EXECUTE
- [ ] third — EXECUTE`, "second", "third", 4)
	_, _, err := deriveNextStage(current, loadAdvanceCatalog(t), "second")
	if !errors.Is(err, ErrInvalidGate) {
		t.Fatalf("deriveNextStage() error = %v, want ErrInvalidGate", err)
	}
}

func TestDeriveNextStageRejectsUnknownScope(t *testing.T) {
	t.Parallel()

	current := parseAdvanceState(t, `- [x] first — EXECUTE
- [ ] skipped-by-state — SKIP
- [ ] second — EXECUTE
- [ ] third — EXECUTE`, "first", "second")
	_, _, err := deriveNextStage(current, graph.Snapshot{}, "first")
	if !errors.Is(err, ErrStateCatalogMismatch) {
		t.Fatalf("deriveNextStage() error = %v, want ErrStateCatalogMismatch", err)
	}
}

func parseAdvanceState(t *testing.T, rows, current, storedNext string) state.State {
	return parseAdvanceStateWithTotal(t, rows, current, storedNext, 3)
}

func parseAdvanceStateWithTotal(t *testing.T, rows, current, storedNext string, total int) state.State {
	t.Helper()
	content := `# AI-DLC State Tracking

## Project Information
- **Project**: sample
- **Project Type**: Brownfield
- **Scope**: classic
- **State Version**: 8
- **Active Agent**: agent-one

## Execution Plan Summary
- **Total Stages**: ` + strconv.Itoa(total) + `
- **Completed**: 1
- **In Progress**: first

## Phase Progress
- **Initialization**: Verified
- **Ideation**: Active
- **Inception**: Pending
- **Construction**: Pending
- **Operation**: Pending

## Stage Progress
` + rows + `

## Current Status
- **Lifecycle Phase**: IDEATION
- **Current Stage**: ` + current + `
- **Next Stage**: ` + storedNext + `
- **Status**: Running
- **Last Updated**: 2026-09-04T00:00:00Z

## Runtime State
- **Revision Count**: 0

## Session Resume Point
- **Last Completed Stage**: state-init
- **Next Action**: Execute first
`
	parsed, err := state.Parse([]byte(content))
	if err != nil {
		t.Fatalf("state.Parse() error = %v", err)
	}
	return parsed
}

func loadAdvanceCatalog(t *testing.T) graph.Snapshot {
	t.Helper()
	return loadGraphForAdvance(t, `[
  {"slug":"first","number":"1.1","name":"First","phase":"ideation","execution":"ALWAYS","lead_agent":"agent-one","support_agents":[],"mode":"inline","scopes":["classic"],"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"skipped-by-state","number":"1.2","name":"Skipped","phase":"ideation","execution":"ALWAYS","lead_agent":"agent-two","support_agents":[],"mode":"inline","scopes":["classic"],"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"second","number":"1.3","name":"Second","phase":"ideation","execution":"ALWAYS","lead_agent":"agent-three","support_agents":[],"mode":"inline","scopes":["classic"],"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"third","number":"1.4","name":"Third","phase":"ideation","execution":"ALWAYS","lead_agent":"agent-four","support_agents":[],"mode":"inline","scopes":["classic"],"produces":[],"consumes":[],"requires_stage":[]}
]`)
}

func loadGraphForAdvance(t *testing.T, stages string) graph.Snapshot {
	t.Helper()
	// Kept as a separate helper so routing tests can vary the fixed graph
	// without sharing mutable graph internals.
	catalog, err := graph.Load(fstest.MapFS{
		"stage-graph.json": {Data: []byte(stages)},
		"scope-grid.json":  {Data: []byte(`{"classic":{"stages":{"first":"EXECUTE","skipped-by-state":"EXECUTE","second":"EXECUTE","third":"EXECUTE"}}}`)},
	})
	if err != nil {
		t.Fatalf("graph.Load() error = %v", err)
	}
	return catalog
}
