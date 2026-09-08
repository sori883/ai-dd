//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/flow"
	"github.com/sori883/ai-dd/src/internal/install"
	"github.com/sori883/ai-dd/src/internal/minimal"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type flowLiveConfig struct {
	Root, Binary, Evidence, Helper string
	Workers                        map[string]string
}
type flowLiveJob struct {
	Kind                                  string
	Units                                 []string
	Intent, Target, Session, Root, Commit string
}
type flowLiveRecord struct {
	Review  *flowReviewObservation
	Raw     json.RawMessage
	Output  json.RawMessage
	States  []flow.State
	Session string
	Bound   bool
}

func flowLiveSave(dir, prefix string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, prefix+"-"+strconv.FormatInt(time.Now().UnixNano(), 10)+"-*.json")
	if err != nil {
		return err
	}
	_, err = file.Write(raw)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func flowLiveArgs() []string {
	for i, a := range os.Args {
		if a == "--" {
			return os.Args[i+1:]
		}
	}
	return nil
}

// TestFlowLiveHelper is test-only transport. It never changes product decisions.
func TestFlowLiveHelper(t *testing.T) {
	args := flowLiveArgs()
	if len(args) == 0 {
		t.Skip("live helper subprocess only")
	}
	if len(args) < 2 {
		t.Fatal("helper mode and config required")
	}
	var cfg flowLiveConfig
	raw, err := os.ReadFile(args[1])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	switch args[0] {
	case "hook":
		input, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(cfg.Binary, "__minimal-hook", "--project-dir", cfg.Root)
		cmd.Stdin = bytes.NewReader(input)
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		var h minimal.HookInput
		if err := json.Unmarshal(input, &h); err != nil {
			t.Fatal(err)
		}
		states, err := (flow.Store{Root: cfg.Root, Space: "default"}).List()
		if err != nil {
			t.Fatal(err)
		}
		session, _ := (minimal.Service{Root: cfg.Root, Binary: cfg.Binary}).Inspect(h.Session)
		record := flowLiveRecord{Raw: input, Output: out, States: states, Session: h.Session, Bound: session.Intent != ""}
		if h.Event == "PreToolUse" && h.Tool == "Bash" {
			r, _, ok := flowReviewCommand(cfg.Binary, h.Input.Command)
			if ok {
				file := r.File
				if !filepath.IsAbs(file) {
					file = filepath.Join(cfg.Root, file)
				}
				relative, relErr := filepath.Rel(cfg.Root, file)
				if relErr != nil {
					t.Fatal(relErr)
				}
				requestRaw, readErr := filestore.ReadFile(cfg.Root, filepath.ToSlash(relative))
				if readErr != nil {
					t.Fatal(readErr)
				}
				var request flow.ReviewRequest
				if err := json.Unmarshal(requestRaw, &request); err != nil {
					t.Fatal(err)
				}
				current, readErr := (flow.Store{Root: cfg.Root, Space: r.Space}).Read(r.Target)
				if readErr != nil {
					t.Fatal(readErr)
				}
				record.Review = &flowReviewObservation{Session: h.Session, Command: h.Input.Command, Before: current.Revision, Request: request}
			}
		}
		if err := flowLiveSave(cfg.Evidence, "hook", record); err != nil {
			t.Fatal(err)
		}
		fmt.Print(string(out))
		os.Exit(0)
	case "request":
		if len(args) != 3 {
			t.Fatal("request JSON file required")
		}
		raw, err := os.ReadFile(args[2])
		if err != nil {
			t.Fatal(err)
		}
		var job flowLiveJob
		if err := json.Unmarshal(raw, &job); err != nil {
			t.Fatal(err)
		}
		queue := filepath.Join(cfg.Root, ".flow-queue")
		if err := os.MkdirAll(queue, 0700); err != nil {
			t.Fatal(err)
		}
		file, err := os.CreateTemp(queue, "job-*.pending")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(raw); err != nil {
			t.Fatal(err)
		}
		file.Close()
		requestPath := strings.TrimSuffix(file.Name(), ".pending") + ".json"
		if err := os.Rename(file.Name(), requestPath); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(12 * time.Minute)
		for time.Now().Before(deadline) {
			result, err := os.ReadFile(requestPath + ".result")
			if err == nil {
				fmt.Print(string(result))
				os.Exit(0)
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Fatal("host job timed out")
	case "test":
		if len(args) != 4 {
			t.Fatal("test requires Unit and test name")
		}
		root, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		unit, name := args[2], args[3]
		source, err := os.ReadFile(filepath.Join(root, unit+".go"))
		if err != nil {
			t.Fatal(err)
		}
		testBytes, err := os.ReadFile(filepath.Join(root, unit+"_test.go"))
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("go", "test", "-json", "-count=1", "-run", "^"+name+"$")
		cmd.Dir = root
		out, runErr := cmd.CombinedOutput()
		code := 0
		if runErr != nil {
			exit, ok := runErr.(*exec.ExitError)
			if !ok {
				t.Fatal(runErr)
			}
			code = exit.ExitCode()
		}
		ran := false
		for _, line := range bytes.Split(out, []byte("\n")) {
			var event struct{ Action, Test string }
			if json.Unmarshal(line, &event) == nil && event.Action == "run" && event.Test == name {
				ran = true
			}
		}
		proof := flowProofTest{Name: name, TestHash: filestore.Hash(testBytes), SourceHash: filestore.Hash(source), Exit: code, Ran: ran}
		dir := filepath.Join(root, ".flow-proof")
		os.MkdirAll(dir, 0700)
		flowLiveSave(dir, "test", proof)
		flowLiveSave(dir, "raw", map[string]any{"stdout": string(out), "source": string(source), "test": string(testBytes), "exit": code})
		fmt.Print(string(out))
		os.Exit(code)
	default:
		t.Fatal("unknown helper mode")
	}
}
func flowRunModel(ctx context.Context, cfg flowLiveConfig, root, label, prompt, sandbox string, resume ...string) (flowProofJob, error) {
	job := flowProofJob{Root: root, Start: time.Now().UnixNano()}
	last := filepath.Join(cfg.Evidence, label+"-last.json")
	args := []string{"exec", "--ignore-user-config", "--dangerously-bypass-hook-trust", "-s", sandbox, "-c", `approval_policy="never"`, "-m", "gpt-6-astra", "-c", `model_reasoning_effort="medium"`, "-c", minimalProbeTrustConfig(root), "-C", root, "--json", "-o", last, prompt}
	if len(resume) > 0 {
		args = []string{"exec", "resume", "--ignore-user-config", "--dangerously-bypass-hook-trust", "-c", `sandbox_mode="` + sandbox + `"`, "-c", `approval_policy="never"`, "-m", "gpt-6-astra", "-c", `model_reasoning_effort="medium"`, "-c", minimalProbeTrustConfig(root), "--json", "-o", last, resume[0], prompt}
	}
	if strings.HasPrefix(label, "review-") {
		schema := filepath.Join(cfg.Evidence, "review-schema.json")
		if err := os.WriteFile(schema, []byte(`{"type":"object","properties":{"target":{"type":"string"},"status":{"type":"string","enum":["pass","fail"]},"summary":{"type":"string"}},"required":["target","status","summary"],"additionalProperties":false}`), 0600); err != nil {
			return job, err
		}
		args = append(args[:len(args)-1], append([]string{"--output-schema", schema}, args[len(args)-1:]...)...)
	}
	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Dir = root
	stdout, err := os.Create(filepath.Join(cfg.Evidence, label+".jsonl"))
	if err != nil {
		return job, err
	}
	defer stdout.Close()
	stderr, err := os.Create(filepath.Join(cfg.Evidence, label+".stderr"))
	if err != nil {
		return job, err
	}
	defer stderr.Close()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	job.End = time.Now().UnixNano()
	raw, _ := os.ReadFile(filepath.Join(cfg.Evidence, label+".jsonl"))
	for _, line := range bytes.Split(raw, []byte("\n")) {
		var event struct {
			Type string `json:"type"`
			ID   string `json:"thread_id"`
		}
		if json.Unmarshal(line, &event) == nil && event.Type == "thread.started" {
			job.Session = event.ID
		}
	}
	return job, err
}
func flowHostGit(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=Flow", "-c", "user.email=flow@example.invalid"}, args...)...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
func flowHostJob(ctx context.Context, cfg flowLiveConfig, request flowLiveJob) (any, []flowProofJob, error) {
	stamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	switch request.Kind {
	case "prepare":
		result := map[string]any{}
		for _, unit := range request.Units {
			root, ok := cfg.Workers[unit]
			if !ok {
				return nil, nil, fmt.Errorf("unknown worker")
			}
			head, err := flowHostGit(cfg.Root, "rev-parse", "HEAD")
			if err != nil {
				return nil, nil, err
			}
			if _, err := flowHostGit(root, "reset", "--hard", head); err != nil {
				return nil, nil, err
			}
			job, err := flowRunModel(ctx, cfg, root, "prepare-"+unit+stamp, "Wait for the next message with your assignment. Do not inspect or change files yet. Reply ready.", "workspace-write")
			if err != nil {
				return nil, nil, err
			}
			result[unit] = map[string]string{"root": root, "session": job.Session, "base_commit": head}
		}
		return result, nil, nil
	case "prepare-review":
		root := filepath.Join(cfg.Evidence, "review-"+stamp)
		if _, err := flowHostGit(cfg.Root, "worktree", "add", "--detach", root, "HEAD"); err != nil {
			return nil, nil, err
		}
		job, err := flowRunModel(ctx, cfg, root, "prepare-review-"+stamp, "You are an independent read-only reviewer. Wait for a fixed target; do not inspect yet. Reply ready.", "read-only")
		return map[string]string{"root": root, "session": job.Session}, nil, err
	case "workers":
		var wg sync.WaitGroup
		var mu sync.Mutex
		var jobs []flowProofJob
		var firstErr error
		for _, unit := range request.Units {
			wg.Go(func() {
				root, ok := cfg.Workers[unit]
				if !ok {
					mu.Lock()
					firstErr = fmt.Errorf("unknown worker")
					mu.Unlock()
					return
				}
				raw, err := os.ReadFile(filepath.Join(cfg.Root, "aidlc/.runtime/flow/units/default", request.Intent, unit+".json"))
				var assignment flow.UnitRequest
				if err == nil {
					err = json.Unmarshal(raw, &assignment)
				}
				if err != nil || assignment.Root != root {
					mu.Lock()
					firstErr = fmt.Errorf("worker assignment missing or wrong: %v", err)
					mu.Unlock()
					return
				}
				task := map[string]string{"a": "Add(a,b int) int returning a+b; TestAdd checks 2+3=5 and 0+4=4", "b": "Mul(a,b int) int returning a*b; TestMul checks 2*3=6 and 0*4=0", "c": "Combine(a,b int) int returning Add(a,b)+Mul(a,b); TestCombine checks 2,3 => 11"}[unit]
				name := map[string]string{"a": "TestAdd", "b": "TestMul", "c": "TestCombine"}[unit]
				prompt := "Implement " + task + " in " + unit + ".go and " + unit + "_test.go only, package calc. First add the real failing test against the existing zero-return scaffold. Run " + cfg.Helper + " test " + unit + " " + name + " to observe RED. Implement and run the identical test again for GREEN. Do not alter a test to make it pass. Do not commit; the fixture host commits exact verified bytes. Read the deployed Rules at " + filepath.Join(cfg.Root, "aidlc/spaces/default/knowledge/rules/rule.md") + ". Your Unit assignment is " + string(raw) + ". Return the outcome after tests finish."
				job, err := flowRunModel(ctx, cfg, root, "worker-"+unit+stamp, prompt, "workspace-write", assignment.Session)
				job.Kind = "worker"
				job.Unit = unit
				if err == nil {
					files, _ := filepath.Glob(filepath.Join(root, ".flow-proof/test-*.json"))
					sort.Strings(files)
					for _, file := range files {
						raw, _ := os.ReadFile(file)
						var proof flowProofTest
						if json.Unmarshal(raw, &proof) == nil {
							job.Tests = append(job.Tests, proof)
						}
					}
					currentSource, readErr := os.ReadFile(filepath.Join(root, unit+".go"))
					if readErr != nil {
						err = readErr
					}
					currentTest, readErr := os.ReadFile(filepath.Join(root, unit+"_test.go"))
					if readErr != nil {
						err = readErr
					}
					verified := false
					for _, p := range job.Tests {
						if p.Ran && p.Exit == 0 && p.SourceHash == filestore.Hash(currentSource) && p.TestHash == filestore.Hash(currentTest) {
							verified = true
						}
					}
					if !verified {
						err = fmt.Errorf("worker bytes have no matching real GREEN")
					}
					if err == nil {
						_, err = flowHostGit(root, "add", unit+".go", unit+"_test.go")
					}
					if err == nil {
						_, err = flowHostGit(root, "commit", "-m", "verified Unit "+unit)
					}
					if err == nil {
						job.Commit, err = flowHostGit(root, "rev-parse", "HEAD")
					}
				}
				mu.Lock()
				defer mu.Unlock()
				jobs = append(jobs, job)
				if err != nil {
					firstErr = err
				}
			})
		}
		wg.Wait()
		return jobs, jobs, firstErr
	case "integrate":
		if len(request.Commit) != 40 {
			return nil, nil, fmt.Errorf("commit required")
		}
		if _, err := flowHostGit(cfg.Root, "merge", "--no-edit", request.Commit); err != nil {
			return nil, nil, err
		}
		head, err := flowHostGit(cfg.Root, "rev-parse", "HEAD")
		return map[string]string{"commit": head}, nil, err
	case "review":
		st, err := (flow.Store{Root: cfg.Root, Space: "default"}).Read(request.Intent)
		if err != nil {
			return nil, nil, err
		}
		gate, err := (flow.Store{Root: cfg.Root, Space: "default"}).Check(st.ID)
		if err != nil {
			return nil, nil, err
		}
		if gate.Target != request.Target || !strings.HasPrefix(request.Root, cfg.Evidence+string(os.PathSeparator)+"review-") {
			return nil, nil, fmt.Errorf("review target/root mismatch")
		}
		stateRaw, _ := json.Marshal(st)
		prompt := "Read-only independent review of fixed target " + request.Target + ". Read actual code in this checkout, and the current artifacts and complete Rules in " + cfg.Root + ". State: " + string(stateRaw) + ". Check current stage acceptance, contradiction in Knowledge, ADR need/reason, real test evidence, and Unit integration. Return ONLY JSON {\"target\":\"" + request.Target + "\",\"status\":\"pass\" or \"fail\",\"summary\":\"specific findings with paths and evidence\"}. Do not mutate anything or trust coordinator self-report."
		reviewCommit, err := flowHostGit(request.Root, "rev-parse", "HEAD")
		if err != nil {
			return nil, nil, err
		}
		codeHash, err := flowLiveCodeHash(request.Root)
		if err != nil {
			return nil, nil, err
		}
		job, err := flowRunModel(ctx, cfg, request.Root, "review-"+stamp, prompt, "read-only", request.Session)
		job.Kind = "review"
		job.Commit = reviewCommit
		job.CodeHash = codeHash
		afterHash, hashErr := flowLiveCodeHash(request.Root)
		if hashErr != nil || afterHash != codeHash {
			return nil, nil, fmt.Errorf("review checkout changed during review: %v", hashErr)
		}
		job.Target = request.Target
		raw, _ := os.ReadFile(filepath.Join(cfg.Evidence, "review-"+stamp+"-last.json"))
		var report struct{ Target, Status, Summary string }
		if err == nil {
			err = json.Unmarshal(raw, &report)
		}
		if err == nil && report.Target != request.Target {
			err = fmt.Errorf("reviewer target mismatch")
		}
		job.Status = report.Status
		job.Summary = report.Summary
		return report, []flowProofJob{job}, err
	case "commit":
		if _, err := flowHostGit(cfg.Root, "add", "--all"); err != nil {
			return nil, nil, err
		}
		if _, err := flowHostGit(cfg.Root, "commit", "--allow-empty", "-m", "workflow artifacts"); err != nil {
			return nil, nil, err
		}
		head, err := flowHostGit(cfg.Root, "rev-parse", "HEAD")
		return map[string]string{"commit": head}, nil, err
	default:
		return nil, nil, fmt.Errorf("unknown host fixture operation")
	}
}

// TestFlowJourneyLive is a fresh actual-model journey. Only the parent runs it.
func TestFlowJourneyLive(t *testing.T) {
	if os.Getenv("AIDLC_FLOW_LIVE") != "1" {
		t.Skip("set AIDLC_FLOW_LIVE=1 for model evidence")
	}
	version, err := exec.Command("codex", "--version").CombinedOutput()
	if err != nil || strings.TrimSpace(string(version)) != "codex-cli 0.153.4" {
		t.Fatalf("fixed Codex unavailable: %s %v", version, err)
	}
	evidence, err := os.MkdirTemp("", "aidlc-flow-live-")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err = filepath.EvalSymlinks(evidence)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("persistent raw evidence: %s", evidence)
	root := filepath.Join(evidence, "project")
	os.MkdirAll(root, 0700)
	binary := buildMinimalBinary(t)
	helper := filepath.Join(root, "flow-helper")
	cfg := flowLiveConfig{Root: root, Binary: binary, Evidence: evidence, Helper: helper, Workers: map[string]string{}}
	runMinimalProcess(t, root, "git", "init", "-q")
	writeMinimalFixture(t, filepath.Join(root, "go.mod"), "module example.invalid/flow\n\ngo 1.26\n")
	for u, body := range map[string]string{"a": "func Add(a,b int)int{return 0}", "b": "func Mul(a,b int)int{return 0}", "c": "func Combine(a,b int)int{return 0}"} {
		writeMinimalFixture(t, filepath.Join(root, u+".go"), "package calc\n"+body+"\n")
	}
	runMinimalProcess(t, root, "git", "add", "go.mod", "a.go", "b.go", "c.go")
	runMinimalProcess(t, root, "git", "-c", "user.name=Flow", "-c", "user.email=flow@example.invalid", "commit", "-qm", "fixture scaffold")
	for _, u := range []string{"a", "b", "c"} {
		worker := filepath.Join(evidence, "worker-"+u)
		runMinimalProcess(t, root, "git", "worktree", "add", "--detach", worker, "HEAD")
		cfg.Workers[u] = worker
	}
	if _, err := install.Codex(root, binary); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, ".flow-config.json")
	raw, _ := json.Marshal(cfg)
	writeMinimalFixture(t, cfgPath, string(raw))
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	script := "#!/bin/sh\nmode=$1\nshift\nexec " + quote(testBinary) + " -test.run='^TestFlowLiveHelper$' -- \"$mode\" " + quote(cfgPath) + " \"$@\"\n"
	writeMinimalFixture(t, helper, script)
	os.Chmod(helper, 0700)
	// Hooks call the same deployed product implementation and retain its raw decisions.
	hooksPath := filepath.Join(root, ".codex/hooks.json")
	hooksRaw, _ := os.ReadFile(hooksPath)
	var hooks map[string]any
	json.Unmarshal(hooksRaw, &hooks)
	for _, groups := range hooks["hooks"].(map[string]any) {
		for _, g := range groups.([]any) {
			for _, h := range g.(map[string]any)["hooks"].([]any) {
				h.(map[string]any)["command"] = quote(helper) + " hook"
			}
		}
	}
	hooksRaw, _ = json.Marshal(hooks)
	os.WriteFile(hooksPath, hooksRaw, 0600)
	writeMinimalFixture(t, filepath.Join(root, ".gitignore"), ".flow-*\nflow-helper\n.codex/\n.agents/\n")
	writeMinimalFixture(t, filepath.Join(root, "aidlc/spaces/default/knowledge/knowledge/current.md"), "---\ntype: Design\ntitle: Arithmetic\ndescription: Current arithmetic behavior\n---\nAdd(0,x) returns 0. Mul multiplies. Combine sums Add and Mul.\n")
	runMinimalProcess(t, root, "git", "add", ".gitignore")
	runMinimalProcess(t, root, "git", "-c", "user.name=Flow", "-c", "user.email=flow@example.invalid", "commit", "-qm", "fixture exclusions")
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Minute)
	defer cancel()
	proof := flowProof{Binary: binary}
	seen := map[string]bool{}
	var coordinatorErr error
	done := make(chan flowProofJob, 1)
	prompt := `Use the installed aidlc skill and actual Rules to complete a fresh Intent for correct Add, Mul and Combine arithmetic. Requirements: Add(2,3)=5 and Add(0,4)=4; Mul(2,3)=6 and Mul(0,4)=0; Combine(2,3)=11. There is a seeded contradictory Knowledge statement: obtain an independent discovery review before correcting it, then correct and re-review. Exercise a stale-review rejection by changing a relevant Knowledge byte after a passing review is returned but before accepting it; then request a fresh review. Use ADR only if needed, otherwise review the no-ADR reason. Plan Units a and b in parallel and c dependent on both. Finish all four stages through actual CLI Sensor/review/advance; do not edit state.json.
This fixture's Git operations need host support under the normal sandbox. The test-only tool ` + helper + ` request FILE submits a JSON host job and waits; use async polling for long jobs. It does not modify workflow state. Job shapes: {"Kind":"prepare","Units":["a","b"]} returns actual worker sessions/root; {"Kind":"workers","Units":["a","b"],"Intent":"ID"} starts those assigned workers concurrently and returns actual tests/commits; {"Kind":"integrate","Commit":"HASH"} integrates a worker commit; {"Kind":"prepare-review"} returns an independent read-only root/session; {"Kind":"review","Intent":"ID","Target":"HASH","Session":"SESSION","Root":"ROOT"} runs that assigned reviewer; {"Kind":"commit"} commits fixture artifacts and returns current HEAD. The commit host job commits all nonignored fixture files. Before preparing a reviewer checkout, commit current code/test evidence and update config.code_revision to the returned HEAD. Use one literal CLI command per tool call. Keep all JSON request drafts under aidlc/.runtime/ or .flow-queue/ so they do not change code targets. You must perform all intent/unit CLI operations yourself with actual revisions. Unit scopes are a.go+a_test.go, b.go+b_test.go, c.go+c_test.go; worker base must match the prepared root HEAD. Record host returned commits through unit result/integrate. For c, prepare its root only after both dependencies integrate. Test proof files are in each worker .flow-proof; preserve relevant final evidence as an artifact. Do not claim AI performed host Git operations. Respond only after completed state. Do not bypass hooks or change Rules.`
	go func() {
		job, err := flowRunModel(ctx, cfg, root, "coordinator-one", prompt, "workspace-write")
		coordinatorErr = err
		done <- job
	}()
	runLoop := func() flowProofJob {
		for {
			select {
			case job := <-done:
				return job
			case <-ctx.Done():
				t.Fatalf("live timeout; evidence %s", evidence)
				return flowProofJob{}
			case <-time.After(100 * time.Millisecond):
				files, _ := filepath.Glob(filepath.Join(root, ".flow-queue/job-*.json"))
				sort.Strings(files)
				for _, file := range files {
					if seen[file] {
						continue
					}
					seen[file] = true
					raw, err := os.ReadFile(file)
					var request flowLiveJob
					if err == nil {
						err = json.Unmarshal(raw, &request)
					}
					var result any
					var jobs []flowProofJob
					if err == nil {
						result, jobs, err = flowHostJob(ctx, cfg, request)
					}
					proof.Jobs = append(proof.Jobs, jobs...)
					flowLiveSave(evidence, "host-job", map[string]any{"request": request, "result": result, "jobs": jobs, "error": fmt.Sprint(err)})
					out, _ := json.Marshal(map[string]any{"result": result, "error": fmt.Sprint(err)})
					os.WriteFile(file+".result", out, 0600)
				}
			}
		}
	}
	first := runLoop()
	if coordinatorErr != nil {
		t.Fatalf("first model failed: %v; %s", coordinatorErr, evidence)
	}
	proof.Sessions = append(proof.Sessions, first.Session)
	go func() {
		job, err := flowRunModel(ctx, cfg, root, "coordinator-two", "Resume the existing Intent in a new conversation through the installed skill. Inspect persisted state and Rules, reopen integration with a reason, verify its results and obtain a fresh independent review using the same fixture host tool, then finish it again. Do not create another Intent. Fixture host tool: "+helper+" request FILE; JSON {Kind:prepare-review} returns root/session, then {Kind:review,Intent:ID,Target:HASH,Session:SESSION,Root:ROOT} returns the actual review. Use valid JSON with quoted keys and values.", "workspace-write")
		coordinatorErr = err
		done <- job
	}()
	second := runLoop()
	if coordinatorErr != nil {
		t.Fatalf("second model failed: %v; %s", coordinatorErr, evidence)
	}
	proof.Sessions = append(proof.Sessions, second.Session)
	files, _ := filepath.Glob(filepath.Join(evidence, "hook-*.json"))
	sort.Strings(files)
	bound := map[string]bool{}
	var observations []flowReviewObservation
	for _, file := range files {
		raw, _ := os.ReadFile(file)
		var record flowLiveRecord
		if json.Unmarshal(raw, &record) != nil {
			t.Fatal("invalid hook record")
		}
		if record.Review != nil {
			observations = append(observations, *record.Review)
		}
		proof.States = append(proof.States, record.States...)
		var input minimal.HookInput
		if json.Unmarshal(record.Raw, &input) == nil && input.Event == "PreToolUse" && input.Tool == "Bash" {
			fields := strings.Fields(input.Input.Command)
			if len(fields) > 2 && strings.Trim(fields[0], "\"' ") == cfg.Binary && !strings.ContainsAny(input.Input.Command, ";|&\n") {
				proof.Actions = append(proof.Actions, fields[1]+" "+fields[2])
			}
		}

		if record.Bound {
			bound[record.Session] = true
			if record.Session == first.Session || record.Session == second.Session {
				for _, state := range record.States {
					proof.Progress = append(proof.Progress, flowProgress{Session: record.Session, State: state})
				}
			}
		}

	}
	for _, label := range []string{"coordinator-one", "coordinator-two"} {
		raw, err := os.ReadFile(filepath.Join(evidence, label+".jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		executions, err := flowExecutions(raw, observations, binary)
		if err != nil {
			t.Fatal(err)
		}
		proof.CLI = append(proof.CLI, executions...)
	}
	for _, e := range proof.CLI {
		if e.Exit == 2 && strings.TrimSpace(e.Output) == "aidlc: unassigned or stale review: invalid argument" {
			proof.StaleRejected = true
		}
	}
	if !bound[first.Session] || !bound[second.Session] {
		t.Fatal("both actual sessions did not bind")
	}
	flowLiveSave(evidence, "proof", proof)
	if err := verifyFlowProof(proof); err != nil {
		t.Fatalf("%v; evidence %s", err, evidence)
	}
}

func flowLiveCodeHash(root string) (string, error) {
	names, err := flowHostGit(root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return "", err
	}
	files := strings.Split(names, "\x00")
	sort.Strings(files)
	var raw []byte
	for _, name := range files {
		if name == "" || strings.HasPrefix(name, "aidlc/") {
			continue
		}
		content, err := filestore.ReadFile(root, name)
		if err != nil {
			return "", err
		}
		raw = append(raw, []byte(name+"\x00")...)
		raw = append(raw, content...)
		raw = append(raw, 0)
	}
	return filestore.Hash(raw), nil
}
