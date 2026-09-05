---
name: go-code-review
description: "Review a bounded Go diff against its authorized plan, Issue, repository rules, correctness, and test evidence. Use for independent read-only review; report prioritized actionable findings and never modify the working tree."
---

# Go Code Review

Review as an independent owner, not as the implementer. Always load `$golang-how-to`, then select relevant Go review skills for the diff, such as `$golang-testing`, `$golang-error-handling`, `$golang-safety`, `$golang-concurrency`, `$golang-context`, `$golang-security`, `$golang-code-style`, or `$golang-naming`.

## Inputs

Require a resolvable base and head, the authorized plan, its direct-approval or comprehensive-authorization source, acceptance criteria, and linked Issue. Pin the comparison before reviewing. If the boundary is missing or the diff is empty, return that problem instead of guessing.

## Review Method

1. Read the applicable `AGENTS.md`, plan, Issue, changed files, and enough surrounding code to understand behavior.
2. Review the diff before relying on the implementer's self-assessment.
3. Prioritize correctness, regressions, data loss, security, error handling, cancellation, goroutine lifecycle, races, API compatibility, and test validity.
4. Check that changed observable behavior has a regression test and that the test would fail against the broken or pre-change behavior. Tests do not validate themselves.
   For Go changes using the work-unit handoff, read [the work-unit contract](../../../docs/tdd-handoff.md). Verify the per-slice executed RED/ALREADY_GREEN and GREEN evidence, test-before-production ordering, bounded ownership, the parent's single boundary rerun, and unchanged meaningful tests. Do not accept manufactured RED or claim missing historical test-first evidence after the fact. Distinguish an observable test gap from unavailable execution history and report either without inventing a history.
5. Run safe targeted tests or diagnostics when they materially confirm a finding. Distinguish verified failures from reasoned risks.
6. Ignore personal style preferences unless they obscure a defect or violate an applicable repository rule.

## Finding Format

Lead with findings ordered by severity:

- `P0`: release-blocking safety, security, or data-loss defect;
- `P1`: likely correctness or compatibility failure;
- `P2`: meaningful maintainability or test gap with concrete impact;
- `P3`: non-blocking improvement.

Each finding must include a concise title, exact file and line, triggering condition, impact, evidence or reproduction, and the smallest safe correction. Avoid duplicates and speculative findings.

If there are no blocking findings, say so explicitly and list residual risks or checks not performed.

## Verification Cadence

Require an explicit `verification_mode=review`. A missing mode resolves to `loop`, which is a role mismatch; reject it
instead of silently promoting it to `review`. Reject `loop`, `final`, and unknown modes for the same reason.

Run only safe targeted tests or diagnostics that materially reproduce a finding or confirm its resolution. Do not run
full-project tests, any race or vet command, full linters, cross compilation, or distribution E2E during review or
re-review. Those checks belong to the parent's `final` gate after the diff has no blocking findings.

When no blocking findings remain, report that the fixed head is ready for `final`. Do not treat a re-review as a
request to repeat the final gate.

## Boundaries

Remain read-only. Do not edit files, commit, post PR comments, or spawn nested review agents. Return findings to the parent, which decides whether to send them back to the implementer.
