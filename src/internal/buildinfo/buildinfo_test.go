package buildinfo_test

import (
	"testing"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
)

func TestCurrent_Defaults(t *testing.T) {
	t.Parallel()

	info := buildinfo.Current()
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "version", got: info.Version, want: "dev"},
		{name: "commit", got: info.Commit, want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}
