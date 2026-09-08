package minimal

import (
	"github.com/sori883/ai-dd/src/internal/flow"
	"os"
	"path/filepath"
	"testing"
)

func TestMemoryHelpHook(t *testing.T) {
	for _, mode := range []string{"unselected", "unread", "running", "inactive"} {
		t.Run(mode, func(t *testing.T) {
			s, st := setup(t)
			if mode == "inactive" {
				st.Status = "completed"
				if _, err := (flow.Store{Root: s.Root, Space: "default"}).Save(st, st.Revision); err != nil {
					t.Fatal(err)
				}
			}
			if mode != "unselected" {
				hook(t, s, "UserPromptSubmit", "", "", "", false)
				bind(t, s, st.ID)
			}
			if mode == "unread" {
				hook(t, s, "UserPromptSubmit", "", "", "", false)
			}
			if mode == "running" {
				hook(t, s, "PreToolUse", "Bash", "running", "sleep 100", false)
			}
			file := filepath.Join(s.Root, "aidlc/.runtime/flow/sessions/session.txt")
			before, beforeErr := os.ReadFile(file)
			for _, command := range []string{"/opt/aidlc memory create --help", "/opt/aidlc help memory update", "/opt/aidlc intent reopen --help", "/opt/aidlc --help"} {
				if out := hook(t, s, "PreToolUse", "Bash", "help", command, false); deny(out) {
					t.Fatalf("normal help denied %+v", out)
				}
			}
			after, afterErr := os.ReadFile(file)
			if string(before) != string(after) || os.IsNotExist(beforeErr) != os.IsNotExist(afterErr) {
				t.Fatal("help changed session")
			}
			for _, command := range []string{"/other/aidlc memory create --help", "/opt/aidlc memory create --help > file", "/opt/aidlc memory create --help; touch file", "/opt/aidlc memory create name --help", "/opt/aidlc unknown --help", "/opt/aidlc memory create --help --body-file file"} {
				if out := hook(t, s, "PreToolUse", "Bash", "bad", command, false); !deny(out) {
					t.Fatalf("invalid help allowed %s", command)
				}
			}
		})
	}
}
