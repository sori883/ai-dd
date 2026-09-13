package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/release"
	"path/filepath"
	"slices"
	"strings"
)

type artifact struct {
	Target        string `json:"target"`
	Binary        string `json:"binary"`
	BinarySHA256  string `json:"binary_sha256"`
	BinarySize    int64  `json:"binary_size"`
	Archive       string `json:"archive"`
	ArchiveSHA256 string `json:"archive_sha256"`
	ArchiveSize   int64  `json:"archive_size"`
}
type manifest struct {
	SchemaVersion int        `json:"schema_version"`
	Version       string     `json:"version"`
	SourceCommit  string     `json:"source_commit"`
	GoVersion     string     `json:"go_version"`
	Artifacts     []artifact `json:"artifacts"`
}

func checksum(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func writeManifest(dir string, m manifest, write func(string, []byte) error) error {
	return writeProductManifest(dir, "aidlc", m, write)
}
func writeProductManifest(dir, product string, m manifest, write func(string, []byte) error) error {
	manifestName, sumsName := release.MetadataNames(product)
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := write(filepath.Join(dir, manifestName), raw); err != nil {
		return err
	}
	lines := make([]string, 0, len(m.Artifacts)+1)
	for _, a := range m.Artifacts {
		lines = append(lines, a.ArchiveSHA256+"  "+a.Archive)
	}
	lines = append(lines, checksum(raw)+"  "+manifestName)
	slices.SortFunc(lines, func(a, b string) int {
		return strings.Compare(strings.SplitN(a, "  ", 2)[1], strings.SplitN(b, "  ", 2)[1])
	})
	return write(filepath.Join(dir, sumsName), []byte(strings.Join(lines, "\n")+"\n"))
}
