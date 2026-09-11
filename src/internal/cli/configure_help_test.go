package cli

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestConfigureHelp(t *testing.T) {
	text, ok := Help([]string{"intent", "configure", "--help"})
	if !ok {
		t.Fatal("missing help")
	}
	examples := regexp.MustCompile("(?s)```json\\n(.*?)\\n```").FindAllStringSubmatch(text, -1)
	if len(examples) != 2 {
		t.Fatalf("want Unitなし/あり JSON examples, got %d", len(examples))
	}
	for i, example := range examples {
		var c map[string]any
		if err := json.Unmarshal([]byte(example[1]), &c); err != nil {
			t.Fatal(err)
		}
		units, ok := c["units"].([]any)
		if !ok || len(units) != i {
			t.Fatalf("example %d units=%v", i, c["units"])
		}
		if i == 1 {
			u := units[0].(map[string]any)
			for _, key := range []string{"id", "bolt", "depends_on", "scope", "tests", "status", "verification_paths", "result_sha256"} {
				if _, ok := u[key]; !ok {
					t.Errorf("missing %s", key)
				}
			}
			if u["status"] != "pending" || u["result_sha256"] != "" {
				t.Fatal("wrong initial progress")
			}
		}
	}
	for _, want := range []string{"文字列", "真偽値", "文字列配列", "<CURRENT_STEP>", "64桁", "置換", "既存", "進捗", "Unitなし", "Unitあり"} {
		if !strings.Contains(text, want) {
			t.Errorf("help lacks %s", want)
		}
	}
}
