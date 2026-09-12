package app

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/flow"
	"testing"
)

func TestVerificationCLI(t *testing.T) {
	s, st := setup(t)
	raw, err := s.Execute(cli.CommandRequest{Command: "intent", Action: "hash", Target: st.ID, Space: "default"})
	if err != nil {
		t.Fatal(err)
	}
	var view flow.VerificationView
	if err := json.Unmarshal(raw, &view); err != nil || view.IntentID != st.ID {
		t.Fatalf("hash %s %v", raw, err)
	}
	input := HookInput{Tool: "Bash"}
	input.Input.Command = s.Binary + " intent hash " + st.ID + " --space default"
	if !s.exception(input, &Session{Intent: st.ID, Space: "default"}) {
		t.Fatal("hash not read-only hook exception")
	}
}
