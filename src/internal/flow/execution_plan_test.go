package flow

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/install"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"github.com/sori883/ai-dd/src/internal/workflow"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func schemaPlanState(s Store) State {
	return State{SchemaVersion: 5, DefinitionHash: strings.Repeat("a", 64), ID: strings.Repeat("b", 32), Space: s.Space, Name: "Schema", Revision: 1, Stage: "initialization", Status: "active", CurrentStepID: "s01", ExecutionPlan: ExecutionPlan{NextID: 3, Bootstrap: []ExecutionStep{{ID: "s01", Stage: "initialization", Status: "pending"}, {ID: "s02", Stage: "discovery", Status: "pending"}}}}
}

func TestExecutionPlanSchemaState(t *testing.T) {
	s := flowStore(t)
	st := schemaPlanState(s)
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	if err = filestore.WriteFile(s.Root, s.path(st.ID), raw); err != nil {
		t.Fatal(err)
	}
	got, err := s.Read(st.ID)
	if err != nil {
		t.Fatalf("valid schema5 state rejected: %v", err)
	}
	if got.CurrentStepID != "s01" || got.ExecutionPlan.Approved != nil {
		t.Fatalf("bootstrap fabricated approval: %+v", got)
	}
	for _, tc := range []struct {
		name   string
		change func(*State)
	}{
		{name: "old schema", change: func(st *State) { st.SchemaVersion = 4 }},
		{name: "duplicate id", change: func(st *State) { st.ExecutionPlan.Bootstrap[1].ID = "s01" }},
		{name: "wrong prefix", change: func(st *State) { st.ExecutionPlan.Bootstrap[1].Stage = "tdd" }},
		{name: "invalid id", change: func(st *State) { st.ExecutionPlan.Bootstrap[1].ID = "../s02" }},
		{name: "reused next id", change: func(st *State) { st.ExecutionPlan.NextID = 2 }},
		{name: "unknown progress", change: func(st *State) { st.ExecutionPlan.Bootstrap[1].Status = "approved" }},
		{name: "current mismatch", change: func(st *State) { st.CurrentStepID = "s02" }},
		{name: "unapproved optional", change: func(st *State) {
			st.ExecutionPlan.Bootstrap = append(st.ExecutionPlan.Bootstrap, ExecutionStep{ID: "s03", Stage: "tdd", Status: "pending"})
			st.ExecutionPlan.NextID = 4
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := schemaPlanState(s)
			tc.change(&st)
			if err := s.persist(st); err == nil {
				t.Fatal("invalid execution plan accepted")
			}
		})
	}
}

func TestExecutionPlanSchemaChoices(t *testing.T) {
	s := flowStore(t)
	st := schemaPlanState(s)
	st.ExecutionPlan.Approved = &PlanVersion{Revision: 1, Reason: "investigation only", Steps: st.ExecutionPlan.Bootstrap, Omitted: []StageOmission{{Stage: "architecture-analysis", Reason: "not needed"}, {Stage: "planning", Reason: "no implementation"}, {Stage: "tdd", Reason: "no implementation"}, {Stage: "integration", Reason: "no implementation"}}}
	st.ExecutionPlan.Bootstrap = nil
	st.ExecutionPlan.Revision = 1
	fixturePlanApproval(t, &st)
	if err := s.persist(st); err != nil {
		t.Fatalf("complete choices rejected: %v", err)
	}
	for _, mode := range []string{"missing choice", "empty reason", "duplicate omission", "selected and omitted"} {
		t.Run(mode, func(t *testing.T) {
			candidate := st
			version := *st.ExecutionPlan.Approved
			version.Omitted = append([]StageOmission{}, version.Omitted...)
			candidate.ExecutionPlan.Approved = &version
			switch mode {
			case "missing choice":
				version.Omitted = version.Omitted[:3]
			case "empty reason":
				version.Omitted[0].Reason = ""
			case "duplicate omission":
				version.Omitted = append(version.Omitted, version.Omitted[0])
			case "selected and omitted":
				version.Steps = append(append([]ExecutionStep{}, version.Steps...), ExecutionStep{ID: "s03", Stage: "tdd", Status: "pending"})
				candidate.ExecutionPlan.NextID = 4
			}
			if err := s.persist(candidate); err == nil {
				t.Fatal("invalid adoption accepted")
			}
		})
	}
}

