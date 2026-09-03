---
name: implementation-planning
description: "Create authorization-ready implementation plans for repository changes, including scope, file ownership, TDD strategy, risks, validation, and whether direct, roadmap, or milestone approval permits execution. Use before implementation; do not edit code or operate Issues or PRs."
---

# Implementation Planning

Turn a requested change and verified repository context into a plan whose implementation permission can be decided without guessing at missing decisions.

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
8. Before returning the plan, reread it as a first-time participant. Remove unfamiliar terms that are not needed for the decision, and explain every necessary unfamiliar term on first use. Replace context-dependent phrases that lack an antecedent in the plan, such as `上記`, `先ほど`, `この方針`, or `検証済みの入力`, with the concrete fact or name. Confirm that no important safety, compatibility, migration, usage-condition, or approval-boundary information is missing. If a public observable behavior needs contract details that the evidence does not establish, ask for them as an unresolved decision instead of guessing or claiming that no unresolved decisions remain.
9. Determine the authorization source:
   - If the whole plan fits an approved roadmap or milestone, identify that comprehensive authorization and state why the plan is inside its scope.
   - Otherwise end at an explicit user-approval gate that states what approval authorizes, which important boundaries remain fixed, and which unresolved choices still require a later decision.
   - Never use comprehensive authorization when upstream evidence is ambiguous or conflicting, a new intentional upstream difference is proposed, the plan exceeds the approved scope, or a material public API, persisted-data, compatibility, migration, security, permission, operational, external-dependency, paid-service, credential, or irreversible-action choice remains unresolved.
10. Do not edit files, create Issues, create PRs, or start implementation. A read-only planner reports either a resolvable authorization source or the approval it still needs.

## Output Contract

Return a concise, self-contained plan that distinguishes confirmed decisions from questions. A first-time reader should be able to understand the need, expected result, proposed change, acceptance evidence, exact boundary, and why implementation is already authorized or still needs approval. Include enough evidence for the user or parent agent to approve, revise, or execute it, but do not include speculative implementation details that repository inspection did not establish.
