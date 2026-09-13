package main

import (
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/core"
	"github.com/sori883/ai-dd/src/harness/codex"
	"github.com/sori883/ai-dd/src/internal/release"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func packageRelease(o options) error {
	if len(o.Targets) != len(release.Targets) {
		return fmt.Errorf("%w: release requires all six targets", errInvalidInput)
	}
	licenses, err := releaseLicenses(o)
	if err != nil {
		return err
	}
	inputs := map[string][]binaryInput{}
	for _, product := range release.Products {
		single := o
		single.Product = product
		data, err := validateInputs(single)
		if err != nil {
			return err
		}
		inputs[product] = data
	}
	data, dm, err := release.BuildData(o.Version, o.Commit, core.Files, codex.Files, licenses["PRODUCT.txt"])
	if err != nil {
		return err
	}
	if err := os.Mkdir(o.OutputDir, 0755); err != nil {
		return err
	}
	write := o.writeFile
	if write == nil {
		write = writeNewFile
	}
	for _, product := range release.Products {
		m := manifest{SchemaVersion: 1, Version: o.Version, SourceCommit: o.Commit, GoVersion: o.GoVersion}
		for _, input := range inputs[product] {
			windows := strings.HasPrefix(input.target, "windows/")
			binary, suffix := product, ".tar.gz"
			if windows {
				binary += ".exe"
				suffix = ".zip"
			}
			entries, err := licensedEntries(binary, input.raw, product, licenses)
			if err != nil {
				return err
			}
			raw, err := release.Archive(entries, binary, windows)
			if err != nil {
				return err
			}
			name := product + "_" + o.Version + "_" + strings.ReplaceAll(input.target, "/", "_") + suffix
			if err := write(filepath.Join(o.OutputDir, name), raw); err != nil {
				return fmt.Errorf("candidate incomplete: %w", err)
			}
			m.Artifacts = append(m.Artifacts, artifact{input.target, binary, checksum(input.raw), int64(len(input.raw)), name, checksum(raw), int64(len(raw))})
		}
		if err := writeProductManifest(o.OutputDir, product, m, write); err != nil {
			return err
		}
	}
	raw, err := json.MarshalIndent(dm, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	for _, file := range []struct {
		name string
		data []byte
	}{{dm.Archive, data}, {"aidlc-assets-manifest.json", raw}} {
		if err := write(filepath.Join(o.OutputDir, file.name), file.data); err != nil {
			return err
		}
	}
	lines := []string{checksum(raw) + "  aidlc-assets-manifest.json", checksum(data) + "  " + dm.Archive}
	slices.SortFunc(lines, func(a, b string) int {
		return strings.Compare(strings.SplitN(a, "  ", 2)[1], strings.SplitN(b, "  ", 2)[1])
	})
	return write(filepath.Join(o.OutputDir, "aidlc-assets-SHA256SUMS"), []byte(strings.Join(lines, "\n")+"\n"))
}
