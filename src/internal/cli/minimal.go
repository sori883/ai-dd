package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/sori883/ai-dd/src/internal/okfmemory"
)

// MinimalRequest is the strict public request shared with hook command recognition.
type MinimalRequest struct {
	Command, Action, Target, Space, ProjectDir, Session, File, Actor, Expect string
	IntentID                                                                 *string
	Reason, ResumeCondition, Stage                                           string
	Raw, Recover                                                             bool
}

func isMinimal(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "install", "unit", "memory", "session", "__minimal-hook":
		return true
	case "intent":
		if len(args) < 2 {
			return true
		}
		switch args[1] {
		case "create", "list", "show", "check", "switch", "configure", "review", "advance", "pause", "resume", "reopen", "wait", "cancel":
			return true
		}
		return false
	}
	return false
}
func runMinimal(args []string, stdout, stderr io.Writer, deps Dependencies) int {
	request, err := ParseMinimal(args)
	if err != nil {
		fmt.Fprintln(stderr, "aidlc:", err)
		return 2
	}
	if deps.Minimal == nil {
		fmt.Fprintln(stderr, "aidlc: minimal runtime unavailable")
		return 1
	}
	output, err := deps.Minimal(request)
	if len(output) > 0 {
		if _, writeErr := stdout.Write(output); writeErr != nil {
			fmt.Fprintln(stderr, writeErr)
			return 1
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "aidlc:", err)
		if errors.Is(err, fs.ErrInvalid) || errors.Is(err, fs.ErrExist) {
			return 2
		}
		return 1
	}
	return 0
}

// ParseMinimal also constrains the runtime hook's single-command exceptions.
func ParseMinimal(args []string) (r MinimalRequest, err error) {
	fail := func(message string) (MinimalRequest, error) { return r, fmt.Errorf("%s: %w", message, fs.ErrInvalid) }
	if len(args) < 1 {
		return fail("missing command")
	}
	r.Command = args[0]
	start := 2
	if r.Command == "__minimal-hook" {
		start = 1
	} else {
		if len(args) < 2 {
			return fail("missing action")
		}
		r.Action = args[1]
	}
	allowed := "--project-dir --space"
	required := ""
	min, max := 0, 0
	switch r.Command + "/" + r.Action {
	case "install/codex":
		allowed = "--project-dir"
		required = "--project-dir"
	case "intent/create":
		min, max = 1, 1
	case "intent/list":
	case "intent/switch":
		min, max = 0, 1
		allowed += " --session --id"
		required = "--session"
	case "memory/rules", "memory/check":
	case "memory/show", "intent/show", "intent/check":
		min, max = 1, 1
	case "intent/configure", "intent/review", "unit/claim", "unit/result", "unit/integrate", "unit/confirm":
		min, max = 1, 1
		allowed += " --expect --file"
		required = "--expect --file"
	case "intent/advance":
		min, max = 1, 1
		allowed += " --expect"
		required = "--expect"
	case "intent/pause", "intent/resume", "intent/reopen", "intent/wait", "intent/cancel":
		min, max = 1, 1
		allowed += " --expect --reason --stage --resume-condition"
		required = "--expect --reason"
	case "memory/search":
		min, max = 0, 1
		allowed += " --intent-id"
	case "memory/create":
		min, max = 1, 1
		allowed += " --file --actor"
		required = "--file --actor"
	case "memory/update":
		min, max = 1, 1
		allowed += " --file --actor --expect"
		required = "--file --actor --expect"
	case "session/bind":
		min, max = 1, 1
		allowed += " --session --recover"
		required = "--session"
	case "session/inspect":
		allowed = "--project-dir --session"
		required = "--session"
	case "__minimal-hook/":
		allowed = "--project-dir"
		required = "--project-dir"
	default:
		return fail("unknown minimal command")
	}
	values := map[string]string{}
	var positional []string
	for i := start; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		if !strings.Contains(" "+allowed+" ", " "+arg+" ") {
			return fail("unknown flag " + arg)
		}
		if _, ok := values[arg]; ok {
			return fail("duplicate flag " + arg)
		}
		if arg == "--raw" || arg == "--recover" {
			values[arg] = "true"
			continue
		}
		i++
		if i >= len(args) {
			return fail("missing value " + arg)
		}
		values[arg] = args[i]
	}
	if len(positional) < min || len(positional) > max {
		return fail("invalid positional arguments")
	}
	for _, flag := range strings.Fields(required) {
		if strings.TrimSpace(values[flag]) == "" {
			return fail("required flag " + flag)
		}
	}
	if strings.Contains(allowed, "--space") && values["--space"] == "" {
		return fail("--space is required")
	}
	if len(positional) == 1 {
		r.Target = positional[0]
	}
	r.Space = values["--space"]
	r.ProjectDir = values["--project-dir"]
	r.Session = values["--session"]
	r.File = values["--file"]
	r.Actor = values["--actor"]
	r.Expect = values["--expect"]
	r.Reason = values["--reason"]
	r.Stage = values["--stage"]
	r.ResumeCondition = values["--resume-condition"]
	r.Raw = values["--raw"] == "true"
	r.Recover = values["--recover"] == "true"
	if id, ok := values["--intent-id"]; ok {
		if !okfmemory.ValidID(id) {
			return fail("invalid --intent-id")
		}
		r.IntentID = &id
	}
	if r.Command == "intent" && r.Action == "switch" {
		id, hasID := values["--id"]
		if hasID {
			if r.Target != "" || !okfmemory.ValidID(id) {
				return fail("use one name or valid --id")
			}
			r.IntentID = &id
		} else if r.Target == "" {
			return fail("Intent name is required")
		}
	}
	return r, nil
}
