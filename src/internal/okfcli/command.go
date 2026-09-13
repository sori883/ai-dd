// Package okfcli defines the public knowledge command contract.
package okfcli

import (
	"fmt"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"io/fs"
	"strings"
)

type CommandRequest struct {
	Action, Target, Space, ProjectDir, BodyFile, Actor, Expect string
	IntentID                                                   *string
	Metadata                                                   okfmemory.MetadataInput
}
type Dependencies struct {
	Execute func(CommandRequest) ([]byte, error)
}

// ParseCommand accepts one complete OKF operation, never runtime or installer commands.
func ParseCommand(args []string) (r CommandRequest, err error) {
	fail := func(message string) (CommandRequest, error) { return r, fmt.Errorf("%s: %w", message, fs.ErrInvalid) }
	if len(args) == 0 {
		return fail("missing action")
	}
	r.Action = args[0]
	allowed := " --space --project-dir "
	required := "--space"
	min, max := 0, 0
	switch r.Action {
	case "rules", "check":
	case "show":
		min, max = 1, 1
	case "search":
		max = 1
		allowed += "--intent-id "
	case "create", "update":
		min, max = 1, 1
		allowed += "--body-file --actor --type --title --description --tag --status --intent-id --resource --stale-after --sources-json --verified-json --metadata-json "
		required += " --body-file --actor"
		if r.Action == "create" {
			required += " --type --title --description"
		} else {
			allowed += "--expect --clear-tags "
			required += " --expect"
		}
	default:
		return fail("unknown action")
	}
	values := map[string]string{}
	var positional []string
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		if arg == "--file" {
			return fail("use --body-file for body-only Markdown; see okf " + r.Action + " --help")
		}
		if !strings.Contains(allowed, " "+arg+" ") {
			return fail("unknown flag " + arg)
		}
		if _, exists := values[arg]; exists && arg != "--tag" {
			return fail("duplicate flag " + arg)
		}
		if arg == "--clear-tags" {
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
	if len(positional) == 1 {
		r.Target = positional[0]
	}
	r.Space, r.ProjectDir = values["--space"], values["--project-dir"]
	r.BodyFile, r.Actor, r.Expect = values["--body-file"], values["--actor"], values["--expect"]
	if id, exists := values["--intent-id"]; exists {
		if !okfmemory.ValidID(id) {
			return fail("invalid --intent-id")
		}
		r.IntentID = &id
	}
	r.Metadata.Actor, r.Metadata.IntentID = r.Actor, r.IntentID
	r.Metadata.ClearTags = values["--clear-tags"] == "true"
	if r.Metadata.ClearTags && r.Metadata.Tags != nil {
		return fail("--tag and --clear-tags conflict")
	}
	for flag, dest := range map[string]**string{"--type": &r.Metadata.Type, "--title": &r.Metadata.Title, "--description": &r.Metadata.Description, "--status": &r.Metadata.Status, "--resource": &r.Metadata.Resource, "--stale-after": &r.Metadata.StaleAfter, "--sources-json": &r.Metadata.SourcesJSON, "--verified-json": &r.Metadata.VerifiedJSON, "--metadata-json": &r.Metadata.ExtraJSON} {
		if value, exists := values[flag]; exists {
			if strings.TrimSpace(value) == "" && flag != "--resource" {
				return fail("empty " + flag)
			}
			*dest = &value
		}
	}
	if status := r.Metadata.Status; status != nil && *status != "draft" && *status != "stable" && *status != "deprecated" {
		return fail("--status must be draft, stable or deprecated")
	}
	return r, nil
}
