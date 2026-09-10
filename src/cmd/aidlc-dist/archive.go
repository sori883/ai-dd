package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

type options struct {
	writeFile                                       func(string, []byte) error
	InputDir, OutputDir, Version, Commit, GoVersion string
	Targets                                         []string
}

var supportedTargets = []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64"}
var errInvalidInput = errors.New("invalid input")

func packageArchives(o options) error {
	inputs, err := validateInputs(o)
	if err != nil {
		return err
	}
	if err := os.Mkdir(o.OutputDir, 0755); err != nil {
		return fmt.Errorf("create new output directory: %w", err)
	}
	write := o.writeFile
	if write == nil {
		write = writeNewFile
	}
	m := manifest{SchemaVersion: 1, Version: o.Version, SourceCommit: o.Commit, GoVersion: o.GoVersion}
	for _, input := range inputs {
		windows := strings.HasPrefix(input.target, "windows/")
		archive, err := archiveBytes(input.raw, windows)
		if err != nil {
			return err
		}
		suffix, binary := ".tar.gz", "aidlc"
		if windows {
			suffix, binary = ".zip", "aidlc.exe"
		}
		name := "aidlc_" + o.Version + "_" + strings.ReplaceAll(input.target, "/", "_") + suffix
		if err := write(filepath.Join(o.OutputDir, name), archive); err != nil {
			return fmt.Errorf("write %s (candidate is incomplete): %w", name, err)
		}
		m.Artifacts = append(m.Artifacts, artifact{Target: input.target, Binary: binary, BinarySHA256: checksum(input.raw), BinarySize: int64(len(input.raw)), Archive: name, ArchiveSHA256: checksum(archive), ArchiveSize: int64(len(archive))})
	}
	return writeManifest(o.OutputDir, m, write)
}

type binaryInput struct {
	target string
	raw    []byte
}

func validateInputs(o options) ([]binaryInput, error) {
	if o.InputDir == "" || o.OutputDir == "" || !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`).MatchString(o.Version) || strings.Contains(o.Version, "..") {
		return nil, fmt.Errorf("%w: directories and safe version required", errInvalidInput)
	}
	if !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(o.Commit) {
		return nil, fmt.Errorf("%w: commit must be 40 lowercase hexadecimal digits", errInvalidInput)
	}
	if !regexp.MustCompile(`^go1\.[0-9]+(\.[0-9]+)?((beta|rc)[0-9]+)?$`).MatchString(o.GoVersion) {
		return nil, fmt.Errorf("%w: go1 toolchain version required", errInvalidInput)
	}
	if len(o.Targets) == 0 {
		return nil, fmt.Errorf("%w: nonempty target subset required", errInvalidInput)
	}
	targets := slices.Clone(o.Targets)
	slices.Sort(targets)
	inputs := make([]binaryInput, 0, len(targets))
	for i, target := range targets {
		if !slices.Contains(supportedTargets, target) || i > 0 && targets[i-1] == target {
			return nil, fmt.Errorf("%w: unknown or duplicate target %q", errInvalidInput, target)
		}
		name := "aidlc-" + strings.ReplaceAll(target, "/", "-")
		if strings.HasPrefix(target, "windows/") {
			name += ".exe"
		}
		path := filepath.Join(o.InputDir, name)
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("input %s: %w", name, err)
		}
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return nil, fmt.Errorf("%w: %s must be a nonempty regular file, not a symlink", errInvalidInput, name)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read input %s: %w", name, err)
		}
		if len(raw) == 0 {
			return nil, fmt.Errorf("%w: empty binary %s", errInvalidInput, name)
		}
		inputs = append(inputs, binaryInput{target, raw})
	}
	return inputs, nil
}
func writeNewFile(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, err = file.Write(raw)
	return errors.Join(err, file.Close())
}

func archiveBytes(raw []byte, windows bool) ([]byte, error) {
	var buf bytes.Buffer
	if windows {
		z := zip.NewWriter(&buf)
		h := &zip.FileHeader{Name: "aidlc.exe", Method: zip.Deflate, Modified: time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)}
		h.SetMode(0644)
		w, err := z.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		_, err = w.Write(raw)
		if err = errors.Join(err, z.Close()); err != nil {
			return nil, err
		}
	} else {
		gz := gzip.NewWriter(&buf)
		tr := tar.NewWriter(gz)
		if err := tr.WriteHeader(&tar.Header{Name: "aidlc", Mode: 0755, Size: int64(len(raw)), Typeflag: tar.TypeReg}); err != nil {
			return nil, err
		}
		_, err := tr.Write(raw)
		if err = errors.Join(err, tr.Close(), gz.Close()); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}
