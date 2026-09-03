package pathnorm

import "testing"

func TestECMAScriptDefaultLowerFixedWindowsVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "İ", want: "i\u0307"},
		{input: "AİB", want: "ai\u0307b"},
		{input: "Σ", want: "σ"},
		{input: "ΟΣ", want: "ος"},
		{input: "ΟΣΑ", want: "οσα"},
		{input: "AΣ\u0301", want: "aς\u0301"},
		{input: "AΣ\u0301B", want: "aσ\u0301b"},
		{input: "AΣ'B", want: "aσ'b"},
		{input: "AΣ-B", want: "aς-b"},
		{input: "AΣʰ", want: "aςʰ"},
		{input: "AΣⅠ", want: "aσⅰ"},
		{input: "K", want: "k"},
		{input: "AᲉB", want: "aᲉb"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			if got := ECMAScriptDefaultLower(tt.input); got != tt.want {
				t.Errorf("ECMAScriptDefaultLower(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeForPlatformOnlyWindowsFoldsCase(t *testing.T) {
	t.Parallel()

	if got := NormalizeForPlatform("AİB", "windows"); got != "ai\u0307b" {
		t.Errorf("Windows normalization = %q", got)
	}
	if got := NormalizeForPlatform("AİB", "darwin"); got != "AİB" {
		t.Errorf("non-Windows normalization = %q, want unchanged", got)
	}
}
