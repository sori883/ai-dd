package minimal

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/flow"
	"github.com/sori883/ai-dd/src/internal/install"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestIntentDocumentsCommands(t *testing.T) {
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
	draft := s.draftPath("session")
	if err := os.MkdirAll(filepath.Dir(draft), 0700); err != nil {
		t.Fatal(err)
	}
	doc := `{"inputs":[],"outputs":[{"stage":"integration","path":"aidlc/spaces/default/knowledge/codekb/feature.md","metadata":{"type":"Knowledge","title":"Feature","description":"Current"}}]}`
	if err = os.WriteFile(draft, []byte(doc), 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := s.Execute(cli.MinimalRequest{Command: "intent", Action: "documents", Target: st.ID, Space: "default", Expect: strconv.FormatUint(st.Revision, 10), File: draft})
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &st); err != nil || len(st.Config.DocumentOutputs) != 1 {
		t.Fatalf("replacement failed %s %v", raw, err)
	}
	raw, err = s.Execute(cli.MinimalRequest{Command: "intent", Action: "documents", Target: st.ID, Space: "default"})
	if err != nil || !strings.Contains(string(raw), "feature.md") {
		t.Fatalf("read %s %v", raw, err)
	}
	if err = os.WriteFile(draft, []byte(`{"objective":"updated"}`), 0600); err != nil {
		t.Fatal(err)
	}
	raw, err = s.Execute(cli.MinimalRequest{Command: "intent", Action: "configure", Target: st.ID, Space: "default", Expect: strconv.FormatUint(st.Revision, 10), File: draft})
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &st); err != nil || len(st.Config.DocumentOutputs) != 1 {
		t.Fatal("configure lost document lists")
	}
	if err = os.WriteFile(draft, []byte(`{"document_outputs":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Execute(cli.MinimalRequest{Command: "intent", Action: "configure", Target: st.ID, Space: "default", Expect: strconv.FormatUint(st.Revision, 10), File: draft}); err == nil {
		t.Fatal("configure accepted document authority")
	}
}

func TestIntentDocumentsHookRepair(t *testing.T) {
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
	title, description := "Feature", "Current"
	st, err = store.SetDocuments(st.ID, st.Revision, flow.IntentDocuments{Inputs: []flow.DocumentDeclaration{}, Outputs: []flow.DocumentDeclaration{{Stage: "discovery", Path: "aidlc/spaces/default/knowledge/codekb/custom.md", Metadata: okfmemory.DocumentMatch{Type: "Knowledge", Title: &title, Description: &description}}}})
	if err != nil {
		t.Fatal(err)
	}
	hook(t, s, "SessionStart", "", "", "", false)
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	if _, err = s.Execute(cli.MinimalRequest{Command: "session", Action: "bind", Target: st.ID, Space: "default", Session: "session"}); err != nil {
		t.Fatal(err)
	}
	read := "/opt/aidlc intent documents " + st.ID + " --space default"
	if deny(hook(t, s, "PreToolUse", "Bash", "read", read, false)) {
		t.Fatal("documents read denied")
	}
	hook(t, s, "PostToolUse", "Bash", "read", "", false)
	repair := "/opt/aidlc memory create codekb/custom --space default --body-file " + s.draftPath("session") + " --actor process:a --type Knowledge --title Feature --description Current"
	if deny(hook(t, s, "PreToolUse", "Bash", "repair", repair, false)) {
		t.Fatal("declared output repair denied")
	}
	hook(t, s, "PostToolUse", "Bash", "repair", "", false)
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	if !deny(hook(t, s, "PreToolUse", "Bash", "unread", repair, false)) {
		t.Fatal("repair bypassed turn Rules")
	}
}

func TestIntentDocumentsUnregisteredInputRepair(t *testing.T) {
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
	st.Stage = "planning"
	st, err = store.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	session := &Session{Space: "default", Intent: st.ID}
	command := "/opt/aidlc memory create design/recovered --space default --body-file draft --actor process:a --type Requirements --title Req --description Req --intent-id " + st.ID
	in := HookInput{Tool: "Bash"}
	in.Input.Command = command
	if !s.documentRepair(in, session, st) {
		t.Fatal("unresolved required selector repair denied")
	}
	in.Input.Command = strings.Replace(command, st.ID, strings.Repeat("b", 32), 1)
	if s.documentRepair(in, session, st) {
		t.Fatal("other Intent repair allowed")
	}
}

func TestIntentDocumentsProcedureResolution(t *testing.T) {
	root := t.TempDir()
	if _, err := install.Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	s := Service{Root: root}
	store := flow.Store{Root: root, Space: "default"}
	st, err := store.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := s.Execute(cli.MinimalRequest{Command: "intent", Action: "procedure", Target: st.ID, Space: "default"})
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]json.RawMessage
	if err = json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(view["inputs"]), "rules/rule.md") || !strings.Contains(string(view["outputs"]), st.ID+"/requirements.md") {
		t.Fatalf("missing concrete resolution %s", raw)
	}
}

func TestIntentDocumentsRoundtrip(t *testing.T) {
	root := t.TempDir()
	if _, err := install.Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	s := Service{Root: root}
	st, err := (flow.Store{Root: root, Space: "default"}).Create("Fresh")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := s.Execute(cli.MinimalRequest{Command: "intent", Action: "documents", Target: st.ID, Space: "default"})
	if err != nil {
		t.Fatal(err)
	}
	var docs flow.IntentDocuments
	if err = json.Unmarshal(raw, &docs); err != nil {
		t.Fatal(err)
	}
	if docs.Inputs == nil || docs.Outputs == nil {
		t.Fatalf("empty lists are not arrays: %s", raw)
	}
	draft := filepath.Join(root, "documents.json")
	if err = os.WriteFile(draft, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Execute(cli.MinimalRequest{Command: "intent", Action: "documents", Target: st.ID, Space: "default", Expect: strconv.FormatUint(st.Revision, 10), File: draft}); err != nil {
		t.Fatalf("read/update roundtrip: %v", err)
	}
}
