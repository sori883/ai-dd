package buildinfo_test

import (
	"testing"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
)

func TestCurrent_Defaults(t *testing.T) {
	t.Parallel()

	if got, want := buildinfo.Current(), (buildinfo.Info{Version: "dev", Commit: "unknown"}); got != want {
		t.Errorf("Current() = %#v, want %#v", got, want)
	}
}
