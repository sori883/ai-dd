// Package artifact checks whether a stage has produced a required output.
package artifact

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"

	"github.com/sori883/ai-dd/src/internal/graph"
)

var (
	// ErrInvalidMetadata indicates that a stage contains an unsafe artifact
	// path component.
	ErrInvalidMetadata = errors.New("artifact: invalid metadata")
	// ErrInvalidFilesystem indicates that the filesystem input is nil.
	ErrInvalidFilesystem = errors.New("artifact: invalid filesystem")
)

var artifactNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// HasRequiredOutput reports whether at least one required artifact for a
// caller-classified ordinary stage exists as a regular file beneath the
// stage's record directory. The caller must exclude per-unit and CodeKB
// stages; this function does not resolve their special artifact placement.
func HasRequiredOutput(recordFS fs.FS, stage graph.Stage) (bool, error) {
	if len(stage.Produces) == 0 {
		return true, nil
	}

	if err := validateStageMetadata(stage); err != nil {
		return false, err
	}
	if recordFS == nil {
		return false, fmt.Errorf("has required output: filesystem is nil: %w", ErrInvalidFilesystem)
	}

	for _, artifact := range stage.Produces {
		filename := Filename(artifact)
		candidate := path.Join(stage.Phase, stage.Slug, filename)
		info, err := fs.Stat(recordFS, candidate)
		if err != nil {
			continue
		}
		if info != nil && info.Mode().IsRegular() {
			return true, nil
		}
	}
	return false, nil
}

func validateStageMetadata(stage graph.Stage) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "phase", value: stage.Phase},
		{name: "slug", value: stage.Slug},
	} {
		if !artifactNamePattern.MatchString(field.value) {
			return fmt.Errorf("has required output: invalid %s %q: %w", field.name, field.value, ErrInvalidMetadata)
		}
	}
	for index, artifact := range stage.Produces {
		if !artifactNamePattern.MatchString(artifact) {
			return fmt.Errorf("has required output: invalid produces[%d] %q: %w", index, artifact, ErrInvalidMetadata)
		}
	}
	return nil
}

// Filename returns the canonical file name for one declared artifact. The
// mapping is shared by presence checks and audit evidence matching so a
// filename exception cannot drift between the two readers.
func Filename(name string) string {
	switch name {
	case "traceability":
		return "traceability.json"
	case "build-test-results", "load-test-results":
		return "test-results.md"
	default:
		return name + ".md"
	}
}

func artifactFilename(name string) string { return Filename(name) }
