package okf

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestScanBundleSelection(t *testing.T) {
	t.Parallel()
	f := fstest.MapFS{}
	for _, name := range []string{"z.md", "😀/a.md", "\ue000.md"} {
		f[name] = &fstest.MapFile{Data: []byte("---\ntype: A\ntags: [x]\n---\n[broken](/absent.md)")}
	}
	for _, name := range []string{"index.md", "nested/log.md", "other.MD", "readme.txt"} {
		f[name] = &fstest.MapFile{Data: []byte("not a concept")}
	}
	f["bad.md"] = &fstest.MapFile{Data: []byte("invalid")}
	f["link"] = &fstest.MapFile{Mode: fs.ModeSymlink}
	f["pipe.md"] = &fstest.MapFile{Mode: fs.ModeNamedPipe}
	got, err := ScanBundle(f, "aidlc/spaces/main/knowledge/okf")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Concepts) != 3 || got.Concepts[0].ID != "z" || got.Concepts[1].ID != "😀/a" || got.Concepts[2].ID != "\ue000" {
		t.Fatalf("concepts %+v", got.Concepts)
	}
	if got.Concepts[0].Path != "aidlc/spaces/main/knowledge/okf/z.md" || len(got.Warnings) != 3 {
		t.Fatalf("scan %+v", got)
	}
	for _, w := range got.Warnings {
		if w.Path == "" || w.Reason == "" {
			t.Fatalf("warning %+v", w)
		}
	}
	got.Concepts[0].Tags[0] = "mutated"
	again, err := ScanBundle(f, "aidlc/spaces/main/knowledge/okf")
	if err != nil {
		t.Fatal(err)
	}
	if again.Concepts[0].Tags[0] != "x" {
		t.Fatal("shared tags")
	}
}

func TestScanBundleLimits(t *testing.T) {
	t.Parallel()
	f := fstest.MapFS{}
	for i := range 4096 {
		f[fmt.Sprintf("%04d.md", i)] = &fstest.MapFile{Data: []byte("---\ntype: A\n---\n")}
	}
	got, err := ScanBundle(f, "")
	if err != nil || len(got.Concepts) != 4096 {
		t.Fatalf("4096: %d %v", len(got.Concepts), err)
	}
	f["4096.md"] = &fstest.MapFile{Data: []byte("---\ntype: A\n---\n")}
	got, err = ScanBundle(f, "")
	if err == nil || len(got.Concepts) != 0 {
		t.Fatalf("4097 returned partial success: %d %v", len(got.Concepts), err)
	}
}

func TestScanBundleWarningBudget(t *testing.T) {
	t.Parallel()
	f := fstest.MapFS{}
	for i := range 500 {
		f[fmt.Sprintf("%04d-%s.md", i, strings.Repeat("日\"", 50))] = &fstest.MapFile{Data: []byte("bad")}
	}
	got, err := ScanBundle(f, "bundle")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(got.Warnings)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 6144 || len(got.Warnings) == 0 || len(got.Warnings) >= 500 {
		t.Fatalf("warnings bytes %d count %d", len(data), len(got.Warnings))
	}
	if !strings.Contains(got.Warnings[len(got.Warnings)-1].Reason, "omitted") {
		t.Fatal("missing omission summary")
	}
}

func TestScanBundleRootFailure(t *testing.T) {
	t.Parallel()
	if _, err := ScanBundle(failingFS{}, "bundle"); err == nil {
		t.Fatal("accepted failed root")
	}
	got, err := ScanBundle(fstest.MapFS{}, "bundle")
	if err != nil || got.Concepts == nil || got.Warnings == nil {
		t.Fatalf("empty: %+v %v", got, err)
	}
}

type failingFS struct{}

func (failingFS) Open(string) (fs.File, error) { return nil, fs.ErrPermission }
