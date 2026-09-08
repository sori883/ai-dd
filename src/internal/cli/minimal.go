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
	BodyFile                                                                 string
	Metadata                                                                 okfmemory.MetadataInput
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
		hint := "aidlc --help"
		if len(args) >= 2 {
			if _, ok := Help([]string{args[0], args[1], "--help"}); ok {
				hint = "aidlc " + args[0] + " " + args[1] + " --help"
			}
		}
		fmt.Fprintln(stderr, "使い方:", hint)
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
		allowed += " --body-file --actor --type --title --description --tag --status --intent-id --resource --stale-after --sources-json --verified-json --metadata-json"
		required = "--body-file --actor --type --title --description"
	case "memory/update":
		min, max = 1, 1
		allowed += " --body-file --actor --expect --type --title --description --tag --clear-tags --status --intent-id --resource --stale-after --sources-json --verified-json --metadata-json"
		required = "--body-file --actor --expect"
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
		if arg == "--file" && r.Command == "memory" && (r.Action == "create" || r.Action == "update") {
			return fail("use --body-file for body-only Markdown; see aidlc memory " + r.Action + " --help")
		}
		if !strings.Contains(" "+allowed+" ", " "+arg+" ") {
			return fail("unknown flag " + arg)
		}
		if _, ok := values[arg]; ok && arg != "--tag" {
			return fail("duplicate flag " + arg)
		}
		if arg == "--raw" || arg == "--recover" || arg == "--clear-tags" {
			values[arg] = "true"
			continue
		}
		i++
		if i >= len(args) || strings.HasPrefix(args[i], "--") {
			return fail("missing value " + arg)
		}
		values[arg] = args[i]
		if arg == "--tag" {
			if strings.TrimSpace(args[i]) == "" {
				return fail("empty tag")
			}
			r.Metadata.Tags = append(r.Metadata.Tags, args[i])
		}
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
	r.BodyFile = values["--body-file"]
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
	if r.Command == "memory" && (r.Action == "create" || r.Action == "update") {
		r.Metadata.Actor = r.Actor
		r.Metadata.IntentID = r.IntentID
		r.Metadata.ClearTags = values["--clear-tags"] == "true"
		if r.Metadata.ClearTags && r.Metadata.Tags != nil {
			return fail("--tag and --clear-tags conflict")
		}
		for flag, dest := range map[string]**string{"--type": &r.Metadata.Type, "--title": &r.Metadata.Title, "--description": &r.Metadata.Description, "--status": &r.Metadata.Status, "--resource": &r.Metadata.Resource, "--stale-after": &r.Metadata.StaleAfter, "--sources-json": &r.Metadata.SourcesJSON, "--verified-json": &r.Metadata.VerifiedJSON, "--metadata-json": &r.Metadata.ExtraJSON} {
			if value, ok := values[flag]; ok {
				if strings.TrimSpace(value) == "" && flag != "--resource" {
					return fail("empty " + flag)
				}
				*dest = &value
			}
		}
		if status := r.Metadata.Status; status != nil && *status != "draft" && *status != "stable" && *status != "deprecated" {
			return fail("--status must be draft, stable or deprecated")
		}
	}
	return r, nil
}
