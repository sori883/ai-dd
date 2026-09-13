// Package release defines the versioned binary and source asset wire formats.
package release

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

var Products = []string{"aidlc-install", "aidlc", "okf", "natural-japanese-go", "aidlc-dist"}
var Targets = []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64"}

func Hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func ValidVersion(v string) bool {
	return regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`).MatchString(v) && !strings.Contains(v, "..")
}
func ValidCommit(v string) bool { return regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(v) }
func MetadataNames(product string) (string, string) {
	if product == "aidlc" {
		return "manifest.json", "SHA256SUMS"
	}
	return product + "-manifest.json", product + "-SHA256SUMS"
}

type Artifact struct {
	Target        string `json:"target"`
	Binary        string `json:"binary"`
	BinarySHA256  string `json:"binary_sha256"`
	BinarySize    int64  `json:"binary_size"`
	Archive       string `json:"archive"`
	ArchiveSHA256 string `json:"archive_sha256"`
	ArchiveSize   int64  `json:"archive_size"`
}
type Manifest struct {
	SchemaVersion int        `json:"schema_version"`
	Version       string     `json:"version"`
	SourceCommit  string     `json:"source_commit"`
	GoVersion     string     `json:"go_version"`
	Artifacts     []Artifact `json:"artifacts"`
}
type DataFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}
type DataManifest struct {
	SchemaVersion int        `json:"schema_version"`
	Version       string     `json:"version"`
	SourceCommit  string     `json:"source_commit"`
	Archive       string     `json:"archive"`
	ArchiveSHA256 string     `json:"archive_sha256"`
	ArchiveSize   int64      `json:"archive_size"`
	Files         []DataFile `json:"files"`
}
