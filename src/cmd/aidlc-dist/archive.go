package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type options struct {
	LicenseDir                                      string
	Product                                         string
	writeFile                                       func(string, []byte) error
	InputDir, OutputDir, Version, Commit, GoVersion string
	Targets                                         []string
}

var supportedTargets = []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64"}
var errInvalidInput = errors.New("invalid input")

type binaryInput struct {
	target string
	raw    []byte
}

func validateInputs(o options) ([]binaryInput, error) {
	if o.Product == "" {
		o.Product = "aidlc"
	}
	if !slices.Contains([]string{"aidlc", "aidlc-install", "okf", "natural-japanese-go", "aidlc-dist"}, o.Product) {
		return nil, fmt.Errorf("%w: unknown product", errInvalidInput)
	}
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
		name := o.Product + "-" + strings.ReplaceAll(target, "/", "-")
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
