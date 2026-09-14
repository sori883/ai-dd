//go:build integration && diagnostic

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/app"
	"github.com/sori883/ai-dd/src/internal/flow"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

type flowLiveConfig struct {
	Root, Binary, Evidence, Helper string
}
type flowLiveRecord struct {
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
		cmd := observerHookCommand(context.Background(), cfg.Binary, cfg.Root)
		cmd.Stdin = bytes.NewReader(input)
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		var h app.HookInput
		if err := json.Unmarshal(input, &h); err != nil {
			t.Fatal(err)
		}
		states, err := (flow.Store{Root: cfg.Root, Space: "default"}).List()
		if err != nil {
			t.Fatal(err)
		}
		session, _ := (app.Service{Root: cfg.Root, Binary: cfg.Binary}).Inspect(h.Session)
		record := flowLiveRecord{Raw: input, Output: out, States: states, Session: h.Session, Bound: session.Intent != ""}
		if err := flowLiveSave(cfg.Evidence, "hook", record); err != nil {
			t.Fatal(err)
		}
		fmt.Print(string(out))
		os.Exit(0)
	default:
		t.Fatal("unknown helper mode")
	}
}
func flowRunModel(ctx context.Context, cfg flowLiveConfig, root, label, prompt, sandbox string, resume ...string) (flowProofJob, error) {
	job := flowProofJob{Root: root, Start: time.Now().UnixNano()}
	last := filepath.Join(cfg.Evidence, label+"-last.json")
	args := []string{"exec", "--ignore-user-config", "--dangerously-bypass-hook-trust", "-s", sandbox, "-c", `approval_policy="never"`, "-m", "gpt-6-astra", "-c", `model_reasoning_effort="medium"`, "-c", hookProbeTrustConfig(root), "-C", root, "--json", "-o", last, prompt}
	if len(resume) > 0 {
		args = []string{"exec", "resume", "--ignore-user-config", "--dangerously-bypass-hook-trust", "-c", `sandbox_mode="` + sandbox + `"`, "-c", `approval_policy="never"`, "-m", "gpt-6-astra", "-c", `model_reasoning_effort="medium"`, "-c", hookProbeTrustConfig(root), "--json", "-o", last, resume[0], prompt}
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

type flowProofJob struct {
	Root, Session string
	Start, End    int64
}
