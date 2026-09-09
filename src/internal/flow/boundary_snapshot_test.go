package flow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoundaryTransitionCollectorSnapshot(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(fmt.Sprint(remove), func(t *testing.T) {
			s := flowStore(t)
			boundaryFile(t, s, "evidence.txt", "reviewed")
			c := boundaryCollector{store: s}
			first, ok := c.file("evidence.txt")
			if !ok {
				t.Fatal(c.failures)
			}
			if remove {
				if err := os.Remove(filepath.Join(s.Root, "evidence.txt")); err != nil {
					t.Fatal(err)
				}
			} else {
				boundaryFile(t, s, "evidence.txt", "changed")
			}
			second, ok := c.file("evidence.txt")
			if !ok || string(first) != string(second) {
				t.Fatal("collector reread changed or missing bytes under the first hash")
			}
		})
	}
}
func TestBoundaryTransitionUsesOneSensorSnapshot(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(fmt.Sprint(remove), func(t *testing.T) {
			s, st := boundaryFixture(t)
			st.Stage = "tdd"
			prepareBoundaryStage(t, s, &st)
			head := flowGit(t, s.Root, "rev-parse", "HEAD")
			st.Config = Config{Objective: "Build", Scope: []string{"src"}, Acceptance: []string{"works"}, NoMaterialsReason: "new", ADR: ADR{Reason: "none"}, CodeRevision: head, DirectCommit: head, Plan: "Implement", Tests: []string{"go test"}}
			prepareBoundaryResults(t, s, &st)
			if err := s.persist(st); err != nil {
				t.Fatal(err)
			}
			g, err := s.Check(st.ID)
			if err != nil || g.Status != "pass" {
				t.Fatalf("gate %+v %v", g, err)
			}
			st.Review = Gate{Status: "pass", Target: g.Target}
			if err = s.persist(st); err != nil {
				t.Fatal(err)
			}
			output := "aidlc/evidence/tdd.txt"
			want := boundaryVersion(t, s, output)
			realGit, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			bin := t.TempDir()
			count := filepath.Join(bin, "count")
			quote := func(v string) string { return "'" + strings.ReplaceAll(v, "'", "'\"'\"'") + "'" }
			mutation := "printf changed > " + quote(filepath.Join(s.Root, output))
			if remove {
				mutation = "rm " + quote(filepath.Join(s.Root, output))
			}
			script := "#!/bin/sh\ncase \"$*\" in *cat-file*) n=0; [ ! -f " + quote(count) + " ] || n=$(cat " + quote(count) + "); n=$((n+1)); echo $n > " + quote(count) + "; if [ $n -eq 2 ]; then " + mutation + "; fi;; esac\nexec " + quote(realGit) + " \"$@\"\n"
			if err = os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			next, err := s.Transition(st.ID, st.Revision, TransitionRequest{Action: "advance"})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, f := range next.Accepted["tdd"].Outputs {
				if f.Path == output {
					found = f == want
				}
			}
			if !found {
				t.Fatal("accepted outputs differ from the reviewed Sensor snapshot")
			}
			raw, err := os.ReadFile(count)
			if err != nil || strings.TrimSpace(string(raw)) != "1" {
				t.Fatalf("Sensor recollected after review comparison: %s %v", raw, err)
			}
		})
	}
}

func TestBoundaryTransitionEvidenceRole(t *testing.T) {
	for _, changed := range []string{"results.json", "output.log"} {
		t.Run(changed, func(t *testing.T) {
			s, st := boundaryFixture(t)
			st.Stage = "tdd"
			prepareBoundaryStage(t, s, &st)
			head := flowGit(t, s.Root, "rev-parse", "HEAD")
			st.Config = Config{Objective: "Build", Scope: []string{"src"}, Acceptance: []string{"works"}, NoMaterialsReason: "new", ADR: ADR{Reason: "none"}, CodeRevision: head, DirectCommit: head, Plan: "Implement", Tests: []string{"go test"}}
			prefix := "aidlc/spaces/" + s.Space + "/knowledge/evidence/"
			boundaryFile(t, s, prefix+"output.log", "PASS")
			boundaryFile(t, s, prefix+"results.json", fmt.Sprintf(`{"stage":"tdd","runs":[{"command":"go test","commit":%q,"exit_code":0,"output_path":%q}]}`, head, prefix+"output.log"))
			st.Config.TestResults = []string{prefix + "results.json"}
			if err := s.persist(st); err != nil {
				t.Fatal(err)
			}
			gate, err := s.Check(st.ID)
			if err != nil || gate.Status != "pass" {
				t.Fatalf("%+v %v", gate, err)
			}
			st.Review = Gate{Status: "pass", Target: gate.Target}
			if err = s.persist(st); err != nil {
				t.Fatal(err)
			}
			st, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "advance"})
			if err != nil {
				t.Fatal(err)
			}
			if len(st.Accepted["tdd"].Outputs) != 2 {
				t.Errorf("TDD acceptance includes shared documents: %+v", st.Accepted["tdd"].Outputs)
			}
			boundaryFile(t, s, prefix+changed, "changed")
			start, _, _ := s.startState(st)
			if start.Status == "pass" {
				t.Error("integration start ignored changed knowledge evidence")
			}
			st.Entry = &StageEntry{Stage: "integration"}
			if err = s.checkWorkState(st); err == nil {
				t.Error("work ignored changed knowledge evidence")
			}
		})
	}
}
