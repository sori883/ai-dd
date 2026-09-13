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
	data, _, err := release.BuildData(o.Version, o.Commit, core.Files, codex.Files, licenses["PRODUCT.txt"])
	if err != nil {
		return err
	}
	sources, err := release.Unpack(data, false, release.MaxSourceBytes)
	if err != nil {
		return err
	}
	// Validate every complete archive before creating the candidate directory.
	output := map[string][]byte{}
	var lines []string
	for _, target := range release.Targets {
		entries := map[string][]byte{}
		for p, b := range sources {
			entries[p] = b
		}
		for _, product := range release.Products {
			var body []byte
			for _, in := range inputs[product] {
				if in.target == target {
					body = in.raw
				}
			}
			if len(body) == 0 {
				return fmt.Errorf("%w: missing target %s", errInvalidInput, target)
			}
			binary := product
			if strings.HasPrefix(target, "windows/") {
				binary += ".exe"
			}
			licensed, err := licensedEntries(binary, body, product, licenses)
			if err != nil {
				return err
			}
			for p, b := range licensed {
				if strings.HasPrefix(p, "LICENSES/") {
					p = "LICENSES/" + product + "/" + strings.TrimPrefix(p, "LICENSES/")
				}
				entries[p] = b
			}
		}
		modes := release.BundlePaths(target)
		m := release.BundleManifest{SchemaVersion: 2, Version: o.Version, SourceCommit: o.Commit, GoVersion: o.GoVersion, Target: target}
		names := make([]string, 0, len(entries))
		for p := range entries {
			names = append(names, p)
		}
		slices.Sort(names)
		for _, p := range names {
			m.Files = append(m.Files, release.BundleFile{Path: p, Size: int64(len(entries[p])), SHA256: release.Hash(entries[p]), Mode: modes[p]})
		}
		entries["manifest.json"], err = json.MarshalIndent(m, "", "  ")
		if err != nil {
			return err
		}
		entries["manifest.json"] = append(entries["manifest.json"], '\n')
		raw, err := release.ArchiveModes(entries, modes, strings.HasPrefix(target, "windows/"))
		if err != nil {
			return err
		}
		if _, _, err := release.ValidateBundleArchive(raw, o.Version, target, release.Hash(raw)); err != nil {
			return err
		}
		name := release.BundleName(o.Version, target)
		output[name] = raw
		lines = append(lines, release.Hash(raw)+"  "+name)
	}
	output["SHA256SUMS"] = []byte(strings.Join(lines, "\n") + "\n")
	if err := os.Mkdir(o.OutputDir, 0755); err != nil {
		return err
	}
	write := o.writeFile
	if write == nil {
		write = writeNewFile
	}
	names := make([]string, 0, len(output))
	for p := range output {
		names = append(names, p)
	}
	slices.Sort(names)
	for _, p := range names {
		if err := write(filepath.Join(o.OutputDir, p), output[p]); err != nil {
			return fmt.Errorf("candidate incomplete: %w", err)
		}
	}
	return nil
}
