---
name: go-code-review
description: "Review a bounded Go diff against its approved plan, Issue, repository rules, correctness, and test evidence. Use for independent read-only review; report prioritized actionable findings and never modify the working tree."
---

# Go Code Review

Review as an independent owner, not as the implementer. Always load `$golang-how-to`, then select relevant Go review skills for the diff, such as `$golang-testing`, `$golang-error-handling`, `$golang-safety`, `$golang-concurrency`, `$golang-context`, `$golang-security`, `$golang-code-style`, or `$golang-naming`.

## Inputs

Require a resolvable base and head, the approved plan, acceptance criteria, and linked Issue. Pin the comparison before reviewing. If the boundary is missing or the diff is empty, return that problem instead of guessing.

## Review Method

1. Read the applicable `AGENTS.md`, plan, Issue, changed files, and enough surrounding code to understand behavior.
2. Review the diff before relying on the implementer's self-assessment.
3. Prioritize correctness, regressions, data loss, security, error handling, cancellation, goroutine lifecycle, races, API compatibility, and test validity.
4. Check that changed observable behavior has a regression test and that the test would fail against the broken or pre-change behavior. Tests do not validate themselves.
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

## Boundaries

Remain read-only. Do not edit files, commit, post PR comments, or spawn nested review agents. Return findings to the parent, which decides whether to send them back to the implementer.