func executionFixture(t *testing.T) Store {
	t.Helper()
	root := t.TempDir()
	if _, err := install.Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	stages := []map[string]string{}
	for _, id := range []string{"initialization", "discovery", "architecture-analysis", "planning", "tdd", "integration"} {
		stages = append(stages, map[string]string{"id": id, "name": id, "procedure": "stages/" + id + ".md"})
		if id == "initialization" || id == "architecture-analysis" {
			body := "---\nstage_id: " + id + "\nagents: []\ninputs: []\noutputs: []\nsensors:\n  start: " + id + "-start\n  end: " + id + "-end\n---\n# Procedure\nInspect the project.\n"
			if err := os.WriteFile(filepath.Join(root, "aidlc/workflow/stages", id+".md"), []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	raw, err := json.Marshal(map[string]any{"schema_version": 2, "required_prefix": []string{"initialization", "discovery"}, "stages": stages})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "aidlc/workflow/stage-graph.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	return Store{Root: root, Space: "default"}
}

func TestExecutionPlanBootstrap(t *testing.T) {
	s := executionFixture(t)
	marker := filepath.Join(s.Root, "aidlc/spaces/default/user-file")
	if err := os.WriteFile(marker, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	st, err := s.Create("New purpose")
	if err != nil {
		t.Fatalf("bootstrap create: %v", err)
	}
	if st.SchemaVersion != 5 || st.Stage != "initialization" || st.CurrentStepID != "s01" || st.ExecutionPlan.Approved != nil {
		t.Fatalf("wrong bootstrap: %+v", st)
	}
	if len(st.Accepted) != 0 || st.Entry != nil {
		t.Fatal("created fake completion")
	}
	candidate := st
	candidate.ExecutionPlan.Bootstrap = append([]ExecutionStep{}, st.ExecutionPlan.Bootstrap...)
	candidate.ExecutionPlan.Bootstrap[0].Status = "completed"
	candidate.Stage = "discovery"
	candidate.CurrentStepID = "s02"
	if _, err = s.Save(candidate, st.Revision); err == nil {
		t.Fatal("configure skipped initialization")
	}
	started, err := s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatalf("initialization begin: %v", err)
	}
	if started.ExecutionPlan.Bootstrap[0].Status != "active" || started.Entry == nil {
		t.Fatal("begin did not activate first execution")
	}
	if err = s.CheckWork(st.ID); err != nil {
		t.Fatal(err)
	}
	repeated, err := s.Begin(st.ID, started.Revision)
	if err != nil || repeated.Revision != started.Revision {
		t.Fatal("begin retry changed entry", err)
	}
	raw, err := os.ReadFile(marker)
	if err != nil || string(raw) != "keep" {
		t.Fatal("existing Space overwritten", err)
	}
}

func TestExecutionPlanBootstrapMissingConfiguration(t *testing.T) {
	for _, name := range []string{"aidlc/spaces/default/knowledge/rules/rule.md", ".codex/hooks.json", ".agents/skills/aidlc/SKILL.md"} {
		t.Run(name, func(t *testing.T) {
			s := executionFixture(t)
			st, err := s.Create("inspect")
			if err != nil {
				t.Fatal(err)
			}
			if err = os.Remove(filepath.Join(s.Root, name)); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Begin(st.ID, st.Revision); err == nil {
				t.Fatal("missing initialization input accepted")
			}
		})
	}
}

func initialPlanRequest() PlanRequest {
	return PlanRequest{Reason: "implement", Steps: []PlanStepInput{{ID: "s01", Stage: "initialization"}, {ID: "s02", Stage: "discovery"}, {Stage: "tdd"}}, Omitted: []StageOmission{{Stage: "architecture-analysis", Reason: "existing understanding"}, {Stage: "planning", Reason: "scope captured in requirements"}, {Stage: "integration", Reason: "test-only purpose"}}}
}

func TestExecutionPlanDraft(t *testing.T) {
	s := executionFixture(t)
	st, err := s.Create("draft")
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.ProposePlan(st.ID, st.Revision, initialPlanRequest())
	if err != nil {
		t.Fatalf("plan proposal: %v", err)
	}
	if st.ExecutionPlan.Approved != nil || st.ExecutionPlan.Draft == nil || len(executionSteps(st)) != 2 {
		t.Fatal("unapproved draft changed executable order")
	}
	draft := *st.ExecutionPlan.Draft
	if draft.Steps[2].ID != "s03" || st.ExecutionPlan.NextID != 4 {
		t.Fatalf("allocation: %+v", st.ExecutionPlan)
	}
	hash := PlanHash(draft)
	draft.Steps = append([]ExecutionStep{}, draft.Steps...)
	draft.Steps[0].Status = "active"
	if len(hash) != 64 || PlanHash(draft) != hash {
		t.Fatal("progress changes plan hash")
	}
	draft.Reason = "different"
	if PlanHash(draft) == hash {
		t.Fatal("change reason missing from plan hash")
	}
	if err = rejectDraft(&st); err != nil {
		t.Fatal(err)
	}
	if st.ExecutionPlan.Draft != nil || st.ExecutionPlan.NextID != 4 {
		t.Fatal("reject reused allocated id")
	}
	if err = s.persist(st); err != nil {
		t.Fatal(err)
	}
	st, err = s.ProposePlan(st.ID, st.Revision, initialPlanRequest())
	if err != nil {
		t.Fatal(err)
	}
	if st.ExecutionPlan.Draft.Steps[2].ID != "s04" {
		t.Fatal("rejected id reused")
	}
}

func TestExecutionPlanDraftRejects(t *testing.T) {
	for _, mode := range []string{"prefix", "duplicate", "unknown id", "stage swap", "choice missing", "completed removed", "worker running"} {
		t.Run(mode, func(t *testing.T) {
			s := executionFixture(t)
			st, err := s.Create("draft")
			if err != nil {
				t.Fatal(err)
			}
			request := initialPlanRequest()
			switch mode {
			case "prefix":
				request.Steps[0].Stage = "tdd"
			case "duplicate":
				request.Steps[2].ID = "s02"
			case "unknown id":
				request.Steps[2].ID = "s99"
			case "stage swap":
				request.Steps[2] = PlanStepInput{ID: "s02", Stage: "tdd"}
			case "choice missing":
				request.Omitted = request.Omitted[:2]
			case "completed removed":
				st.ExecutionPlan.Bootstrap[0].Status = "completed"
				st.CurrentStepID = "s02"
				st.Stage = "discovery"
				request.Steps = request.Steps[1:]
			case "worker running":
				st.Config.Units = []Unit{{StepID: st.CurrentStepID, ID: "worker", Status: "running"}}
			}
			if err = s.persist(st); err != nil {
				t.Fatal(err)
			}
			if _, err = s.ProposePlan(st.ID, st.Revision, request); err == nil {
				t.Fatal("invalid plan proposal accepted")
			}
		})
	}
}

func TestExecutionPlanEvidenceBindings(t *testing.T) {
	s := executionFixture(t)
	st, err := s.Create("bound evidence")
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if st.Entry.StepID != st.CurrentStepID {
		t.Error("begin entry lacks execution identity")
	}
	for _, mode := range []string{"entry", "sensor", "review", "unit"} {
		t.Run(mode, func(t *testing.T) {
			candidate := st
			switch mode {
			case "entry":
				entry := *st.Entry
				entry.StepID = "s02"
				candidate.Entry = &entry
			case "sensor":
				candidate.Sensor = Gate{StepID: "s02", Status: "pass", Target: strings.Repeat("a", 64)}
			case "review":
				candidate.Review = Gate{StepID: "s02", Status: "pass", Target: strings.Repeat("a", 64)}
			case "unit":
				candidate.Config.Units = []Unit{{StepID: "s02", ID: "u", Status: "integrated", ResultCommit: strings.Repeat("a", 40), IntegratedCommit: strings.Repeat("a", 40)}}
			}
			if err := s.persist(candidate); err == nil {
				t.Fatal("other execution evidence accepted")
			}
		})
	}
}

func TestExecutionPlanEvidenceInitializationEnd(t *testing.T) {
	s := executionFixture(t)
	st, err := s.Create("initialize")
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	c := s.endDocuments(st)
	if len(c.failures) > 0 {
		t.Fatalf("initialization required fabricated documents: %v", c.failures)
	}
}

func executionAt(t *testing.T, stage string) (Store, State) {
	t.Helper()
	s := executionFixture(t)
	st, err := s.Create("selected sensor")
	if err != nil {
		t.Fatal(err)
	}
	steps := []ExecutionStep{{ID: "s01", Stage: "initialization", Status: "completed"}, {ID: "s02", Stage: "discovery", Status: "completed"}, {ID: "s03", Stage: stage, Status: "pending"}}
	omissions := []StageOmission{}
	for _, optional := range []string{"architecture-analysis", "planning", "tdd", "integration"} {
		if optional != stage {
			omissions = append(omissions, StageOmission{Stage: optional, Reason: "not needed"})
		}
	}
	st.ExecutionPlan = ExecutionPlan{Revision: 1, NextID: 4, Approved: &PlanVersion{Revision: 1, Reason: "selected work", Steps: steps, Omitted: omissions}}
	st.CurrentStepID = "s03"
	st.Stage = stage
	name := boundaryDoc(t, s, st, "Requirements")
	st.Accepted = map[string]StageAcceptance{"s02": {StepID: "s02", Stage: "discovery", ReviewTarget: strings.Repeat("a", 64), Outputs: []FileVersion{boundaryVersion(t, s, name)}}}
	fixturePlanApproval(t, &st)
	return s, st
}

func TestExecutionPlanEvidenceSelectedInputs(t *testing.T) {
	for _, stage := range []string{"tdd", "integration"} {
		t.Run(stage, func(t *testing.T) {
			s, st := executionAt(t, stage)
			g, _, _ := s.startState(st)
			if g.Status != "pass" {
				t.Fatalf("omitted predecessor still required: %s", g.Summary)
			}
			name := s.documentPath(st, "Requirements")
			boundaryFile(t, s, name, "changed")
			g, _, _ = s.startState(st)
			if g.Status != "fail" {
				t.Fatal("accepted requirements substitution passed")
			}
		})
	}
}

func TestExecutionPlanEvidenceAcceptedRuns(t *testing.T) {
	s, st := executionAt(t, "tdd")
	st.ExecutionPlan.Approved.Steps[2].Status = "completed"
	st.ExecutionPlan.Approved.Steps = append(st.ExecutionPlan.Approved.Steps, ExecutionStep{ID: "s04", Stage: "tdd", Status: "pending"})
	st.CurrentStepID = "s04"
	st.ExecutionPlan.NextID = 5
	st.Accepted["s03"] = StageAcceptance{StepID: "s03", Stage: "tdd", ReviewTarget: strings.Repeat("b", 64)}
	fixturePlanApproval(t, &st)
	if err := s.persist(st); err != nil {
		t.Fatalf("accepted execution IDs rejected: %v", err)
	}
	candidate := st
	candidate.Entry = &StageEntry{StepID: "s03", Stage: "tdd"}
	if err := s.persist(candidate); err == nil {
		t.Fatal("past same-stage entry reused")
	}
	candidate = st
	candidate.Accepted = map[string]StageAcceptance{"s04": {StepID: "s03", Stage: "tdd", ReviewTarget: strings.Repeat("b", 64)}}
	if err := s.persist(candidate); err == nil {
		t.Fatal("acceptance relabelled for new run")
	}
}

func TestExecutionPlanEvidenceInitialSensor(t *testing.T) {
	s := executionFixture(t)
	st, err := s.Create("setup")
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	g, err := s.Check(st.ID)
	if err != nil || g.Status != "pass" || g.StepID != "s01" {
		t.Fatalf("setup Sensor demanded code work: %+v %v", g, err)
	}
}

func TestExecutionPlanEvidenceDeclarations(t *testing.T) {
	s, st := executionAt(t, "tdd")
	if err := s.persist(st); err != nil {
		t.Fatal(err)
	}
	doc := declaredDoc("tdd", "codekb/output", "Knowledge")
	doc.StepID = "s02"
	if _, err := s.SetDocuments(st.ID, st.Revision, IntentDocuments{Inputs: []DocumentDeclaration{}, Outputs: []DocumentDeclaration{doc}}); err == nil {
		t.Fatal("document stage/step mismatch accepted")
	}
	doc.StepID = "s03"
	if _, err := s.SetDocuments(st.ID, st.Revision, IntentDocuments{Inputs: []DocumentDeclaration{}, Outputs: []DocumentDeclaration{doc}}); err != nil {
		t.Fatalf("current declaration rejected: %v", err)
	}
}

func TestExecutionPlanEvidenceUnitOperation(t *testing.T) {
	s, st := executionAt(t, "tdd")
	if err := s.persist(st); err != nil {
		t.Fatal(err)
	}
	_, err := assignmentUnit(t, s, st.ID, st.Revision, UnitRequest{StepID: "s02", Action: "claim", Unit: "u"})
	if err == nil || !strings.Contains(err.Error(), "execution mismatch") {
		t.Fatalf("Unit operation did not bind execution: %v", err)
	}
}

func TestExecutionPlanEvidenceSelectedEnd(t *testing.T) {
	for _, stage := range []string{"discovery", "architecture-analysis", "tdd", "integration"} {
		t.Run(stage, func(t *testing.T) {
			s := executionFixture(t)
			// The selected procedure has no declared outputs; mandatory Sensor evidence
			// is checked independently from optional document declarations.
			body := "---\nstage_id: " + stage + "\nagents: []\ninputs: []\noutputs: []\nsensors:\n  start: " + stage + "-start\n  end: " + stage + "-end\n---\n# Selected procedure\nInspect.\n"
			boundaryFile(t, s, "aidlc/workflow/stages/"+stage+".md", body)
			st, err := s.Create("selected end")
			if err != nil {
				t.Fatal(err)
			}
			st.Stage = stage
			st.CurrentStepID = "s03"
			st.ExecutionPlan = ExecutionPlan{Revision: 1, NextID: 4, Approved: &PlanVersion{Revision: 1, Reason: "selected", Steps: []ExecutionStep{{ID: "s01", Stage: "initialization", Status: "completed"}, {ID: "s02", Stage: "discovery", Status: "completed"}, {ID: "s03", Stage: stage, Status: "active"}}}}
			if stage == "discovery" {
				st.CurrentStepID = "s02"
				st.ExecutionPlan.Approved.Steps = st.ExecutionPlan.Approved.Steps[:2]
				st.ExecutionPlan.Approved.Steps[1].Status = "active"
			}
			name := boundaryDoc(t, s, st, "Requirements")
			st.Accepted = map[string]StageAcceptance{"s02": {StepID: "s02", Stage: "discovery", Outputs: []FileVersion{boundaryVersion(t, s, name)}}}
			st.Config.NoMaterialsReason = "none"
			if stage == "architecture-analysis" {
				boundaryDoc(t, s, st, "CurrentAnalysis")
				boundaryDoc(t, s, st, "Architecture")
			}
			g, inputs, sources := s.startState(st)
			if g.Status != "pass" {
				t.Fatal(g.Summary)
			}
			st.Entry = &StageEntry{StepID: st.CurrentStepID, Stage: stage, Inputs: inputs, Sources: sources}
			c := s.endDocuments(st)
			for _, failure := range c.failures {
				if strings.Contains(failure, "ImplementationPlan") || strings.Contains(failure, "CurrentAnalysis") || strings.Contains(failure, "Architecture") || strings.Contains(failure, "feature Knowledge") {
					t.Fatalf("unselected document required: %s", failure)
				}
			}
			if stage == "architecture-analysis" {
				if err := os.Remove(filepath.Join(s.Root, s.documentPath(st, "Architecture"))); err != nil {
					t.Fatal(err)
				}
				c = s.endDocuments(st)
				if len(c.failures) == 0 {
					t.Fatal("analysis Sensor omitted diagram")
				}
			}
		})
	}
}

func TestExecutionPlanEvidenceResultRun(t *testing.T) {
	s, st := executionAt(t, "tdd")
	flowGit(t, s.Root, "init", "-q")
	flowGit(t, s.Root, "commit", "--allow-empty", "-qm", "base")
	head := flowGit(t, s.Root, "rev-parse", "HEAD")
	st.Config.DirectCommit = strings.TrimSpace(head)
	st.Config.Tests = []string{"go test ./target"}
	st.Config.TestResults = []string{"aidlc/evidence/result.json"}
	boundaryFile(t, s, "aidlc/evidence/output.txt", "ok")
	zero := 0
	result := resultDocument{StepID: "s03", Stage: "tdd", Runs: []resultRun{{Command: "go test ./target", Commit: st.Config.DirectCommit, ExitCode: &zero, OutputPath: "aidlc/evidence/output.txt"}}}
	for _, id := range []string{"s03", "s02", ""} {
		result.StepID = id
		raw, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		boundaryFile(t, s, "aidlc/evidence/result.json", string(raw))
		c := boundaryCollector{store: s}
		c.results(st)
		if id == "s03" && len(c.failures) > 0 {
			t.Fatalf("current result failed: %v", c.failures)
		}
		if id != "s03" && len(c.failures) == 0 {
			t.Fatal("past or unbound result passed")
		}
	}
}

func TestExecutionPlanEvidenceSensorTarget(t *testing.T) {
	s, st := executionAt(t, "tdd")
	flowGit(t, s.Root, "init", "-q")
	flowGit(t, s.Root, "commit", "--allow-empty", "-qm", "base")
	first, err := s.checkState(st)
	if err != nil {
		t.Fatal(err)
	}
	st.ExecutionPlan.Approved.Steps[2].Status = "completed"
	st.ExecutionPlan.Approved.Steps = append(st.ExecutionPlan.Approved.Steps, ExecutionStep{ID: "s04", Stage: "tdd", Status: "pending"})
	st.CurrentStepID = "s04"
	st.ExecutionPlan.NextID = 5
	second, err := s.checkState(st)
	if err != nil {
		t.Fatal(err)
	}
	if first.StepID != "s03" || second.StepID != "s04" || first.Target == second.Target {
		t.Fatalf("Sensor lost execution identity: %+v %+v", first, second)
	}
}

func TestExecutionPlanEvidenceInitialReview(t *testing.T) {
	s := executionFixture(t)
	st, err := s.Create("setup review")
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	reviewer := t.TempDir()
	st, err = s.Review(st.ID, st.Revision, ReviewRequest{Action: "assign", Root: reviewer, Session: "reviewer", CoordinatorSession: "coordinator"})
	if err != nil {
		t.Fatalf("initialization review requires unrelated code checkout: %v", err)
	}
	if st.Review.StepID != "s01" {
		t.Fatal("review assignment lost execution")
	}
	st, err = s.Review(st.ID, st.Revision, ReviewRequest{Action: "accept", Root: reviewer, Session: "reviewer", Target: st.Review.Target, Status: "pass", Summary: "settings inspected"})
	if err != nil {
		t.Fatal(err)
	}
	if st.Review.StepID != "s01" || st.Review.Status != "pass" {
		t.Fatal("review result lost execution")
	}
}

func TestExecutionPlanEvidenceOptionalPlan(t *testing.T) {
	s, st := executionAt(t, "tdd")
	flowGit(t, s.Root, "init", "-q")
	flowGit(t, s.Root, "commit", "--allow-empty", "-qm", "base")
	g, err := s.checkState(st)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(g.Summary, "implementation plan required") {
		t.Fatal("omitted planning still requires separate plan")
	}
	if !strings.Contains(g.Summary, "scope required") || !strings.Contains(g.Summary, "acceptance required") || !strings.Contains(g.Summary, "verification required") {
		t.Fatal("omission removed implementation quality conditions")
	}
}

func TestExecutionPlanEvidenceCurrentDocuments(t *testing.T) {
	s, st := executionAt(t, "tdd")
	analysis := boundaryDoc(t, s, st, "CurrentAnalysis")
	g, inputs, sources := s.startState(st)
	if g.Status != "pass" {
		t.Fatal(g.Summary)
	}
	st.Entry = &StageEntry{StepID: st.CurrentStepID, Stage: st.Stage, Inputs: inputs, Sources: sources}
	raw, err := os.ReadFile(filepath.Join(s.Root, analysis))
	if err != nil {
		t.Fatal(err)
	}
	boundaryFile(t, s, analysis, string(raw)+"\nUpdated understanding.\n")
	if err = s.checkWorkState(st); err != nil {
		t.Fatalf("shared current update rejected: %v", err)
	}
	doc := declaredDoc("tdd", "codekb/future", "Knowledge")
	doc.StepID = "s04"
	st.Config.DocumentOutputs = []DocumentDeclaration{doc}
	c := boundaryCollector{store: s, outputs: true}
	c.references(st, []workflow.Reference{{Declared: "intent_documents"}})
	if len(c.failures) != 0 || len(c.files) != 0 {
		t.Fatalf("future same-stage document used: %v", c.failures)
	}
}

// Synthetic persisted state for tests of evidence after an already approved plan.
func fixturePlanApproval(t *testing.T, st *State) {
	t.Helper()
	p := st.ExecutionPlan.Approved
	if p == nil {
		return
	}
	a, err := newApproval(*st, PlanHash(*p), p.Revision, PlanHash(*p))
	if err != nil {
		t.Fatal(err)
	}
	a.Status = "approved"
	a.Session = "fixture-user"
	a.Turn = "fixture-turn"
	a.Quote = "approved fixture"
	a.PromptHash = strings.Repeat("c", 64)
	p.Approval = a
}

func TestExecutionPlanEvidenceStartGateStep(t *testing.T) {
	s := executionFixture(t)
	st, err := s.Create("start binding")
	if err != nil {
		t.Fatal(err)
	}
	gate, _, _ := s.startState(st)
	if gate.StepID != st.CurrentStepID {
		t.Fatalf("start gate step=%q want %q", gate.StepID, st.CurrentStepID)
	}
}
func TestExecutionPlanSchemaEncodedLimit(t *testing.T) {
	s := flowStore(t)
	st := schemaPlanState(s)
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	st.Name += strings.Repeat("a", filestore.MaxBytes-len(raw))
	writes := 0
	s.write = func(root, name string, raw []byte) error { writes++; return nil }
	if err := s.persist(st); err == nil || writes != 0 {
		t.Fatalf("oversize newline was not prevalidated: err=%v writes=%d", err, writes)
	}
}

func TestExecutionPlanSchemaMandatoryIdentity(t *testing.T) {
	s := flowStore(t)
	st := schemaPlanState(s)
	st.ExecutionPlan.NextID = 4
	st.ExecutionPlan.Bootstrap[0].ID = "s03"
	st.CurrentStepID = "s03"
	if err := s.persist(st); err == nil {
		t.Fatal("arbitrary mandatory initialization replacement accepted")
	}
}

func TestExecutionPlanEvidenceArtifactSelectedOrder(t *testing.T) {
	s, st := executionAt(t, "tdd")
	flowGit(t, s.Root, "init", "-q")
	flowGit(t, s.Root, "commit", "--allow-empty", "-qm", "base")
	st.ExecutionPlan.Approved.Steps = append(st.ExecutionPlan.Approved.Steps, ExecutionStep{ID: "s04", Stage: "architecture-analysis", Status: "pending"})
	st.ExecutionPlan.NextID = 5
	st.Config.Artifacts = []Artifact{{Stage: "architecture-analysis", Kind: "Knowledge", Path: "aidlc/spaces/default/knowledge/codekb/future.md"}}
	gate, err := s.checkState(st)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(gate.Summary, "unknown artifact stage") || strings.Contains(gate.Summary, "future.md") {
		t.Fatalf("future selected artifact required: %s", gate.Summary)
	}
}

func TestExecutionPlanEvidenceADRPreviousExecution(t *testing.T) {
	s, st := executionAt(t, "tdd")
	name := "aidlc/spaces/default/knowledge/adr/previous.md"
	boundaryFile(t, s, name, "---\ntype: adr\ntitle: Previous\ndescription: Decision\n---\nDecision reason.\n")
	st.Config.ADR.Required = true
	st.Config.DocumentInputs = []DocumentDeclaration{{StepID: "s02", Stage: "discovery", Path: name, Metadata: okfmemory.DocumentMatch{Type: "adr"}}}
	got := strings.Join(s.endDocuments(st).failures, ";")
	if strings.Contains(got, "declared adr required") {
		t.Fatalf("preceding adopted ADR ignored: %s", got)
	}
}
