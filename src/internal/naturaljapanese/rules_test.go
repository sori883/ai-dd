package naturaljapanese

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReport(t *testing.T) {
	r, err := Check("# 見出し\n非常に重要。", "a.md", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.SchemaVersion != 1 || r.Engine != "Kagome v2.11.0" || r.Dictionary != "UniDic v1.2.6" || r.File != "a.md" || len(r.Findings) != 1 || r.Findings[0].Line != 2 || r.Stats == nil {
		t.Fatalf("%+v", r)
	}
	b, err := json.Marshal(r)
	if err != nil || !json.Valid(b) {
		t.Fatal(err)
	}
}
func TestGenre(t *testing.T) {
	for _, tt := range []struct {
		name, genre string
		lead        int
	}{{"default", "", 0}, {"essay", "essay", 5}, {"tech", "tech", 0}, {"business", "business", 0}} {
		t.Run(tt.name, func(t *testing.T) {
			r, err := Check(strings.Repeat("猫が走る。", 5), "-", tt.genre)
			if err != nil {
				t.Fatal(err)
			}
			if countCategory(r.Findings, "repeated_sentence_lead") != tt.lead {
				t.Fatal(r)
			}
		})
	}
	if _, err := Check("猫。", "-", "invalid"); err == nil {
		t.Fatal("accepted genre")
	}
}
