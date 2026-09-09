package cli

import "testing"

func TestProcedureGrammar(t *testing.T) {
	r, err := ParseMinimal([]string{"intent", "procedure", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--space", "default"})
	if err != nil || r.Action != "procedure" {
		t.Fatalf("procedure rejected: %+v %v", r, err)
	}
	for _, flag := range []string{"--stage", "--file", "--expect"} {
		t.Run(flag, func(t *testing.T) {
			if _, err := ParseMinimal([]string{"intent", "procedure", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--space", "default", flag, "value"}); err == nil {
				t.Fatal("unexpected flag accepted")
			}
		})
	}
}
