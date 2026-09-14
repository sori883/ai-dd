package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHookSplitCLI(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		allow         bool
	}{
		{"read", "/opt/okf rules --space default", true},
		{"help", "/opt/okf create --help", true},
		{"wrong binary", "/tmp/okf rules --space default", false},
		{"bare binary", "okf rules --space default", false},
		{"old alias", "/opt/aidlc memory rules --space default", false},
		{"cross role", "/opt/okf intent list --space default", false},
		{"installer", "/opt/aidlc-install codex --help", false},
		{"wrong root", "/opt/okf rules --space default --project-dir /elsewhere", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := setup(t)
			s.OKFBinary = "/opt/okf"
			out := hook(t, s, "PreToolUse", "Bash", "id", tc.command, false)
			if deny(out) == tc.allow {
				t.Fatalf("allow=%v output=%+v", tc.allow, out)
			}
		})
	}
}
func TestHookSplitCLIPost(t *testing.T) {
	s, st := setup(t)
	s.OKFBinary = "/opt/okf"
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	bind(t, s, st.ID)
	before, err := s.Inspect("session")
	if err != nil {
		t.Fatal(err)
	}
	if deny(hook(t, s, "PreToolUse", "Bash", "read", "/opt/okf rules --space default", false)) {
		t.Fatal("read denied")
	}
	after, _ := s.Inspect("session")
	if after != before {
		t.Fatal("read exception changed session evidence")
	}
	after.Tool = "running"
	if err := s.save("session", after); err != nil {
		t.Fatal(err)
	}
	hook(t, s, "PostToolUse", "Bash", "other", "/opt/okf rules --space default", false)
	got, _ := s.Inspect("session")
	if got.Tool != "running" {
		t.Fatal("unmatched Post released slot")
	}
	hook(t, s, "PostToolUse", "Bash", "running", "/opt/okf rules --space default", false)
	got, _ = s.Inspect("session")
	if got.Tool != "" || got.RuleHash != before.RuleHash {
		t.Fatal("Post altered Rule contract")
	}
	p := filepath.Join(s.Root, "aidlc/spaces/default/knowledge/rules/rule.md")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(raw, []byte("\nChanged\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if !deny(hook(t, s, "PreToolUse", "Bash", "work", "touch code.go", false)) {
		t.Fatal("stale Rule allowed work")
	}
}
