package naturaljapanese

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBaseline(t *testing.T) {
	old, err := Check("非常に重要。\n非常に重要。\n根本的な問題。", "old", "")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	current, err := Check("\n非常に重要。\n包括的な問題。", "new", "")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Compare(current, raw)
	if err != nil {
		t.Fatal(err)
	}
	if r.Comparison["persisting"] != 1 || r.Comparison["resolved"] != 2 || r.Comparison["new"] != 1 {
		t.Fatalf("%+v", r)
	}
}
func TestBaselineInvalid(t *testing.T) {
	r, err := Check("猫。", "-", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"{}", "null", `{"findings":[]}`, `{"schema_version":1,"engine":"Sudachi","dictionary":"UniDic v1.2.6","findings":[]}`} {
		t.Run(raw, func(t *testing.T) {
			if _, err := Compare(r, []byte(raw)); err == nil {
				t.Fatal("accepted malformed or incompatible baseline")
			}
		})
	}
}

func TestBaselineExcerptRequired(t *testing.T) {
	r, err := Check("非常に重要。", "-", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, missing := range []bool{true, false} {
		raw, _ := json.Marshal(r)
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		f := doc["findings"].([]any)[0].(map[string]any)
		if missing {
			delete(f, "excerpt")
		} else {
			f["excerpt"] = ""
		}
		raw, _ = json.Marshal(doc)
		if _, err := Compare(r, raw); err == nil || !strings.Contains(err.Error(), "excerpt") {
			t.Fatalf("missing=%v error=%v", missing, err)
		}
	}
}
