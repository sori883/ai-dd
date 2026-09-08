package main

import (
	"bytes"
	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
	"strings"
	"testing"
)

func TestFlowCommandPublicCutover(t *testing.T) {
	var out, errout bytes.Buffer
	code := cli.Run([]string{"help"}, &out, &errout, buildinfo.Info{}, cli.Dependencies{})
	if code != 0 {
		t.Fatal(code)
	}
	for _, old := range []string{"aidlc kdr ", "aidlc next ", "aidlc continue ", "aidlc report ", "aidlc intent <target>"} {
		if strings.Contains(out.String(), old) {
			t.Errorf("obsolete entry %s", old)
		}
	}
	for _, want := range []string{"intent configure", "intent review", "intent advance", "unit claim"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %s", want)
		}
	}
	for _, args := range [][]string{{"next"}, {"report"}, {"intent", "legacy"}, {"__codex-user-prompt-submit"}} {
		out.Reset()
		errout.Reset()
		if code := cli.Run(args, &out, &errout, buildinfo.Info{}, cli.Dependencies{}); code != 2 {
			t.Errorf("old route %v exit %d", args, code)
		}
	}
}
