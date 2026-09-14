package app

import (
	"github.com/sori883/ai-dd/src/internal/flow"
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
			st = executionFixtureState(t, flow.Store{Root: s.Root, Space: "default"}, st, "architecture-analysis")
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
			command := strings.Join([]string{"/opt/okf create", tc.name, "--space default --body-file", s.draftPath("session"), "--actor process:test --type", tc.kind, "--title Shared --description Shared"}, " ")
			out := hook(t, s, "PreToolUse", "Bash", "repair", command, false)
			if deny(out) == tc.allowed {
				t.Fatalf("repair allowed=%v want %v: %+v", !deny(out), tc.allowed, out)
			}
		})
	}
}
