package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/sori883/ai-dd/src/internal/okfmemory"
)

// CommandRequest is the strict public request shared with hook command recognition.
type CommandRequest struct {
	Unit, Root                                                               string
	Step                                                                     string
	Relocate                                                                 bool
	FromProjectDir, FromBinary                                               string
	BodyFile                                                                 string
	Metadata                                                                 okfmemory.MetadataInput
	Command, Action, Target, Space, ProjectDir, Session, File, Actor, Expect string
	IntentID                                                                 *string
	Reason, ResumeCondition, Stage, Boundary                                 string
	Raw, Recover                                                             bool
}

func isServiceCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "assignment", "install", "unit", "memory", "session", "__hook":
		return true
	case "intent":
		if len(args) < 2 {
			return true
		}
		switch args[1] {
		case "hash", "plan", "plan-approval", "approval", "finish", "history", "create", "list", "show", "documents", "procedure", "check", "begin", "switch", "configure", "review", "advance", "pause", "resume", "reopen", "wait", "cancel":
			return true
		}
		return false
	}
	return false
}
func runServiceCommand(args []string, stdout, stderr io.Writer, deps Dependencies) int {
	request, err := ParseCommand(args)
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
	if deps.Execute == nil {
		fmt.Fprintln(stderr, "aidlc: command runtime unavailable")
		return 1
	}
	output, err := deps.Execute(request)
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

// ParseCommand also constrains the runtime hook's single-command exceptions.
func ParseCommand(args []string) (r CommandRequest, err error) {
	fail := func(message string) (CommandRequest, error) { return r, fmt.Errorf("%s: %w", message, fs.ErrInvalid) }
	if len(args) < 1 {
		return fail("missing command")
	}
	r.Command = args[0]
	start := 2
	if r.Command == "__hook" {
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
	case "assignment/init", "assignment/reset":
		allowed = "--project-dir --file"
		required = "--file"
	case "assignment/list":
		allowed = "--project-dir"
	case "assignment/show", "assignment/check":
		allowed = "--project-dir"
		min, max = 1, 1
	case "assignment/reserve":
		allowed += " --session --expect --file"
		required = "--session --expect --file"
		min, max = 1, 1
	case "assignment/release":
		allowed = "--project-dir --session --expect --file"
		required = "--session --expect --file"
		min, max = 1, 1

	case "install/codex":
		allowed = "--project-dir --relocate --from-project-dir --from-binary"
	case "intent/hash":
		min, max = 1, 1
		allowed += " --unit --root"
	case "intent/create":
		min, max = 1, 1
	case "intent/list":
	case "intent/switch":
		min, max = 0, 1
		allowed += " --session --id"
		required = "--session"
	case "memory/rules", "memory/check":
	case "intent/check":
		min, max = 1, 1
		allowed += " --boundary"
	case "memory/show", "intent/show", "intent/procedure", "intent/history":
		min, max = 1, 1
	case "intent/documents", "intent/plan":
		min, max = 1, 1
		allowed += " --expect --file"
	case "intent/plan-approval", "intent/approval", "intent/configure", "intent/review", "unit/claim", "unit/result", "unit/integrate", "unit/confirm", "unit/reassign":
		min, max = 1, 1
		allowed += " --expect --file"
		required = "--expect --file"
	case "intent/finish", "intent/advance", "intent/begin":
		min, max = 1, 1
		allowed += " --expect"
		required = "--expect"
	case "intent/reopen":
		min, max = 1, 1
		allowed += " --expect --reason --step"
		required = "--expect --reason --step"
	case "intent/pause", "intent/resume", "intent/wait", "intent/cancel":
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
	case "__hook/":
		allowed = "--project-dir"
		required = "--project-dir"
	default:
		return fail("unknown command")
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
		if arg == "--raw" || arg == "--recover" || arg == "--clear-tags" || arg == "--relocate" {
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
	r.Unit, r.Root = values["--unit"], values["--root"]
	if r.Root != "" && r.Unit == "" {
		return fail("--root requires --unit")
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
	r.Step = values["--step"]
	r.Boundary = values["--boundary"]
	if r.Boundary != "" && r.Boundary != "start" && r.Boundary != "end" {
		return fail("--boundary must be start or end")
	}
	r.ResumeCondition = values["--resume-condition"]
	r.Raw = values["--raw"] == "true"
	r.Recover = values["--recover"] == "true"
	r.Relocate = values["--relocate"] == "true"
	r.FromProjectDir, r.FromBinary = values["--from-project-dir"], values["--from-binary"]
	if r.Command == "install" {
		if r.Relocate {
			for _, p := range []string{r.ProjectDir, r.FromProjectDir, r.FromBinary} {
				if !filepath.IsAbs(p) {
					return fail("relocate requires absolute new and old paths")
				}
			}
		} else if _, hasRoot := values["--from-project-dir"]; hasRoot {
			return fail("source paths require --relocate")
		} else if _, hasBinary := values["--from-binary"]; hasBinary {
			return fail("source paths require --relocate")
		}
	}
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
	if r.Command == "intent" && (r.Action == "documents" || r.Action == "plan") && ((r.Expect == "") != (r.File == "")) {
		return fail("--expect and --file must be supplied together")
	}
	return r, nil
}
