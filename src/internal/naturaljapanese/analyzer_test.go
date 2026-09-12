package naturaljapanese

import "testing"

func TestAnalyzer(t *testing.T) {
	got, err := Analyze(Prepare("猫が走った。").Sentences)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Tokens) < 3 {
		t.Fatalf("%+v", got)
	}
	cat, verb := false, false
	for _, tok := range got[0].Tokens {
		t.Logf("%+v", tok)
		if tok.Surface == "猫" && tok.Reading == "ネコ" && tok.POS[0] == "名詞" {
			cat = true
		}
		if tok.Surface == "走っ" && tok.Base == "走る" && tok.POS[0] == "動詞" {
			verb = true
		}
	}
	if !cat || !verb {
		t.Fatal(got)
	}
}
func TestAnalyzerEmpty(t *testing.T) {
	got, err := Analyze(nil)
	if err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
}
func TestAnalyzerExamples(t *testing.T) {
	got, err := Analyze(Prepare("これは結果をもたらした。この事実は意味する。そのことは証明した。これは生み出した。ことができる。").Sentences)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range got {
		t.Logf("%s: %+v", s.Text, s.Tokens)
	}
}
