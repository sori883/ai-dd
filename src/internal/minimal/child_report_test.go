package minimal

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/assignment"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/flow"
)

func bindReportChild(t *testing.T, s Service, st flow.State, task, role, parent string) {
	t.Helper()
	store := assignment.Store{Root: s.Root}
	_, err := store.PreSpawn(assignment.DispatchRequest{Session: "session", Turn: "turn", ToolID: task, TaskName: task, Agent: role, Space: "default", IntentID: st.ID, StepID: st.CurrentStepID, DefinitionHash: st.DefinitionHash})
	if err != nil {
		t.Fatal(err)
	}
	response, err := json.Marshal(map[string]string{"task_name": parent + "/" + task})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PostSpawn("session", task, response); err != nil {
		t.Fatal(err)
	}
}

func rewriteReportRegistry(t *testing.T, s Service, mutate func(*assignment.Registry)) {
	t.Helper()
	r, err := (assignment.Store{Root: s.Root}).Read()
	if err != nil {
		t.Fatal(err)
	}
	mutate(&r)
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Root, "aidlc/.runtime/assignments/registry.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestChildReportBoundary(t *testing.T) {
	for _, mode := range []string{"active_worker", "released_worker", "pending_release", "owner_mismatch"} {
		t.Run(mode, func(t *testing.T) {
			s, st := childWorkerFixture(t, true)
			store := assignment.Store{Root: s.Root}
			r, err := store.Init(assignment.InitRequest{RequestID: "init", HumanConfirmed: true, Reason: "isolated test"})
			if err != nil {
				t.Fatal(err)
			}
			workerRoot := t.TempDir()
			if out, err := exec.CommandContext(t.Context(), "git", "init", "--quiet", workerRoot).CombinedOutput(); err != nil {
				t.Fatalf("worker git init: %v %s", err, out)
			}
			v, err := store.Reserve(assignment.ReserveRequest{RegistryEpoch: r.Epoch, RequestID: "reserve", CoordinatorSession: "session", Session: "worker-session", Space: "default", IntentID: st.ID, StepID: st.CurrentStepID, DefinitionHash: st.DefinitionHash, Root: workerRoot, Agent: "aidlc-worker", SourceRevision: st.Revision})
			if err != nil {
				t.Fatal(err)
			}
			_, err = store.PreSpawn(assignment.DispatchRequest{Session: "session", Turn: "turn", ToolID: "worker", TaskName: v.TaskName, Agent: "aidlc-worker", Space: "default", IntentID: st.ID, StepID: st.CurrentStepID, DefinitionHash: st.DefinitionHash})
			if err != nil {
				t.Fatal(err)
			}
			if mode != "pending_release" {
				raw, _ := json.Marshal(map[string]string{"task_name": "/root/" + v.TaskName})
				if _, err := store.PostSpawn("session", "worker", raw); err != nil {
					t.Fatal(err)
				}
			}
			release := func() {
				current, err := store.Read()
				if err != nil {
					t.Fatal(err)
				}
				reservation := current.Reservations[0]
				if _, err := store.Release(v.ID, "session", reservation.EntryRevision, assignment.ReleaseRequest{RegistryEpoch: r.Epoch, RequestID: "release", PreviousRunStopped: true, NoMoreRequests: true, Reason: "test process ended"}); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "released_worker" {
				release()
			}
			if mode == "owner_mismatch" {
				rewriteReportRegistry(t, s, func(r *assignment.Registry) {
					r.Reservations[0].CoordinatorSession = "other"
					raw, _ := json.Marshal(r.Reservations[0].ReserveRequest)
					r.Reservations[0].RequestHash = filestore.Hash(raw)
				})
			}
			waited := false
			s.hookClock = &hookClock{now: time.Now, wait: func(time.Duration) {
				if mode != "pending_release" || waited {
					t.Fatal("unexpected worker wait")
				}
				waited = true
				release()
			}}
			before := childSnapshot(t, s.Root)
			in := childInput(t, "PreToolUse", "aidlc-worker", "send_message", "")
			in.Input.Target = "/root"
			out, err := s.Hook(in)
			if err != nil || deny(out) != (mode != "active_worker") {
				t.Fatalf("worker report %s: %+v %v", mode, out, err)
			}
			if mode == "pending_release" {
				if !waited {
					t.Fatal("did not observe release during wait")
				}
			} else if !reflect.DeepEqual(before, childSnapshot(t, s.Root)) {
				t.Fatal("report changed worker registry")
			}
		})
	}
	for _, tc := range []struct {
		name         string
		mutate       func(*assignment.Registry)
		target, role string
	}{
		{"relative", nil, "phase", "aidlc-reviewer"},
		{"ancestor", nil, "/root", "aidlc-reviewer"},
		{"sibling", nil, "/root/phase/sibling", "aidlc-reviewer"},
		{"missing_target", nil, "", "aidlc-reviewer"},
		{"missing_role", nil, "/root/phase", ""},
		{"unbound_role", nil, "/root/phase", "aidlc-stage-planner"},
		{"wrong_session", func(r *assignment.Registry) { r.Dispatches[0].Session = "other" }, "/root/phase", "aidlc-reviewer"},
		{"wrong_space", func(r *assignment.Registry) { r.Dispatches[0].Space = "other" }, "/root/phase", "aidlc-reviewer"},
		{"wrong_step", func(r *assignment.Registry) { r.Dispatches[0].StepID = "other" }, "/root/phase", "aidlc-reviewer"},
		{"wrong_definition", func(r *assignment.Registry) { r.Dispatches[0].DefinitionHash = strings.Repeat("a", 64) }, "/root/phase", "aidlc-reviewer"},
		{"uncertain", func(r *assignment.Registry) { r.Dispatches[0].Status = "uncertain" }, "/root/phase", "aidlc-reviewer"},
		{"dirty_path", func(r *assignment.Registry) { r.Dispatches[0].Canonical = "/root/phase/../review" }, "/root/phase", "aidlc-reviewer"},
		{"wrong_leaf", func(r *assignment.Registry) { r.Dispatches[0].Canonical = "/root/phase/other" }, "/root/phase", "aidlc-reviewer"},
		{"wrong_namespace", func(r *assignment.Registry) { r.Dispatches[0].Canonical = "/other/phase/review" }, "/root/phase", "aidlc-reviewer"},
		{"oversized", func(r *assignment.Registry) {
			r.Dispatches[0].Canonical = "/root/" + strings.Repeat("a", 513) + "/review"
		}, "/root/phase", "aidlc-reviewer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, st := agentFixture(t)
			bindReportChild(t, s, st, "review", "aidlc-reviewer", "/root/phase")
			if tc.mutate != nil {
				rewriteReportRegistry(t, s, tc.mutate)
			}
			s.hookClock = &hookClock{now: time.Now, wait: func(time.Duration) { t.Fatal("waited for an invalid or non-pending report") }}
			before := childSnapshot(t, s.Root)
			in := childInput(t, "PreToolUse", tc.role, "send_message", "")
			in.Input.Target = tc.target
			out, err := s.Hook(in)
			if err != nil || !deny(out) {
				t.Fatalf("unsafe report accepted: %+v %v", out, err)
			}
			if !reflect.DeepEqual(before, childSnapshot(t, s.Root)) {
				t.Fatal("denial changed shared state")
			}
		})
	}
	for _, mode := range []string{"contradictory_parent", "uncertain_with_bound"} {
		t.Run(mode, func(t *testing.T) {
			s, st := agentFixture(t)
			bindReportChild(t, s, st, "review", "aidlc-reviewer", "/root/phase")
			parent := "/root/other"
			if mode == "uncertain_with_bound" {
				parent = "/root/phase"
			}
			bindReportChild(t, s, st, "plan", "aidlc-stage-planner", parent)
			if mode == "uncertain_with_bound" {
				rewriteReportRegistry(t, s, func(r *assignment.Registry) { r.Dispatches[1].Status = "uncertain" })
			}
			in := childInput(t, "PreToolUse", "aidlc-reviewer", "send_message", "")
			in.Input.Target = "/root/phase"
			out, err := s.Hook(in)
			if err != nil || !deny(out) {
				t.Fatalf("ambiguous report accepted: %+v %v", out, err)
			}
		})
	}
	for _, mode := range []string{"bound_after_wait", "bound_other_role", "bound_other_target", "timeout", "binding_changed", "stage_changed", "reset"} {
		t.Run(mode, func(t *testing.T) {
			s, st := agentFixture(t)
			reg := assignment.Store{Root: s.Root}
			if mode == "bound_other_role" || mode == "bound_other_target" {
				bindReportChild(t, s, st, "plan", "aidlc-stage-planner", "/root")
			}
			_, err := reg.PreSpawn(assignment.DispatchRequest{Session: "session", Turn: "turn", ToolID: "review", TaskName: "review", Agent: "aidlc-reviewer", Space: "default", IntentID: st.ID, StepID: st.CurrentStepID, DefinitionHash: st.DefinitionHash})
			if err != nil {
				t.Fatal(err)
			}
			now := time.Unix(1, 0)
			waited := time.Duration(0)
			s.hookClock = &hookClock{now: func() time.Time { return now }, wait: func(d time.Duration) {
				waited += d
				if mode == "bound_other_target" {
					t.Fatal("waited despite different known parent target")
				}
				now = now.Add(d)
				for _, key := range []string{"session-session", "assignments"} {
					release, err := filestore.Lock(s.Root, key)
					if err != nil {
						t.Fatal("waiting with lock held", err)
					}
					if err := release(); err != nil {
						t.Fatal(err)
					}
				}
				if waited != d {
					return
				}
				switch mode {
				case "bound_after_wait", "bound_other_role":
					if _, err := reg.PostSpawn("session", "review", []byte(`{"task_name":"/root/review"}`)); err != nil {
						t.Fatal(err)
					}
				case "binding_changed":
					if err := s.save("session", Session{}); err != nil {
						t.Fatal(err)
					}
				case "stage_changed":
					st.Status = "paused"
					raw, err := json.Marshal(st)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(s.Root, "aidlc/spaces/default/intents", st.ID, "state.json"), raw, 0600); err != nil {
						t.Fatal(err)
					}
				case "reset":
					rewriteReportRegistry(t, s, func(r *assignment.Registry) { r.Epoch = strings.Repeat("b", 32); r.Dispatches = nil })
				}
			}}
			in := childInput(t, "PreToolUse", "aidlc-reviewer", "send_message", "")
			in.Input.Target = "/root"
			if mode == "bound_other_target" {
				in.Input.Target = "/root/other"
			}
			out, err := s.Hook(in)
			wantDenied := mode != "bound_after_wait" && mode != "bound_other_role"
			if err != nil || deny(out) != wantDenied || (mode != "bound_other_target" && waited == 0) {
				t.Fatalf("pending report %s: %+v %v waited=%v", mode, out, err, waited)
			}
			if mode == "timeout" && waited != 2*time.Second {
				t.Fatal("unbounded wait", waited)
			}
		})
	}
}

