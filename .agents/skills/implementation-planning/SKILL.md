---
name: implementation-planning
description: "Create approval-ready implementation plans for repository changes, including scope, file ownership, TDD strategy, risks, and validation. Use before implementation; do not use to edit code or create Issues or PRs."
---

# Implementation Planning

Turn a requested change and verified repository context into a plan the user can approve without guessing at missing decisions.

## Workflow

1. Read the applicable `AGENTS.md`, canonical specifications, current code, tests, and any technical-research report.
2. Identify material ambiguities. Return focused questions when an answer would change behavior, public interfaces, dependencies, or delivery scope.
3. Inspect the actual repository structure and name concrete target files or packages. For Go work, use `$golang-how-to` and semantic tooling when available.
4. Begin with a plain-language summary of the current problem, why the change is needed, what result the user will get, and who or what is affected. Preserve formal technical terms and code identifiers, but explain each unfamiliar term's role on first use.
5. Make the plan understandable without prior conversation or opening linked material. Use links as supporting evidence, not as a replacement for the facts and decisions needed to approve the plan.
6. State separately:
   - goal and acceptance criteria;
   - scope, plus only those non-goals needed to prevent a material misunderstanding about safety, compatibility, migration, usage conditions, or the approval boundary;
   - verified facts, assumptions, and unresolved decisions;
   - design and alternatives considered;
   - files or packages to add or change, with one writer per owned area;
   - ordered TDD slices and the observable seam for each slice;
   - targeted and final verification commands;
   - dependency impact, risks, rollback, and documentation updates.
7. Call out every proposed external Go module with its necessity and why the standard library is insufficient.
8. End at an explicit approval gate that states what the approval authorizes, which important boundaries remain fixed, and which unresolved choices still require a later decision. Do not edit files, create Issues, create PRs, or start implementation.

## Output Contract

Return a concise, self-contained plan that distinguishes confirmed decisions from questions. A first-time reader should be able to understand the need, expected result, proposed change, acceptance evidence, and exact approval boundary from the plan alone. Include enough evidence for the user to approve or revise it, but do not include speculative implementation details that repository inspection did not establish.
