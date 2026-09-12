package naturaljapanese

import (
	"os"
	"testing"
)

func TestFixtures(t *testing.T) {
	for _, name := range []string{"ai-smelly", "natural"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/" + name + ".md")
			if err != nil {
				t.Fatal(err)
			}
			r, err := Check(string(raw), name, "")
			if err != nil {
				t.Fatal(err)
			}
			if name == "ai-smelly" && countCategory(r.Findings, "forbidden_phrase") == 0 {
				t.Fatal("catalog not exercised")
			}
			for _, f := range r.Findings {
				if !member(f.Category, Categories...) || f.Line < 1 {
					t.Fatal(f)
				}
			}
			t.Logf("findings=%d stats=%+v", len(r.Findings), r.Stats)
		})
	}
}