func TestChildReportToParent(t *testing.T) {
	for _, parent := range []string{"/root", "/root/phase"} {
		for _, tool := range []string{"send_message", "collaborationsend_message"} {
			t.Run(parent+"/"+tool, func(t *testing.T) {
				s, st := agentFixture(t)
				bindReportChild(t, s, st, "review", "aidlc-reviewer", parent)
				bindReportChild(t, s, st, "plan", "aidlc-stage-planner", parent)
				state, err := s.Inspect("session")
				if err != nil {
					t.Fatal(err)
				}
				state.Tool = "parent-running"
				state.RuleTurn = "earlier"
				if err := s.save("session", state); err != nil {
					t.Fatal(err)
				}
				before := childSnapshot(t, s.Root)
				in := childInput(t, "PreToolUse", "aidlc-reviewer", tool, "")
				in.Input.Target = parent
				out, err := s.Hook(in)
				if err != nil || deny(out) {
					t.Fatalf("eligible child's parent report denied: %+v %v", out, err)
				}
				in.Event = "PostToolUse"
				out, err = s.Hook(in)
				if err != nil || len(out) != 0 {
					t.Fatal("report Post changed parent", out, err)
				}
				if after := childSnapshot(t, s.Root); !reflect.DeepEqual(before, after) {
					t.Fatal("report changed shared bytes")
				}
			})
		}
	}
}
