package core

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestContentSharedChangeReachesEveryConsumer(t *testing.T) {
	files := fstest.MapFS{
		"shared.md":        {Data: []byte("共有stateは調整役だけが保存する。")},
		"worker.md.tmpl":   {Data: []byte("実装担当: {{include \"shared.md\"}}\n{{host \"tools\"}}")},
		"reviewer.md.tmpl": {Data: []byte("確認担当: {{include \"shared.md\"}}")},
	}
	for _, body := range []string{"共有stateは調整役だけが保存する。", "変更後の共通契約: 他者の変更を保持する。"} {
		files["shared.md"].Data = []byte(body)
		for _, name := range []string{"worker.md.tmpl", "reviewer.md.tmpl"} {
			got, err := RenderContent(files, name, map[string]string{"tools": "Codex tools"})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(got), body) {
				t.Fatalf("%s did not receive shared content: %q", name, got)
			}
		}
	}
}

func TestContentRejectsInvalidComposition(t *testing.T) {
	for _, tc := range []struct{ name, content string }{
		{"missing", `{{include "absent.md"}}`},
		{"traversal", `{{include "../secret"}}`},
		{"cycle", `{{include "entry.md.tmpl"}}`},
		{"missing host", `{{host "absent"}}`},
		{"malformed", `{{include`},
		{"unexpanded", `{{include "raw.md"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := fstest.MapFS{"entry.md.tmpl": {Data: []byte(tc.content)}, "raw.md": {Data: []byte(`{{unexpanded}}`)}}
			if _, err := RenderContent(files, "entry.md.tmpl", nil); err == nil {
				t.Fatal("invalid composition accepted")
			}
		})
	}
}
