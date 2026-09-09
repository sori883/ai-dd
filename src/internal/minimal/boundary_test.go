package minimal

import (
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/flow"
	"github.com/sori883/ai-dd/src/internal/install"
	"strconv"
	"strings"
	"testing"
)

func TestBoundaryHookRepairAndBegin(t *testing.T) {
	root := t.TempDir()
	if _, err := install.Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	s := Service{Root: root, Binary: "/opt/aidlc"}
	store := flow.Store{Root: root, Space: "default"}
	st, err := store.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	hook(t, s, "SessionStart", "", "", "", false)
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	if _, err = s.Execute(cli.MinimalRequest{Command: "session", Action: "bind", Target: st.ID, Space: "default", Session: "session"}); err != nil {
		t.Fatal(err)
	}
	if !deny(hook(t, s, "PreToolUse", "Bash", "unstarted", "touch code.go", false)) {
		t.Fatal("unstarted ordinary work allowed")
	}
	repair := "/opt/aidlc memory create design/" + st.ID + "/requirements --space default --body-file " + s.draftPath("session") + " --actor process:coordinator --type Requirements --title Req --description Req"
	if deny(hook(t, s, "PreToolUse", "Bash", "repair", repair, false)) {
		t.Fatal("fixed document repair blocked")
	}
	hook(t, s, "PostToolUse", "Bash", "repair", "", false)
	if !deny(hook(t, s, "PreToolUse", "Bash", "random", "/opt/aidlc memory create knowledge/unknown --space default --body-file draft --actor process:a --type Knowledge --title X --description X", false)) {
		t.Fatal("arbitrary memory bypassed begin")
	}
	if _, err = s.Execute(cli.MinimalRequest{Command: "intent", Action: "begin", Target: st.ID, Space: "default", Expect: strconv.FormatUint(st.Revision, 10)}); err != nil {
		t.Fatal(err)
	}
	if deny(hook(t, s, "PreToolUse", "Bash", "started", "touch code.go", false)) {
		t.Fatal("started operation denied")
	}
	hook(t, s, "PostToolUse", "Bash", "started", "", false)
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	if !deny(hook(t, s, "PreToolUse", "Bash", "unread", repair, false)) {
		t.Fatal("repair bypassed current Rule read")
	}
}

func TestBoundaryUnitClaimRequiresBegin(t *testing.T) {
	root := t.TempDir()
	if _, err := install.Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	store := flow.Store{Root: root, Space: "default"}
	st, err := store.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	st.Stage = "tdd"
	st.Config.Units = []flow.Unit{{ID: "a", Status: "pending"}}
	st, err = store.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Unit(st.ID, st.Revision, flow.UnitRequest{Action: "claim", Unit: "a"})
	if err == nil || !strings.Contains(err.Error(), "begin") {
		t.Fatalf("missing begin was not first gate: %v", err)
	}
}
