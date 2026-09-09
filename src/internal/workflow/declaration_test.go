package workflow

import (
	coreworkflow "github.com/sori883/ai-dd/src/core/workflow"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentDeclaration(t *testing.T) {
	for _, tc := range []struct {
		name, input, output string
		bad                 bool
	}{
		{"selector", "[{match: {type: Requirements, intent_id: '${intent_id}'}, count: one, version: accepted, accepted_at: discovery}]", "[]", false},
		{"output", "[]", "[{role: plan, path: '${knowledge_root}/design/${intent_id}/plan.md', metadata: {type: ImplementationPlan, intent_id: '${intent_id}'}}]", false},
		{"declared", "[{declared: intent_documents}]", "[{declared: intent_documents}]", false},
		{"old refs", "[{refs: config.adr.refs, version: current}]", "[]", true},
		{"input path", "[{path: '${knowledge_root}/a.md', version: current}]", "[]", true},
		{"output no metadata", "[]", "[{role: plan, path: '${knowledge_root}/a.md'}]", true},
		{"unknown match", "[{match: {type: Note, unknown: true}, count: one, version: current}]", "[]", true},
		{"numeric type", "[{match: {type: 123}, count: one, version: current}]", "[]", true},
		{"numeric tag", "[{match: {type: Note, tags: [123]}, count: one, version: current}]", "[]", true},
		{"count", "[{match: {type: Note}, count: zero, version: current}]", "[]", true},
		{"status", "[{match: {type: Note, status: accepted}, count: one, version: current}]", "[]", true},
		{"declaration mixed", "[{declared: intent_documents, match: {type: Note}}]", "[]", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := definitionFixture(t)
			p := filepath.Join(root, "aidlc/workflow/stages/planning.md")
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			text := strings.Replace(string(raw), "inputs: []", "inputs: "+tc.input, 1)
			text = strings.Replace(text, "outputs: []", "outputs: "+tc.output, 1)
			if err = os.WriteFile(p, []byte(text), 0644); err != nil {
				t.Fatal(err)
			}
			_, err = Load(root)
			if (err != nil) != tc.bad {
				t.Fatalf("declaration error=%v want bad=%v", err, tc.bad)
			}
		})
	}
}

func TestDocumentDeclarationDefault(t *testing.T) {
	root := t.TempDir()
	err := fs.WalkDir(coreworkflow.Files, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		raw, err := coreworkflow.Files.ReadFile(name)
		if err != nil {
			return err
		}
		dest := filepath.Join(root, "aidlc/workflow", name)
		if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		return os.WriteFile(dest, raw, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
	d, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"discovery", "planning", "tdd", "integration"} {
		if len(d.Procedures[stage].Inputs) == 0 || len(d.Procedures[stage].Outputs) == 0 {
			t.Fatal("default declarations absent")
		}
	}
}
