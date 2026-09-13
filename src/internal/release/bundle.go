package release

import (
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
)

const MaxManifestBytes = 4 << 20
const MaxSumsBytes = 4 << 10
const MaxInstallerBytes = 64 << 20

type BundleFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}
type BundleManifest struct {
	SchemaVersion int          `json:"schema_version"`
	Version       string       `json:"version"`
	SourceCommit  string       `json:"source_commit"`
	GoVersion     string       `json:"go_version"`
	Target        string       `json:"target"`
	Files         []BundleFile `json:"files"`
}

func BundleName(version, target string) string {
	suffix := ".tar.gz"
	if strings.HasPrefix(target, "windows/") {
		suffix = ".zip"
	}
	return "ai-dd_" + version + "_" + strings.ReplaceAll(target, "/", "_") + suffix
}
func ParseBundleManifest(raw []byte, version, target string) (BundleManifest, error) {
	var m BundleManifest
	if len(raw) > MaxManifestBytes {
		return m, fmt.Errorf("manifest exceeds limit")
	}
	if err := StrictJSON(raw, &m); err != nil {
		return m, err
	}
	if m.SchemaVersion != 2 || !ValidVersion(version) || m.Version != version || m.Target != target || !slices.Contains(Targets, target) || !ValidCommit(m.SourceCommit) || !regexp.MustCompile(`^go1\.[0-9]+\.[0-9]+$`).MatchString(m.GoVersion) {
		return m, fmt.Errorf("invalid bundle manifest identity")
	}
	previous := ""
	if len(m.Files) == 0 {
		return m, fmt.Errorf("empty manifest")
	}
	for _, f := range m.Files {
		if !fs.ValidPath(f.Path) || strings.ContainsAny(f.Path, "\\:\x00") || f.Path == "manifest.json" || f.Path <= previous || f.Size < 0 || f.Size > MaxArchiveBytes || !ValidHash(f.SHA256) || (f.Mode != 0755 && f.Mode != 0644) {
			return m, fmt.Errorf("invalid manifest entry %q", f.Path)
		}
		previous = f.Path
	}
	return m, nil
}
func ParseBundleSums(raw []byte, version string) (map[string]string, error) {
	if !ValidVersion(version) || len(raw) > MaxSumsBytes || !strings.HasSuffix(string(raw), "\n") {
		return nil, fmt.Errorf("invalid bundle sums")
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) != len(Targets) {
		return nil, fmt.Errorf("bundle sums must contain six archives")
	}
	sums := make(map[string]string, len(lines))
	for i, target := range Targets {
		name := BundleName(version, target)
		line := lines[i]
		if len(line) != 66+len(name) || line[64:] != "  "+name || !ValidHash(line[:64]) {
			return nil, fmt.Errorf("invalid bundle sums line %d", i+1)
		}
		sums[name] = line[:64]
	}
	return sums, nil
}
