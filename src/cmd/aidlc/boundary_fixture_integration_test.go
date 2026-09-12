//go:build integration

package main

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/flow"
	"path/filepath"
	"testing"
)

// boundaryFixtureDocument prepares contract documents, never state or review results.
func boundaryFixtureDocument(t *testing.T, root, id, kind string) string {
	t.Helper()
	rel := map[string]string{"Requirements": "design/" + id + "/requirements.md", "ImplementationPlan": "design/" + id + "/implementation-plan.md", "CurrentAnalysis": "codekb/current-analysis.md", "Architecture": "codekb/architecture.md", "Knowledge": "codekb/feature.md"}[kind]
	sections := map[string][]string{"Requirements": {"目的", "範囲", "要件", "受入条件", "未確定事項"}, "ImplementationPlan": {"変更箇所", "実装手順", "検証方法"}, "CurrentAnalysis": {"現状", "構成・動作", "根拠", "未確認事項"}, "Architecture": {"構成図", "構成要素", "データフロー"}, "Knowledge": {"機能", "利用手順", "制約"}}[kind]
	body := "---\ntype: " + kind + "\ntitle: Addition\ndescription: Addition contract\nintent_id: " + id + "\n---\n"
	for _, heading := range sections {
		body += "\n## " + heading + "\nAddition returns the sum of two integers.\n"
		if heading == "構成図" {
			body += "```mermaid\ngraph LR\n Caller-->Add\n```\n"
		}
	}
	name := "aidlc/spaces/default/knowledge/" + rel
	writeAIDLCFixture(t, filepath.Join(root, name), body)
	return name
}
func boundaryFixtureResults(t *testing.T, root, step, stage, head string, commands []string, output []byte) string {
	t.Helper()
	log := "aidlc/evidence/" + stage + ".txt"
	writeAIDLCFixture(t, filepath.Join(root, log), string(output))
	states, err := (flow.Store{Root: root, Space: "default"}).List()
	if err != nil {
		t.Fatal(err)
	}
	var config flow.Config
	for _, st := range states {
		if st.CurrentStepID == step {
			config = st.Config
			break
		}
	}
	digest, err := flow.ComputeVerification(root, config.VerificationPaths)
	if err != nil {
		t.Fatal(err)
	}
	var runs []map[string]any
	for _, command := range commands {
		if len(config.Units) == 0 {
			runs = append(runs, map[string]any{"command": command, "exit_code": 0, "output_path": log})
		} else {
			for _, unit := range config.Units {
				runs = append(runs, map[string]any{"unit_id": unit.ID, "command": command, "exit_code": 0, "output_path": log})
			}
		}
	}
	raw, err := json.Marshal(map[string]any{"step_id": step, "stage": stage, "verification_scope": "intent", "verification_sha256": digest.SHA256, "runs": runs})
	if err != nil {
		t.Fatal(err)
	}
	name := "aidlc/evidence/" + stage + ".json"
	writeAIDLCFixture(t, filepath.Join(root, name), string(raw))
	return name
}
