package app

import (
	"encoding/json"
	"path/filepath"

	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/install"
	"github.com/sori883/ai-dd/src/internal/okfapp"
	"github.com/sori883/ai-dd/src/internal/okfcli"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
)

func encode(value any) ([]byte, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	return append(raw, '\n'), err
}
func (s Service) readDraft(file string) ([]byte, error) {
	if filepath.IsAbs(file) {
		relative, err := filepath.Rel(s.Root, file)
		if err != nil {
			return nil, err
		}
		file = relative
	}
	return okfmemory.ReadFile(s.Root, filepath.ToSlash(file))
}

// Execute runs one validated public operation; partial saves return output together with an error.
func (s Service) Execute(r cli.CommandRequest) ([]byte, error) {
	if r.Command == "assignment" {
		return s.executeAssignment(r)
	}
	if r.Command == "session" && r.Action == "bind" {
		return s.bindFlow(r.Session, r.Space, r.Target, r.Recover)
	}
	if r.Command == "intent" || r.Command == "unit" {
		return s.executeFlow(r)
	}
	switch r.Command + "/" + r.Action {
	case "install/codex":
		if r.Relocate {
			result, err := install.Relocate(s.Root, s.Binary, r.FromProjectDir, r.FromBinary)
			out, _ := encode(result)
			return out, err
		}
		result, err := install.Codex(s.Root, s.Binary)
		out, _ := encode(result)
		return out, err
	case "session/inspect":
		state, err := s.Inspect(r.Session)
		if err != nil {
			return nil, err
		}
		return encode(state)
	case "memory/rules", "memory/search", "memory/show", "memory/check", "memory/create", "memory/update":
		return (okfapp.Service{Root: s.Root}).Execute(okfcli.CommandRequest{Action: r.Action, Target: r.Target, Space: r.Space, ProjectDir: r.ProjectDir, BodyFile: r.BodyFile, Actor: r.Actor, Expect: r.Expect, IntentID: r.IntentID, Metadata: r.Metadata})
	}
	return nil, invalid("unsupported operation")
}
