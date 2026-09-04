---
name: go-tdd
description: "Execute one authorized Go TDD phase: write a failing test and return, or implement one parent-accepted RED and return. Require a plan, Issue, phase, and bounded file ownership; never advance phases autonomously."
---

# Go TDD

Execute one authorized phase as the sole writer, not an entire feature in one turn. Always load `$golang-how-to` and `$golang-testing`, then load only the task-specific Go skills routed by `$golang-how-to`.

First extract the explicit `tdd_phase` value from the current parent message for a `loop` request. Never infer RED from a new behavior, a test file list, or a plan describing both phases; never inherit a phase from an earlier turn. If absent, return `BLOCKED` without editing files or running tests.

For every `loop` handoff, read [the phase contract](../../../docs/tdd-handoff.md) completely. It is the canonical contract for both parent and implementer, including required inputs, snapshots, and return fields. Do not rely on remembered instructions from an earlier turn.

## Preconditions

Before editing, verify that the parent supplied:

- a self-contained plan with either direct user approval or a named approved roadmap／milestone that fully contains it;
- a GitHub Issue identifying the work;
- acceptance criteria and the owned files or packages.

For `loop`, also require exactly one `slice_id`, one observable `behavior`, `tdd_phase=red` or `green`, the workdir and starting HEAD, phase-specific file lists, and an exact targeted test command. A missing or ambiguous phase is a stop condition, not permission to run the full loop. `green` additionally requires the parent's explicit, fresh `red_acceptance` from the phase contract.

If any precondition is missing, the plan's authorization boundary is ambiguous, or an external Go module or tool is required without explicit approval, stop and return every missing gate in one response. Do not install `gotests`, `testify`, `goleak`, `golangci-lint`, or another tool merely because a referenced skill mentions it.

## Verification Mode

Read `verification_mode` from the parent handoff. Treat a missing mode as `loop`.

- `loop`: use for initial implementation, refactoring, and every review-finding fix. Run only the narrowest targeted tests needed for RED, GREEN, and affected behavior.
- `final`: use only when the parent explicitly says the approved implementation and blocking review fixes are stable. Run the approved full project gate once.

Reject `review` or an unknown mode as a role mismatch. Do not infer `final` from the end of a slice, a fix handoff,
or a request to return results.

## One Phase per Handoff

### RED

Write only the one requested behavior's test, run the exact targeted command, and return a final response. Do not implement the behavior, continue to GREEN, or start another slice. Production edits are forbidden except for a compile-only scaffold explicitly scoped by the parent before the task. Missing scaffolding authority is BLOCKED; a compile failure is not RED.

Return `RED_READY` only for the intended executed test's expectation mismatch. Return `ALREADY_GREEN` when it passes initially; never remove completed code or distort an assertion to manufacture RED. Return `BLOCKED` for missing authority, scope, environment, compile, skipped-test, or other unrelated failures.

### GREEN

Before editing, verify the parent's `red_acceptance` against the workdir, slice, HEAD, test command, and all target file hashes/ABSENT entries. Missing, incomplete, or stale acceptance is BLOCKED; never mint or refresh it yourself.
Compare every `red_acceptance.files` entry mechanically using the parent's expected hashes (for example, `shasum -a 256 -c`), including documents and pre-existing user changes. Inspecting printed hashes is not a comparison. Require exit code 0 for all entries before edits and return the check command/result; no harmless-change exception.

Implement only the accepted behavior within the implementation file list. Preserve the accepted test and command; if either needs changing, return to the parent. Run the test to GREEN, refactor only this scope while green, apply gofmt to implementation files, and rerun affected targeted tests. Return `GREEN_READY` or `BLOCKED` in a final response and stop. Never start the next slice.

Only the parent can issue a new phase after checking the diff and independently rerunning the targeted test. Commentary or a messaging tool is not the handoff boundary; finish the turn even when such a tool is unavailable.

Prefer the standard `testing` package, table-driven tests with named cases, deterministic fixtures, and assertions on observable behavior rather than implementation details.

## Loop Evidence

In `loop`, return the current phase's status, slice, command and exit code, observed assertion/result, changed files, HEAD and file hashes, and residual risks as specified by the phase contract. Do not report both phases unless the parent actually issued and accepted separate handoffs. Apply `gofmt` to permitted changed
Go files before returning and rerun the affected targeted tests afterward. Do not run `go test ./...`, any
race or vet command, full linters, cross compilation, or distribution E2E. A review-finding fix remains a loop even
when it is returned as a separate task.

## Final Evidence

Only in an explicitly delegated `final`, run the applicable approved checks with fresh output without modifying target files:

- `go test ./...`;
- `go test -race ./...` for concurrent or race-sensitive code, and as a final project gate when supported;
- `go vet ./...`;
- `gofmt -l` as a read-only format check;
- `git diff --check`.

Include plan-specific checks such as linters, cross compilation, or distribution E2E only in this final gate. Report
the final checks and residual risks. If any target file changes afterward, state that the evidence is stale and return
to `loop`; do not reuse the old result.

## Boundaries

- Do not expand beyond the authorized plan or edit unrelated user changes.
- Do not use additional writer agents or delegate nested implementation work.
- Leave independent review, commits, Issue updates, and PR operations to the parent unless they are explicitly delegated.
