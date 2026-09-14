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

func TestECMAScriptDefaultLowerUsesUnicode15FinalSigmaContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "AΣᲉ", want: "aςᲉ"},
		{input: "ᲉΣ", want: "Ᲊσ"},
		{input: "AΣʕ", want: "aσʕ"},
		{input: "ʕΣ", want: "ʕς"},
		{input: "AΣ\u0897B", want: "aς\u0897b"},
		{input: "AΣ\U0001171eB", want: "aσ\U0001171eb"},
		{input: "AΣ\uA7F1B", want: "aς\uA7F1b"},
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
