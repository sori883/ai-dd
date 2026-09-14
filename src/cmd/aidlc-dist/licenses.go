package main

import (
	"bytes"
	"fmt"
	"github.com/sori883/ai-dd/src/core"
	"github.com/sori883/ai-dd/src/internal/release"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const productLicenseSHA = "faf14717d84fffb46463c5d2a6a8d380411126d0090009940a7db810f6275f76"

func releaseLicenses(o options) (map[string][]byte, error) {
	if o.LicenseDir == "" {
		return nil, fmt.Errorf("%w: --license-dir required", errInvalidInput)
	}
	files := map[string][]byte{}
	for _, name := range []string{"PRODUCT.txt", "Go-LICENSE.txt", "Go-PATENTS.txt", "GO_VERSION", "yaml-LICENSE.txt", "yaml-NOTICE.txt", "Apache-2.0.txt"} {
		p := filepath.Join(o.LicenseDir, name)
		info, err := os.Lstat(p)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%w: regular license required", errInvalidInput)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			return nil, fmt.Errorf("%w: empty license", errInvalidInput)
		}
		files[name] = raw
	}
	for name, want := range map[string]string{"PRODUCT.txt": productLicenseSHA, "yaml-LICENSE.txt": "d18f6323b71b0b768bb5e9616e36da390fbd39369a81807cca352de4e4e6aa0b", "yaml-NOTICE.txt": "f6c2dd3a67b576eafb89b80200b8b1627230bf3821a0c14cb99a22ac19107d00", "Apache-2.0.txt": "cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30"} {
		if release.Hash(files[name]) != want {
			return nil, fmt.Errorf("%w: wrong %s", errInvalidInput, name)
		}
	}
	if strings.TrimSpace(string(files["GO_VERSION"])) != o.GoVersion || o.GoVersion != runtime.Version() {
		return nil, fmt.Errorf("%w: license/build Go version mismatch", errInvalidInput)
	}
	for _, name := range []string{"LICENSE", "PATENTS"} {
		raw, err := os.ReadFile(filepath.Join(runtime.GOROOT(), name))
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(raw, files["Go-"+name+".txt"]) {
			return nil, fmt.Errorf("%w: Go license mismatch", errInvalidInput)
		}
	}
	return files, nil
}
func licensedEntries(binary string, raw []byte, product string, licenses map[string][]byte) (map[string][]byte, error) {
	entries := map[string][]byte{binary: raw}
	for _, name := range []string{"PRODUCT.txt", "Go-LICENSE.txt", "Go-PATENTS.txt"} {
		entries["LICENSES/"+name] = licenses[name]
	}
	if product != "natural-japanese-go" {
		for _, name := range []string{"yaml-LICENSE.txt", "yaml-NOTICE.txt", "Apache-2.0.txt"} {
			entries["LICENSES/"+name] = licenses[name]
		}
	}
	if product == "natural-japanese-go" {
		raw, err := core.Files.ReadFile("skills/natural-japanese-go/references/cli.md")
		if err != nil {
			return nil, err
		}
		entries["README.md"] = []byte(strings.ReplaceAll(string(raw), "@@NATURAL_BINARY@@", "natural-japanese-go"))
		if err := fs.WalkDir(core.Files, "skills/natural-japanese-go/licenses", func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			raw, e := core.Files.ReadFile(p)
			if e != nil {
				return e
			}
			entries["LICENSES/"+filepath.Base(p)] = raw
			return nil
		}); err != nil {
			return nil, err
		}
	}
	// The embedded distribution is included in runtime/installer/packager binaries.
	if product == "aidlc" || product == "aidlc-install" || product == "aidlc-dist" {
		if err := fs.WalkDir(core.Files, "skills", func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Base(p) != "LICENSE" && !strings.HasSuffix(p, "/references/source.md") {
				return nil
			}
			raw, e := core.Files.ReadFile(p)
			if e != nil {
				return e
			}
			entries["LICENSES/skills/"+strings.TrimPrefix(p, "skills/")] = raw
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return entries, nil
}
