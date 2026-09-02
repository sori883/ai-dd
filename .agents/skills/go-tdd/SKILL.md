---
name: go-tdd
description: "Implement approved Go changes through observable Red-Green-Refactor slices and fresh verification evidence. Use only when an approved plan and GitHub Issue exist; stop before adding unapproved dependencies or expanding scope."
---

# Go TDD

Implement one approved Go change as the sole writer. Always load `$golang-how-to` and `$golang-testing`, then load any task-specific Go skills routed by `$golang-how-to`.

## Preconditions

Before editing, verify that the parent supplied:

- an explicitly approved plan;
- a GitHub Issue identifying the work;
- acceptance criteria and the owned files or packages.

If any precondition is missing, or an external Go module or tool is required without explicit approval, stop and return every missing gate in one response. Do not install `gotests`, `testify`, `goleak`, `golangci-lint`, or another tool merely because a referenced skill mentions it.

## Verification Mode

Read `verification_mode` from the parent handoff. Treat a missing mode as `loop`.

- `loop`: use for initial implementation, refactoring, and every review-finding fix. Run only the narrowest targeted tests needed for RED, GREEN, and affected behavior.
- `final`: use only when the parent explicitly says the approved implementation and blocking review fixes are stable. Run the approved full project gate once.

Reject `review` or an unknown mode as a role mismatch. Do not infer `final` from the end of a slice, a fix handoff,
or a request to return results.

## Red-Green-Refactor Loop

1. Record the current targeted-test baseline and create a test list from the acceptance criteria.
2. Select exactly one observable behavior and write one runnable test through the agreed public seam.
3. Run the narrowest relevant `go test` command and observe RED. Confirm it fails for the intended missing or incorrect behavior, not for an unrelated compile, fixture, or environment problem.
4. Write the minimum production code needed to make that test and all earlier tests pass.
5. Run the targeted test and observe GREEN.
6. Refactor only while tests are green. Do not combine unrelated cleanup with the behavior change.
7. Repeat until the test list is empty.

Prefer the standard `testing` package, table-driven tests with named cases, deterministic fixtures, and assertions on observable behavior rather than implementation details.

## Loop Evidence

In `loop`, report fresh RED and GREEN commands, changed files, refactors, and residual risks. Apply `gofmt` to changed
Go files before the review handoff and rerun the affected targeted tests afterward. Do not run `go test ./...`, any
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

- Do not expand beyond the approved plan or edit unrelated user changes.
- Do not use additional writer agents or delegate nested implementation work.
- Leave independent review, commits, Issue updates, and PR operations to the parent unless they are explicitly delegated.
