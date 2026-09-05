---
name: aidlc
description: AI-DLC Codex receiver for safely reading delivered workflow context.
---

# AI-DLC context receiver

This skill is a read-only receiver. It processes the directive returned by the
`aidlc` binary and reads only the context named by that directive. It does not
perform stage execution, create deliverables, or ask for human authorization.

Start a fresh delivery with:

```text
aidlc next --project-dir .
```

## Directive sequence

### `load-steering`

For a `load-steering` directive, consume every entry in `rules_content` in its
declared array order. Preserve each entry's path and text; do not merge entries
in a way that changes their order. If the directive has a `context_warnings`
field, show the warnings without omitting any rules.

After all rule entries have been read successfully, immediately run:

```text
aidlc continue "<opaque token>" --project-dir .
```

Replace `"<opaque token>"` with the directive's `continue_token` as one opaque
argument, exactly as received. Do not emit a progress or report message before
continuing, and do not invent a token or retry a different command.

### `run-stage`

For a `run-stage` directive, display every `context_warnings` value, but do not
skip any read because of a warning. First read all files listed by
`inline_context_paths`, in their listed order, and wait for every full file
read to finish. Next read the complete `stage_file`. Finally read every
existing path in `consumes`, in the declared order. A declared path is consumed
only after its complete UTF-8 content has been read; existence or directory
listing is not a read receipt.

For an ordinary invocation, when and only when all of those reads succeed,
return exactly `context ready` and stop. If and only if the caller explicitly
supplies a machine-readable read receipt request together with an output schema
for verification, return only the schema-conforming receipt requested by that
schema and stop. This is a verification-only exception, not permission for a
general context dump or ordinary context output. In either case, do not run the
stage, do not create outputs, and do not send any additional progress message or
claim a stage result. This receiver never advances into stage execution,
deliverable creation, review,
sensing, reporting, or an approval gate.

## Safe failure behavior

An `error` directive is terminal: show its message and stop. An `unknown directive`
kind, malformed directive, unknown version, or read failure is a `fail closed`
condition. Do not skip a missing or unreadable file, do not run a stage on
partial context, and stop without guessing a path, token, or next
action.

A shell tool may be used only for the aidlc commands above and for direct
reads of each explicitly declared path. Do not use a shell for search, glob, or
directory listing, and do not guess replacement context. Resolve relative paths
from the project directory, reject paths that escape it, and read in full every
regular file. Keep the declared ordering and preserve the exact opaque
continuation token until it is passed to the binary.
