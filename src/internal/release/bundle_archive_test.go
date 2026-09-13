package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// A manifest alone must never authorize an arbitrary executable or incomplete bundle.
func TestBundleArchiveRejectsIncomplete(t *testing.T) {
	entries := map[string][]byte{"aidlc": []byte("binary")}
	m := BundleManifest{2, "v0.1.2", strings.Repeat("a", 40), "go1.26.4", "linux/amd64", []BundleFile{{"aidlc", 6, Hash(entries["aidlc"]), 0755}}}
	entries["manifest.json"], _ = json.Marshal(m)
	raw, err := Archive(entries, "aidlc", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ValidateBundleArchive(raw, m.Version, m.Target, Hash(raw)); err == nil {
		t.Fatal("incomplete bundle accepted")
	}
}
func TestBundleArchiveRejectsUnsafe(t *testing.T) {
	for _, mode := range []string{"hash", "duplicate", "symlink", "parent collision", "bad mode"} {
		t.Run(mode, func(t *testing.T) {
			var b bytes.Buffer
			g := gzip.NewWriter(&b)
			tw := tar.NewWriter(g)
			names := []string{"manifest.json"}
			if mode == "duplicate" {
				names = append(names, "manifest.json")
			}
			if mode == "parent collision" {
				names = []string{"a", "a/b"}
			}
			for _, n := range names {
				h := &tar.Header{Name: n, Mode: 0644, Size: 2, Typeflag: tar.TypeReg}
				if mode == "symlink" {
					h.Typeflag = tar.TypeSymlink
					h.Size = 0
					h.Linkname = "outside"
				}
				if mode == "bad mode" {
					h.Mode = 0777
				}
				if err := tw.WriteHeader(h); err != nil {
					t.Fatal(err)
				}
				if h.Size > 0 {
					tw.Write([]byte("{}"))
				}
			}
			tw.Close()
			g.Close()
			raw := b.Bytes()
			digest := Hash(raw)
			if mode == "hash" {
				digest = strings.Repeat("0", 64)
			}
			if _, _, err := ValidateBundleArchive(raw, "v0.1.2", "linux/amd64", digest); err == nil {
				t.Fatal("unsafe bundle accepted")
			}
		})
	}
}

func TestBundleArchiveComplete(t *testing.T) {
	for _, change := range []string{"valid", "missing", "unknown", "size", "hash", "mode", "source limit", "installer limit", "trailing"} {
		t.Run(change, func(t *testing.T) {
			target := "linux/amd64"
			entries := map[string][]byte{}
			modes := BundlePaths(target)
			for p := range modes {
				entries[p] = []byte("fixture")
			}
			if change == "missing" {
				delete(entries, "okf")
			}
			if change == "unknown" {
				entries["unexpected"] = []byte("x")
				modes["unexpected"] = 0644
			}
			if change == "installer limit" {
				entries["aidlc-install"] = bytes.Repeat([]byte("x"), MaxInstallerBytes+1)
			}
			if change == "source limit" {
				entries["core/adr-template.md"] = bytes.Repeat([]byte("x"), int(MaxSourceBytes)+1)
			}
			names := make([]string, 0, len(entries))
			for p := range entries {
				names = append(names, p)
			}
			slices.Sort(names)
			m := BundleManifest{2, "v0.1.2", strings.Repeat("a", 40), "go1.26.4", target, nil}
			for _, p := range names {
				m.Files = append(m.Files, BundleFile{p, int64(len(entries[p])), Hash(entries[p]), modes[p]})
			}
			switch change {
			case "size":
				m.Files[0].Size++
			case "hash":
				m.Files[0].SHA256 = strings.Repeat("0", 64)
			case "mode":
				modes["aidlc"] = 0644
			}
			entries["manifest.json"], _ = json.Marshal(m)
			names = append(names, "manifest.json")
			modes["manifest.json"] = 0644
			var b bytes.Buffer
			g := gzip.NewWriter(&b)
			tw := tar.NewWriter(g)
			for _, p := range names {
				if err := tw.WriteHeader(&tar.Header{Name: p, Size: int64(len(entries[p])), Mode: int64(modes[p]), Typeflag: tar.TypeReg}); err != nil {
					t.Fatal(err)
				}
				if _, err := tw.Write(entries[p]); err != nil {
					t.Fatal(err)
				}
			}
			tw.Close()
			if change == "trailing" {
				g.Write([]byte("unexpected trailing member"))
			}
			g.Close()
			got, files, err := ValidateBundleArchive(b.Bytes(), m.Version, target, Hash(b.Bytes()))
			if change == "valid" {
				if err != nil || got.Version != m.Version || len(files) != len(entries) {
					t.Fatal(got.Version, len(files), err)
				}
			} else if err == nil {
				t.Fatal("invalid bundle accepted", change)
			}
		})
	}
}
