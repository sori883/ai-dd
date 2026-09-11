package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func gitIndependentEnvironment(env []string, path string) []string {
	out := []string{}
	for _, entry := range env {
		if !strings.HasPrefix(entry, "PATH=") {
			out = append(out, entry)
		}
	}
	return append(out, "PATH="+path)
}
func gitIndependentResult(step, stage, sha string, units []string, command, output string) ([]byte, error) {
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(sha) {
		return nil, fmt.Errorf("invalid verification SHA")
	}
	if len(units) == 0 {
		units = []string{""}
	}
	runs := []map[string]any{}
	for _, unit := range units {
		run := map[string]any{"command": command, "exit_code": 0, "output_path": output}
		if unit != "" {
			run["unit_id"] = unit
		}
		runs = append(runs, run)
	}
	return json.Marshal(map[string]any{"step_id": step, "stage": stage, "verification_scope": "intent", "verification_sha256": sha, "runs": runs})
}
