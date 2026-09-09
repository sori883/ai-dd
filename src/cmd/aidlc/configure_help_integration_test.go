//go:build integration

package main

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/flow"
)

func TestConfigureHelpExamples(t *testing.T) {
	binary := buildMinimalBinary(t)
	for _, tc := range []struct {
		name  string
		index int
	}{{"without_units", 0}, {"with_units", 1}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			runMinimalProcess(t, root, "git", "init", "-q")
			runMinimalCLI(t, binary, root, nil, "install", "codex", "--project-dir", root)
			writeMinimalFixture(t, filepath.Join(root, "aidlc/spaces/default/knowledge/codekb/current.md"), "---\ntype: Design\ntitle: Addition\ndescription: Current behavior\n---\nAdd returns the sum.\n")
			runMinimalProcess(t, root, "git", "add", ".")
			runMinimalProcess(t, root, "git", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "assets")
			head := strings.TrimSpace(string(runMinimalProcess(t, root, "git", "rev-parse", "HEAD")))
			help := runMinimalCLI(t, binary, root, nil, "intent", "configure", "--help")
			examples := regexp.MustCompile("(?s)```json\\n(.*?)\\n```").FindAllStringSubmatch(string(help), -1)
			if len(examples) != 2 {
				t.Fatalf("missing examples: %s", help)
			}
			config := strings.ReplaceAll(examples[tc.index][1], "<CURRENT_HEAD>", head)
			file := filepath.Join(root, "aidlc/.runtime/config.json")
			writeMinimalFixture(t, file, config)
			var st flow.State
			read := func(raw []byte) {
				t.Helper()
				if err := json.Unmarshal(raw, &st); err != nil {
					t.Fatal(err)
				}
			}
			read(runMinimalCLI(t, binary, root, nil, "intent", "create", tc.name, "--space", "default"))
			call := func(action string, args ...string) {
				t.Helper()
				base := []string{"intent", action, st.ID, "--space", "default", "--expect", strconv.FormatUint(st.Revision, 10)}
				read(runMinimalCLI(t, binary, root, nil, append(base, args...)...))
			}
			boundaryFixtureDocument(t, root, st.ID, "Requirements")
			call("configure", "--file", file)
			call("begin")
			reviewer := filepath.Join(t.TempDir(), "review")
			runMinimalProcess(t, root, "git", "worktree", "add", "--detach", reviewer, head)
			request := func(r flow.ReviewRequest) string {
				t.Helper()
				raw, err := json.Marshal(r)
				if err != nil {
					t.Fatal(err)
				}
				p := filepath.Join(root, "aidlc/.runtime/review.json")
				writeMinimalFixture(t, p, string(raw))
				return p
			}
			call("review", "--file", request(flow.ReviewRequest{Action: "assign", CoordinatorSession: "coordinator", Session: "reviewer", Root: reviewer}))
			check := func() flow.Gate {
				t.Helper()
				var gate flow.Gate
				raw := runMinimalCLI(t, binary, root, nil, "intent", "check", st.ID, "--space", "default")
				if err := json.Unmarshal(raw, &gate); err != nil {
					t.Fatal(err)
				}
				if gate.Status != "pass" {
					t.Fatalf("Sensor rejected: %s", raw)
				}
				return gate
			}
			gate := check()
			call("review", "--file", request(flow.ReviewRequest{Action: "accept", Session: "reviewer", Root: reviewer, Target: gate.Target, Status: "pass", Summary: "Deterministic example validation, not AI review"}))
			call("advance")
			if st.Stage != "planning" {
				t.Fatalf("stage=%s", st.Stage)
			}
			// Reapply the public help example at planning, then evaluate its real prerequisites.
			boundaryFixtureDocument(t, root, st.ID, "ImplementationPlan")
			call("begin")
			call("configure", "--file", file)
			check()
			if st.Config.CodeRevision != head || len(st.Config.Units) != tc.index {
				t.Fatal("configuration not preserved")
			}
			t.Logf("configure exit=0 planning Sensor=pass example=%s HEAD=%s revision=%d", tc.name, head, st.Revision)
		})
	}
}
