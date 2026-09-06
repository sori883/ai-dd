---
name: aidlc
description: AI-DLC Codex receiver for safely reading delivered workflow context.
---

# AI-DLC context receiver

This skill receives the directive returned by the `aidlc` binary. The explicit
intent-capture execution contract below is the only inline-conductor exception.

Start a fresh delivery with:

```text
aidlc next --project-dir .
```

## Directive sequence

### `load-steering`

Consume every entry in `rules_content` in its declared array order. Preserve
each entry's path and text and do not merge entries in a way that changes their
order. If `context_warnings` is present, retain the warnings without dropping
any rule.

After all rule entries have been received successfully, immediately run:

```text
aidlc continue "<opaque token>" --project-dir .
```

Pass `continue_token` as one opaque argument exactly as received. Do not
invent a token or retry a different command.

When the response is another `load-steering` directive, append its ordered
`rules_content` entries to the active bundle and repeat this section. Continue
until the next directive is `run-stage`.

### `run-stage`

For a `run-stage` directive, retain every `context_warnings` value. Its
context declaration is ordered as `inline_context_paths`, then `stage_file`,
then `consumes`; retain that order and do not infer any additional input.

Once the directive has been received, read the delivered context only through:

```text
aidlc read-context --project-dir .
```

Each successful response is one bounded context chunk. If it contains a
`read_continue_token`, pass that value unchanged to:

```text
aidlc read-context continue "<opaque read token>" --project-dir .
```

Repeat this command until the response contains `complete:true`. Preserve the
chunk order and stop on any error, malformed response, or missing continuation
token. Do not choose a path, slot, part, or replacement input yourself.

Ordinary run-stage directives remain read-only context handoffs.

For an ordinary invocation of `run-stage`, when every context chunk has been
received, return exactly `context ready` and stop. If and only if the caller explicitly
supplies a machine-readable read receipt request together with an output
schema for verification, return only the schema-conforming receipt requested by
that schema and stop. This is a verification-only exception, not permission
for a general context dump. In either case, do not run the stage, do not create
outputs, do not send any additional progress message, and do not claim a stage
result. This receiver never advances into deliverable creation, review,
sensing, reporting, or an approval gate.

The verification-only receipt uses the supplied schema with these fixed
meanings: `rules` contains the last non-empty line of each received
`rules_content` entry, in received order; `inline_context` contains each inline
file's full text after concatenating its chunks; `stage_file` contains the full
text after concatenating its chunks; and `consumes` contains each consume file's
full text after concatenating its chunks. For every file field, concatenate
chunks in `slot/index/part` order and do not omit empty or trailing text.

If the supplied verification schema requires `files`, `files` contains one
compact proof for each delivered file in `slot/index` order. Each `files` entry has these
fields in this order: `slot`, `index`, `parts`, `content_sha256`,
`first_non_empty_line`, `middle_marker_line`, `last_non_empty_line`. `parts` is
the total number of context chunks for that file. `content_sha256` is the
SHA-256 digest of the concatenated chunk text. `first_non_empty_line` is the
first line whose trimmed text is non-empty. `middle_marker_line` is the first
line beginning with `MIDDLE-`. `last_non_empty_line` is the final line whose
trimmed text is non-empty. The legacy `inline_context`, `stage_file`, and
`consumes` fields retain their full-text meanings when the supplied schema
requires those fields; a compact schema does not require those legacy fields.

## Safe failure behavior

An `error` directive is terminal: show its message and stop. An unknown directive
kind, malformed directive, unknown version, nonzero read-context command, or
read failure is a fail closed condition. Do not skip a missing
chunk, guess a token, or take another workflow action after failure. Stage
execution and reporting remain outside the ordinary run-stage receiver's contract.

## Intent-capture execution contract

When the surrounding conductor selects the `intent-capture` stage, keep its
inline context in this order: inline persona/knowledge, base stage protocol,
reviewer/ensemble protocol, question-rendering annex, authoritative
`project-description.json`, stage file, then consumes. The architect is
inline (`architect: inline`); there is no architect subagent and no
contribution file. The only reviewer is `product-lead`, with an advisory
review class and one effective iteration.

Architect routing is inline: no architect subagent; no contribution file.

