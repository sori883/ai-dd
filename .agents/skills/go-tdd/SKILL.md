---
name: go-tdd
description: "Execute an authorized Go work unit as the sole writer through ordered, evidence-backed Red-Green-Refactor cycles, then return once at the work-unit boundary."
---

# Go TDD

Implement one authorized Go work unit as the sole writer. A work unit may contain multiple ordered behaviors. Complete each behavior test-first, record its RED and GREEN evidence, and return to the parent once after the whole work unit or immediately when genuinely blocked.

Default to one implementation work unit for all approved behaviors in one Issue／PR. Do not split merely because there are many slices. Split only for a distinct writer boundary, unresolved authorization or design gate, or a workload that cannot be handled safely in one bounded turn, and require the reason and boundary in the plan.

Always load `$golang-how-to` and `$golang-testing`, then load only the task-specific Go skills routed by `$golang-how-to`.

For every `loop` handoff, read [the work-unit contract](../../../docs/tdd-handoff.md) completely. It is the canonical contract for the parent and implementer. Do not require or infer the retired per-phase `tdd_phase` or `red_acceptance` protocol.

## Preconditions

Before editing, require:

- a self-contained plan authorized by direct user approval or a named approved roadmap／milestone;
- a GitHub Issue, acceptance criteria, workdir, starting HEAD, and bounded file or package ownership;
- one `work_unit_id` and an ordered behavior list containing each behavior's `slice_id`, observable result, test files, implementation files, and exact targeted test command;
- explicit compile-only scaffold authority for a new API when a runnable assertion cannot otherwise be reached.

If a gate is missing, the work exceeds the plan, another writer is active, or an external Go module or tool lacks approval, stop before editing and return all blockers together.

## Verification Mode

Read `verification_mode`; a missing value means `loop`.

- `loop`: implement or repair the authorized work unit. Run only its targeted tests and affected package checks.
- `final`: only when the parent explicitly says the implementation and blocking review fixes are stable. Run the approved full read-only gate once and do not edit target files.

Reject `review` or an unknown mode as a role mismatch.

## Continuous Work-Unit Loop

For each behavior in the supplied order:

1. Add or change that behavior's test before changing its production implementation. A preauthorized compile-only scaffold may declare the planned API but must not implement the behavior.
2. Format the test and run its exact targeted command. A valid RED is a compiled, executed test that fails for the intended observable expectation. Compile errors, environment failures, skipped tests, and `no tests to run` are blockers, not RED.
3. If the test is already green, record `ALREADY_GREEN`; do not delete correct code or distort the assertion. Otherwise record the RED command, exit code, test, and failure reason.
4. Implement only the smallest change for that behavior. Do not implement later behaviors ahead of their tests.
5. Run the same command to GREEN, refactor only while green, apply `gofmt` to permitted Go files, and rerun the affected targeted tests.
6. Continue directly to the next behavior without returning to the parent.

If a test expectation is wrong, a new material decision is required, ownership must expand, or an unrelated failure prevents safe progress, stop the work unit. Preserve completed valid cycles and return their evidence with the blocker; never weaken a test to keep moving.

After all behaviors, update authorized documentation as one batch, run every work-unit targeted command once more plus the permitted affected-package checks and `git diff --check`, then return `WORK_UNIT_READY`. Do not run full-project tests, race, vet, cross compilation, or distribution E2E in `loop`.

## Review-Finding Repair

The parent should group all current blocking findings into one repair work unit. For each finding that changes observable Go behavior, add and run a failing regression test before the fix. Documentation, configuration, and an `ALREADY_GREEN` coverage gap do not require an artificial RED. Complete the grouped fixes, rerun their targeted tests, and return once for one re-review.

## Evidence

The final loop response must include:

- status: `WORK_UNIT_READY` or `BLOCKED`;
- work unit, completed and remaining slice IDs;
- for every slice: RED or `ALREADY_GREEN` result, exact command and exit code, then GREEN command and exit code when implementation changed;
- changed files, starting and ending HEAD, final file hashes, formatting and `git diff --check` results;
- affected targeted/package commands rerun at the work-unit boundary;
- residual risks or the exact blocker.

The parent reviews the complete diff and reruns the work unit's targeted command set once. The parent does not repeat every intermediate RED/GREEN transition. Historical RED evidence comes from the implementer's executed command transcript; do not claim test-first work that was not actually observed.

## Final Evidence

Only in an explicitly delegated `final`, run the applicable approved checks with fresh output and without modifying target files:

- `go test ./...`;
- `go test -race ./...` when supported or required;
- `go vet ./...`;
- `gofmt -l` as a read-only format check;
- `git diff --check`;
- plan-specific lint, cross compilation, or distribution E2E checks.

If a target file changes afterward, report the evidence as stale and return to `loop`.

## Boundaries

- Do not expand beyond the authorized plan or overwrite unrelated user changes.
- Do not use additional writer agents or delegate nested implementation work.
- Leave independent review, commits, Issue updates, and PR operations to the parent unless explicitly delegated.
