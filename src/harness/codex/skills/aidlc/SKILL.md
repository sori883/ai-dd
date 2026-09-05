---
name: aidlc
description: AI-DLC Codex receiver for safely reading delivered workflow context.
---

# AI-DLC context receiver

This skill is a read-only receiver. It handles the directive returned by the
`aidlc` binary and never performs stage execution, creates outputs, or asks for
authorization.

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

For an ordinary invocation, when every context chunk has been received,
return exactly `context ready` and stop. If and only if the caller explicitly
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

## Safe failure behavior

An `error` directive is terminal: show its message and stop. An unknown directive
kind, malformed directive, unknown version, nonzero read-context command, or
read failure is a fail closed condition. Do not skip a missing
chunk, guess a token, or take another workflow action after failure. Stage
execution and reporting are outside this receiver's contract.
