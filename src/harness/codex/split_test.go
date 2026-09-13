package codex

import (
	"github.com/sori883/ai-dd/src/core"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSplitCLIDistribution(t *testing.T) {
	paths := Binaries{AIDLC: "/runtime/aidlc", OKF: "/knowledge/okf", Natural: "/words/natural"}
	assets, err := DistributionFrom("/project", paths, core.Files, Files)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 71 {
		t.Fatalf("assets %d, want 71", len(assets))
	}
	joined := ""
	for _, a := range assets {
		joined += string(a.Data)
		if strings.Contains(string(a.Data), "@@") || strings.Contains(string(a.Data), "aidlc memory") {
			t.Fatalf("unresolved/old command: %s", a.Path)
		}
	}
	for _, p := range []string{paths.AIDLC, paths.OKF, paths.Natural, "--okf-binary"} {
		if !strings.Contains(joined, p) {
			t.Errorf("missing configured path %s", p)
		}
	}
	changed := fstest.MapFS{}
	fs.WalkDir(core.Files, ".", func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			raw, e := fs.ReadFile(core.Files, p)
			if e != nil {
				return e
			}
			changed[p] = &fstest.MapFile{Data: raw}
		}
		return nil
	})
	changed["skills/aidlc/SKILL.md.tmpl"] = &fstest.MapFile{Data: []byte("@@UNKNOWN_BINARY@@")}
	if _, err := DistributionFrom("/project", paths, changed, Files); err == nil {
		t.Fatal("unknown token accepted")
	}
}
