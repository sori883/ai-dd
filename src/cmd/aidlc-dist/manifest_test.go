package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func digest(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }
func TestArchiveReproducibility(t *testing.T) {
	t.Parallel()
	o := archiveFixture(t)
	if err := packageArchives(o); err != nil {
		t.Fatal(err)
	}
	first := o.OutputDir
	for _, target := range fixtureTargets {
		name := "aidlc-" + strings.ReplaceAll(target, "/", "-")
		if strings.HasPrefix(target, "windows/") {
			name += ".exe"
		}
		path := filepath.Join(o.InputDir, name)
		if err := os.Chtimes(path, time.Unix(1234567890, 0), time.Unix(1234567890, 0)); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	slices.Reverse(o.Targets)
	o.OutputDir = filepath.Join(t.TempDir(), "second")
	if err := packageArchives(o); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(first)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 8 {
		t.Errorf("output count %d want 6 archives + manifest + sums", len(files))
	}
	for _, file := range files {
		if !bytes.Equal(mustRead(t, filepath.Join(first, file.Name())), mustRead(t, filepath.Join(o.OutputDir, file.Name()))) {
			t.Errorf("non-reproducible %s", file.Name())
		}
	}
	var manifest struct {
		Schema    int    `json:"schema_version"`
		Version   string `json:"version"`
		Commit    string `json:"source_commit"`
		GoVersion string `json:"go_version"`
		Artifacts []struct {
			Target      string `json:"target"`
			Binary      string `json:"binary"`
			BinarySHA   string `json:"binary_sha256"`
			BinarySize  int64  `json:"binary_size"`
			Archive     string `json:"archive"`
			ArchiveSHA  string `json:"archive_sha256"`
			ArchiveSize int64  `json:"archive_size"`
		} `json:"artifacts"`
	}
	raw := mustRead(t, filepath.Join(first, "manifest.json"))
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Schema != 1 || manifest.Version != o.Version || manifest.Commit != o.Commit || manifest.GoVersion != o.GoVersion || len(manifest.Artifacts) != 6 {
		t.Fatalf("metadata mismatch: %+v", manifest)
	}
	for i, a := range manifest.Artifacts {
		if a.Target != fixtureTargets[i] {
			t.Errorf("target order %q", a.Target)
		}
		binary := []byte("binary for " + a.Target)
		wantName := "aidlc"
		if strings.HasPrefix(a.Target, "windows/") {
			wantName += ".exe"
		}
		if a.Binary != wantName || a.BinarySHA != digest(binary) || a.BinarySize != int64(len(binary)) {
			t.Errorf("binary metadata: %+v", a)
		}
		archive := mustRead(t, filepath.Join(first, a.Archive))
		if a.ArchiveSHA != digest(archive) || a.ArchiveSize != int64(len(archive)) {
			t.Errorf("archive metadata: %+v", a)
		}
	}
	sums := strings.Split(strings.TrimSuffix(string(mustRead(t, filepath.Join(first, "SHA256SUMS"))), "\n"), "\n")
	if len(sums) != 7 {
		t.Fatalf("checksum lines %d", len(sums))
	}
	names := []string{}
	for _, line := range sums {
		hash, name, ok := strings.Cut(line, "  ")
		if !ok {
			t.Fatal(line)
		}
		names = append(names, name)
		if hash != digest(mustRead(t, filepath.Join(first, name))) {
			t.Errorf("checksum mismatch %s", name)
		}
	}
	if !slices.IsSorted(names) || names[len(names)-1] != "manifest.json" {
		t.Fatalf("checksum names/order %v", names)
	}
}
