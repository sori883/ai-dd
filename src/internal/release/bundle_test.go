package release

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBundleManifest(t *testing.T) {
	valid := BundleManifest{2, "v0.1.2", strings.Repeat("a", 40), "go1.26.4", "linux/amd64", []BundleFile{{"aidlc", 1, Hash([]byte("x")), 0755}}}
	for _, mode := range []string{"valid", "old", "version", "target", "commit", "hash", "mode", "duplicate", "unknown", "trailing", "oversized", "order"} {
		t.Run(mode, func(t *testing.T) {
			m := valid
			m.Files = append([]BundleFile{}, valid.Files...)
			switch mode {
			case "old":
				m.SchemaVersion = 1
			case "version":
				m.Version = "v1"
			case "target":
				m.Target = "plan9/amd64"
			case "commit":
				m.SourceCommit = "bad"
			case "hash":
				m.Files[0].SHA256 = "bad"
			case "mode":
				m.Files[0].Mode = 0777
			case "order":
				m.Files = append(m.Files, BundleFile{"aaa", 1, Hash([]byte("x")), 0644})
			}
			raw, _ := json.Marshal(m)
			switch mode {
			case "duplicate":
				raw = []byte(strings.Replace(string(raw), `"schema_version":2`, `"schema_version":2,"schema_version":2`, 1))
			case "unknown":
				raw = append([]byte(`{"future":true,`), raw[1:]...)
			case "trailing":
				raw = append(raw, []byte(` {}`)...)
			case "oversized":
				raw = append(raw, []byte(strings.Repeat(" ", MaxManifestBytes))...)
			}
			got, err := ParseBundleManifest(raw, "v0.1.2", "linux/amd64")
			if mode == "valid" {
				if err != nil || got.Version != m.Version {
					t.Fatal(got, err)
				}
			} else if err == nil {
				t.Fatal("invalid manifest accepted", mode)
			}
		})
	}
}
func TestBundleSums(t *testing.T) {
	var lines []string
	for _, target := range Targets {
		lines = append(lines, Hash([]byte(target))+"  ai-dd_v0.1.2_"+strings.ReplaceAll(target, "/", "_")+map[bool]string{true: ".zip", false: ".tar.gz"}[strings.HasPrefix(target, "windows/")])
	}
	raw := strings.Join(lines, "\n") + "\n"
	for _, mode := range []string{"valid", "missing", "extra", "duplicate", "bad hash", "no newline", "old name", "oversized"} {
		t.Run(mode, func(t *testing.T) {
			s := raw
			switch mode {
			case "missing":
				s = strings.Join(lines[:5], "\n") + "\n"
			case "extra":
				s += lines[0] + "\n"
			case "duplicate":
				s = strings.Replace(s, lines[1], lines[0], 1)
			case "bad hash":
				s = "z" + s[1:]
			case "no newline":
				s = strings.TrimSuffix(s, "\n")
			case "old name":
				s = strings.ReplaceAll(s, "ai-dd_", "aidlc_")
			case "oversized":
				s += strings.Repeat(" ", MaxSumsBytes)
			}
			got, err := ParseBundleSums([]byte(s), "v0.1.2")
			if mode == "valid" {
				if err != nil || len(got) != 6 {
					t.Fatal(got, err)
				}
			} else if err == nil {
				t.Fatal("invalid sums accepted", mode)
			}
		})
	}
}
