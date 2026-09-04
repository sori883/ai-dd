package knowledge

import "testing"

func TestJSONStringArraySizeMatchesJSONStringifyByteRules(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int
	}{
		{name: "plain", value: "plain", want: 9},
		{name: "quote and slash", value: `"\\`, want: 10},
		{name: "short controls", value: "\b\f\n\r\t", want: 14},
		{name: "other control", value: "\x00", want: 10},
		{name: "html punctuation", value: "<>&", want: 7},
		{name: "line separators", value: "\u2028\u2029", want: 10},
		{name: "supplementary", value: "𐀀", want: 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jsonStringArraySize([]string{tt.value}); got != tt.want {
				t.Errorf("jsonStringArraySize(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}
