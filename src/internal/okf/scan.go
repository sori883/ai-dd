package okf

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type Warning struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}
type Bundle struct {
	Concepts []Concept
	Warnings []Warning
}

// ScanBundle scans a caller-rooted filesystem without following symlinks or links.
func ScanBundle(fsys fs.FS, prefix string) (Bundle, error) {
	result := Bundle{Concepts: []Concept{}, Warnings: []Warning{}}
	if fsys == nil {
		return Bundle{}, errors.New("nil bundle filesystem")
	}
	if prefix != "" && (!fs.ValidPath(prefix) || strings.Contains(prefix, "\\")) {
		return Bundle{}, errors.New("invalid display prefix")
	}
	count := 0
	var walk func(string) error
	walk = func(dir string) error {
		entries, err := fs.ReadDir(fsys, dir)
		if err != nil {
			return fmt.Errorf("enumerate bundle %q: %w", dir, err)
		}
		slices.SortFunc(entries, func(a, b fs.DirEntry) int { return compareUTF16(a.Name(), b.Name()) })
		for _, entry := range entries {
			name := entry.Name()
			relative := path.Join(dir, name)
			display := path.Join(prefix, relative)
			if !fs.ValidPath(name) || strings.ContainsAny(name, "/\\") || !utf8.ValidString(name) {
				result.Warnings = append(result.Warnings, Warning{Path: display, Reason: "invalid entry name"})
				continue
			}
			if name == "index.md" || name == "log.md" {
				continue
			}
			if entry.Type()&fs.ModeSymlink != 0 {
				result.Warnings = append(result.Warnings, Warning{Path: display, Reason: "symlink excluded"})
				continue
			}
			if entry.IsDir() {
				if err := walk(relative); err != nil {
					return err
				}
				continue
			}
			if !entry.Type().IsRegular() {
				result.Warnings = append(result.Warnings, Warning{Path: display, Reason: "special file excluded"})
				continue
			}
			if !strings.HasSuffix(name, ".md") {
				continue
			}
			count++
			if count > 4096 {
				return errors.New("bundle exceeds 4096 concept candidates")
			}
			concept, err := readConcept(fsys, relative)
			if err != nil {
				result.Warnings = append(result.Warnings, Warning{Path: display, Reason: err.Error()})
				continue
			}
			concept.ID = strings.TrimSuffix(relative, ".md")
			concept.Path = display
			result.Concepts = append(result.Concepts, concept)
		}
		return nil
	}
	if err := walk("."); err != nil {
		return Bundle{}, err
	}
	result.Warnings = boundWarnings(result.Warnings)
	return result, nil
}

func readConcept(fsys fs.FS, name string) (Concept, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return Concept{}, fmt.Errorf("open concept: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return Concept{}, fmt.Errorf("stat concept: %w", err)
	}
	if !info.Mode().IsRegular() {
		return Concept{}, errors.New("not a regular file")
	}
	return ParseConcept(f)
}

func compareUTF16(a, b string) int {
	return slices.Compare(utf16.Encode([]rune(a)), utf16.Encode([]rune(b)))
}
