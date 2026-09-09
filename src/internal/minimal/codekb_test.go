package minimal

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/flow"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeKBBeginRepair(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		allowed    bool
	}{
		{name: "codekb/current-analysis", kind: "CurrentAnalysis", allowed: true},
		{name: "codekb/architecture", kind: "Architecture", allowed: true},
		{name: "knowledge/current-analysis", kind: "CurrentAnalysis", allowed: true},
		{name: "knowledge/architecture", kind: "Architecture", allowed: true},
		{name: "codekb/arbitrary", kind: "Knowledge", allowed: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, st := setup(t)
			hook(t, s, "SessionStart", "", "", "", false)
			hook(t, s, "UserPromptSubmit", "", "", "", false)
			bind(t, s, st.ID)
			current, err := (flow.Store{Root: s.Root, Space: "default"}).Read(st.ID)
			if err != nil || current.Entry != nil {
				t.Fatal("fixture must be unstarted", err)
			}
			if !deny(hook(t, s, "PreToolUse", "Bash", "ordinary", "touch code.go", false)) {
				t.Fatal("unstarted ordinary work allowed")
			}
			command := strings.Join([]string{"/opt/aidlc memory create", tc.name, "--space default --body-file", s.draftPath("session"), "--actor process:test --type", tc.kind, "--title Shared --description Shared"}, " ")
			out := hook(t, s, "PreToolUse", "Bash", "repair", command, false)
			if deny(out) == tc.allowed {
				t.Fatalf("repair allowed=%v want %v: %+v", !deny(out), tc.allowed, out)
			}
		})
	}
}

func TestCodeKBMemory(t *testing.T) {
	s, _ := setup(t)
	file := filepath.Join(s.Root, "body.md")
	if err := os.WriteFile(file, []byte("Current feature details."), 0600); err != nil {
		t.Fatal(err)
	}
	request := bodyRequest(file)
	request.Target = "codekb/note"
	if _, err := s.Execute(request); err != nil {
		t.Fatal(err)
	}
	raw, err := s.Execute(cli.MinimalRequest{Command: "memory", Action: "show", Target: "codekb/note", Space: "default"})
	if err != nil {
		t.Fatal(err)
	}
	var shown map[string]string
	if err = json.Unmarshal(raw, &shown); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shown["content"], "Current feature details.") {
		t.Fatalf("show: %s", raw)
	}
	raw, err = s.Execute(cli.MinimalRequest{Command: "memory", Action: "search", Target: "Memory", Space: "default"})
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]string
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["concept_id"] != "codekb/note" {
		t.Fatalf("search: %s", raw)
	}
}
