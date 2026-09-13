package codex

import (
	"fmt"
	"github.com/sori883/ai-dd/src/harness"
	"io/fs"
	"path/filepath"
	"strings"
)

type Binaries struct{ AIDLC, OKF, Natural string }

// SiblingBinaries is for local developer fixtures; release installation supplies all paths explicitly.
func SiblingBinaries(binary string) Binaries {
	suffix := ""
	if strings.HasSuffix(binary, ".exe") {
		suffix = ".exe"
	}
	return Binaries{binary, filepath.Join(filepath.Dir(binary), "okf"+suffix), filepath.Join(filepath.Dir(binary), "natural-japanese-go"+suffix)}
}
func (b Binaries) Validate() error {
	seen := map[string]bool{}
	for _, p := range []string{b.AIDLC, b.OKF, b.Natural} {
		if !filepath.IsAbs(p) || strings.ContainsAny(p, "\x00\r\n") || seen[p] {
			return fmt.Errorf("distinct absolute runtime paths required: %w", fs.ErrInvalid)
		}
		seen[p] = true
	}
	return nil
}
func (b Binaries) replace(raw []byte) ([]byte, error) {
	s := strings.NewReplacer("@@BINARY@@", shellQuote(b.AIDLC), "@@AIDLC_BINARY@@", shellQuote(b.AIDLC), "@@OKF_BINARY@@", shellQuote(b.OKF), "@@NATURAL_BINARY@@", shellQuote(b.Natural)).Replace(string(raw))
	if strings.Contains(s, "@@") {
		return nil, fmt.Errorf("unresolved runtime token: %w", fs.ErrInvalid)
	}
	return []byte(s), nil
}
func HookCommand(root string, b Binaries) string {
	return shellQuote(b.AIDLC) + " __hook --project-dir " + shellQuote(root) + " --okf-binary " + shellQuote(b.OKF)
}
func DistributionFrom(root string, b Binaries, common, host fs.FS) ([]harness.Asset, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	if common == nil || host == nil {
		return nil, fmt.Errorf("explicit source files required: %w", fs.ErrInvalid)
	}
	hooks, err := splitHookConfiguration(root, b)
	if err != nil {
		return nil, err
	}
	content, err := splitContentAssets(common, host, b)
	if err != nil {
		return nil, err
	}
	manifest := harness.Manifest{Mappings: []harness.Mapping{{Files: common, Source: "adr-template.md", Destination: "aidlc/templates/adr.md"}, {Files: common, Source: "knowledge", Destination: "aidlc/spaces/default/knowledge", Tree: true}}, Generated: append(content, harness.Asset{Path: ".codex/hooks.json", Data: hooks})}
	assets, err := manifest.Render(b.AIDLC)
	if err != nil {
		return nil, err
	}
	for i := range assets {
		assets[i].Data, err = b.replace(assets[i].Data)
		if err != nil {
			return nil, err
		}
	}
	return assets, nil
}