For each ordinary question, preserve one `DECISION_RECORDED`, one fresh HUMAN_TURN
and one `QUESTION_ANSWERED` boundary. The consolidated summary
uses a separate decision and fresh `HUMAN_TURN`, then exact `Looks correct`
before `SUMMARY_CONFIRMATION_RECORDED`. Reviewer boundaries are
`REVIEW_REQUESTED` followed by `REVIEW_COMPLETED`. At the gate, attempt every
advisory sensor and retain `SENSOR_FIRED` observations and any terminal result;
sensor output is presentation evidence only. After sensing, ask the mandatory
learnings question on another fresh `HUMAN_TURN` and persist only selected
learnings as `RULE_LEARNED` or `SENSOR_PROPOSED`. Surface the visible learning
candidates and parked Open questions to the user, and always offer `Anything to add for next time?`
with `Nothing to add` or `Add a note`. Arbitrary unselected diary entries are
never persisted. A selected sensor is scaffolded into the project manifest and
stage binding, then becomes eligible to fire from the next compile.

The hidden receiver bridge is invoked only by the configured PATH `aidlc`
binary and is not listed by `aidlc help`. Its fixed action sequence is:

Every JSON payload is supplied on stdin; the action command has no positional
JSON argument. Use the following command grammar (the placeholders are
ordinary action data only):

```text
printf '%s' '<JSON decision data>' | aidlc __codex-stage decision --project-dir .
UserPromptSubmit -> aidlc __codex-user-prompt-submit
printf '%s' '<JSON answer data>' | aidlc __codex-stage answer --project-dir .
aidlc __codex-stage summary --project-dir .
UserPromptSubmit -> aidlc __codex-user-prompt-submit
printf '%s' '<JSON summary answer data>' | aidlc __codex-stage answer --project-dir .
write the three intent-capture artifacts
aidlc __codex-stage review-request --project-dir .
dispatch configured `aidlc-product-lead-agent` with a brief containing all three declared artifacts,
the backend-derived iteration, and any prior findings
configured reviewer appends canonical `## Review` appendix
printf '%s' '<JSON verdict data>' | aidlc __codex-stage review-complete --project-dir .
aidlc __codex-stage run-sensors --project-dir .
aidlc __codex-stage learnings-surface --project-dir .
UserPromptSubmit -> aidlc __codex-user-prompt-submit
printf '%s' '{"selections":[]}' | aidlc __codex-stage learnings-persist --project-dir .
aidlc report --stage intent-capture --result awaiting-approval
UserPromptSubmit -> aidlc __codex-user-prompt-submit
aidlc report --stage intent-capture --result approved --user-input Approve
aidlc next
```

The configured product-lead reviewer owns the appendix: the conductor does not
append or mint the review. On revision recovery, write the backend-derived
iteration returned by `review-request` (for example, iteration 2) into the
canonical appendix; never assume the normal advisory budget's iteration 1.
When the bridge returns a challenge, include exactly one `**Request Challenge:** review:<32 lowercase hex>` line in the new appendix.
If the approval response is `Request Changes`, use
the public rejected report grammar. Keep valid question, summary, and learnings
evidence; only reconfirm the summary when its confirmed content changed. Revise
the artifacts, then request a fresh review, run the advisory sensors, and
record `revised`; old review receipts are never reused:

```text
aidlc report --stage intent-capture --result rejected --user-input "Request Changes" --reason "<feedback>"
...if confirmed content changed, repeat the summary decision/turn/answer...
...revise artifacts, request/complete review, and run all three sensors...
...do not surface or persist learnings again...
aidlc report --stage intent-capture --result revised
```

The bridge accepts only ordinary action data. Timestamps, hashes, fire IDs,
receipt booleans, artifact snapshots, reviewer identity, and iteration are
derived from the active identity, graph, roots, and audit ledger. A sensor
failure or missing terminal is shown as an advisory observation and does not
decide the gate. At approval, accept only the exact `Approve` or `Request
Changes` choice. The learnings question is always a separate fresh turn before
the first gate; after `Request Changes`, keep its valid decision/answer and do
not surface or persist learnings again. Revise only the changed summary when
its confirmed content is stale, request a new review, preserve the prior
findings while removing the old terminal appendix, include the returned
`review:<32 lowercase hex>` challenge in the new canonical appendix, complete
that review, and run the three advisory sensors once for the new review cycle.
An advisory sensor failure or missing terminal does not decide the gate.
