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

## Red-Green-Refactor Loop

1. Record the current targeted-test baseline and create a test list from the acceptance criteria.
2. Select exactly one observable behavior and write one runnable test through the agreed public seam.
3. Run the narrowest relevant `go test` command and observe RED. Confirm it fails for the intended missing or incorrect behavior, not for an unrelated compile, fixture, or environment problem.
4. Write the minimum production code needed to make that test and all earlier tests pass.
5. Run the targeted test and observe GREEN.
6. Refactor only while tests are green. Do not combine unrelated cleanup with the behavior change.
7. Repeat until the test list is empty.

Prefer the standard `testing` package, table-driven tests with named cases, deterministic fixtures, and assertions on observable behavior rather than implementation details.

## Completion Evidence

Format changed Go files and run the applicable checks with fresh output:

- targeted package tests during each loop;
- `go test ./...`;
- `go test -race ./...` for concurrent or race-sensitive code, and as a final project gate when supported;
- `go vet ./...`;
- `git diff --check`.

Report the RED and GREEN commands, refactors performed, changed files, final checks, and residual risks. Do not claim success from stale output.

## Boundaries

- Do not expand beyond the approved plan or edit unrelated user changes.
- Do not use additional writer agents or delegate nested implementation work.
- Leave independent review, commits, Issue updates, and PR operations to the parent unless they are explicitly delegated.
