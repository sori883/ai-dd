package minimal

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func bodyString(s string) *string { return &s }
func bodyRequest(file string) cli.MinimalRequest {
	return cli.MinimalRequest{Command: "memory", Action: "create", Target: "codekb/note", Space: "default", BodyFile: file, Actor: "process:test", Metadata: okfmemory.MetadataInput{Type: bodyString("Note"), Title: bodyString("Memory"), Description: bodyString("Example"), ExtraJSON: bodyString(`{"custom":"keep"}`)}}
}
func TestMemoryBodyWrite(t *testing.T) {
	s, _ := setup(t)
	draft := filepath.Join(s.Root, "body.md")
	if err := os.WriteFile(draft, []byte("First.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r := bodyRequest(draft)
	start := time.Now().UTC()
	out, err := s.Execute(r)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(s.Root, "aidlc/spaces/default/knowledge")
	doc, err := okfmemory.Read(bundle, r.Target)
	if err != nil {
		t.Fatal(err)
	}
	generated := doc.Metadata["generated"].(map[string]any)
	at, err := time.Parse(time.RFC3339Nano, generated["at"].(string))
	if err != nil || at.Before(start) || at.After(time.Now()) || generated["by"] != r.Actor {
		t.Fatalf("generation %+v %v", generated, err)
	}
	r.Action = "update"
	r.Expect = result["hash"]
	r.Metadata = okfmemory.MetadataInput{Title: bodyString("Renamed")}
	r.Actor = "human:editor"
	out, err = s.Execute(r)
	if err != nil {
		t.Fatal(err)
	}
	doc, err = okfmemory.Read(bundle, r.Target)
	if err != nil {
		t.Fatal(err)
	}
	if doc.String("custom") != "keep" || doc.String("title") != "Renamed" || doc.Body != "First.\n" {
		t.Fatalf("metadata-only update %+v", doc)
	}
	raw, err := os.ReadFile(filepath.Join(bundle, "codekb/note.md"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute(r); err == nil {
		t.Fatal("CAS accepted")
	}
	r.Expect = filestore.Hash(raw)
	r.Metadata = okfmemory.MetadataInput{}
	if _, err := s.Execute(r); err == nil {
		t.Fatal("no-op accepted")
	}
	if err := os.WriteFile(draft, []byte("Second.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute(r); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"codekb/index.md", "log.md"} {
		raw, err := os.ReadFile(filepath.Join(bundle, name))
		if err != nil || !strings.Contains(string(raw), "note") {
			t.Fatalf("bookkeeping %s %s %v", name, raw, err)
		}
	}
}
func TestMemoryBodyWriteRejectsAndPreserves(t *testing.T) {
	for _, mode := range []string{"frontmatter", "symlink body", "outside body", "invalid UTF8", "oversized", "destination directory", "body missing"} {
		t.Run(mode, func(t *testing.T) {
			s, _ := setup(t)
			draft := filepath.Join(s.Root, "body.md")
			if err := os.WriteFile(draft, []byte("Body"), 0600); err != nil {
				t.Fatal(err)
			}
			r := bodyRequest(draft)
			switch mode {
			case "frontmatter":
				os.WriteFile(draft, []byte("---\ntype: Note\n---\nBody"), 0600)
			case "invalid UTF8":
				os.WriteFile(draft, []byte{0xff}, 0600)
			case "oversized":
				os.WriteFile(draft, []byte(strings.Repeat("x", okfmemory.MaxBytes+1)), 0600)
			case "body missing":
				os.Remove(draft)
			case "symlink body", "outside body":
				outside := filepath.Join(t.TempDir(), "body.md")
				os.WriteFile(outside, []byte("Body"), 0600)
				if mode == "outside body" {
					r.BodyFile = outside
				} else {
					os.Remove(draft)
					os.Symlink(outside, draft)
				}
			case "destination directory":
				os.Mkdir(filepath.Join(s.Root, "aidlc/spaces/default/knowledge/codekb/note.md"), 0700)
			}
			if out, err := s.Execute(r); err == nil || len(out) != 0 {
				t.Fatalf("invalid write result %s %v", out, err)
			}
			if _, err := os.Stat(filepath.Join(s.Root, "aidlc/spaces/default/knowledge/log.md")); !os.IsNotExist(err) {
				t.Fatal("failure wrote bookkeeping")
			}
		})
	}
}
