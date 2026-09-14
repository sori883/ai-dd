package cli

import (
	"bytes"
	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"strings"
	"testing"
)

func TestExecutionPlanReviewHelp(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, errout bytes.Buffer
			if code := Run(args, &out, &errout, buildinfo.Info{}, Dependencies{}); code != 0 {
				t.Fatalf("help exit %d", code)
			}
			text := out.String()
			for _, old := range []string{"four-stage", "intent advance ", "--stage <stage>", "Stages: discovery, planning, tdd, integration."} {
				if strings.Contains(text, old) {
					t.Errorf("stale help %q", old)
				}
			}
		})
	}
}
