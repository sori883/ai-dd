package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/minimal"
)

func minimalCommand(request cli.MinimalRequest) ([]byte, error) {
	root := request.ProjectDir
	if root == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		output, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").Output()
		if err != nil {
			return nil, fmt.Errorf("resolve Git worktree root: %w", err)
		}
		root = strings.TrimSpace(string(output))
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	binary, err := os.Executable()
	if err != nil {
		return nil, err
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		return nil, err
	}
	service := minimal.Service{Root: root, Binary: binary}
	if request.Command == "__minimal-hook" {
		raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1024*1024+1))
		if err != nil {
			return nil, err
		}
		if len(raw) > 1024*1024 {
			return nil, fmt.Errorf("hook payload exceeds 1 MiB")
		}
		var input minimal.HookInput
		if err := json.Unmarshal(raw, &input); err != nil {
			return nil, err
		}
		output, err := service.Hook(input)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(output)
		return append(encoded, '\n'), err
	}
	for _, file := range []*string{&request.File, &request.BodyFile} {
		if *file == "" {
			continue
		}
		absolute, err := filepath.Abs(*file)
		if err != nil {
			return nil, err
		}
		// Preserve the final component so rooted reads still reject symlinks.
		parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
		if err != nil {
			return nil, err
		}
		*file = filepath.Join(parent, filepath.Base(absolute))
	}

	return service.Execute(request)
}
