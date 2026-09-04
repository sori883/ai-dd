// Package artifact resolves declared stage artifacts to record-relative paths.
package artifact

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/sori883/ai-dd/src/internal/graph"
)

// Input describes one consumed artifact and its resolved record-relative path.
type Input struct {
	Artifact string
	Path     string
	Required bool
}

// Paths contains resolved consumed inputs and produced output paths.
type Paths struct {
	Consumes []Input
	Produces []string
}

// ErrUnsupportedPlacement indicates that an artifact belongs to a placement
// not represented by the ordinary stage-relative path contract.
var ErrUnsupportedPlacement = errors.New("artifact: unsupported placement")

// ResolvePaths resolves declared outputs to record-relative paths.
func ResolvePaths(stage graph.Stage, catalog graph.Snapshot, projectType string) (Paths, error) {
	if err := validateResolvePathsMetadata(stage); err != nil {
		return Paths{}, err
	}
	if err := validateUnsupportedPlacement(stage); err != nil {
		return Paths{}, err
	}

	normalizedProjectType := strings.ToLower(projectType)
	knownProjectType := normalizedProjectType == "brownfield" || normalizedProjectType == "greenfield"
	resolved := Paths{
		Consumes: make([]Input, 0, len(stage.Consumes)),
		Produces: make([]string, 0, len(stage.Produces)+len(stage.OptionalProduces)),
	}
	for _, artifact := range stage.Produces {
		resolved.Produces = append(resolved.Produces, path.Join(stage.Phase, stage.Slug, Filename(artifact)))
	}
	for _, artifact := range stage.OptionalProduces {
		resolved.Produces = append(resolved.Produces, path.Join(stage.Phase, stage.Slug, Filename(artifact)))
	}
	candidates := catalog.Stages()
	for _, consume := range stage.Consumes {
		conditionalProjectType := strings.ToLower(consume.ConditionalOn)
		if conditionalProjectType != "" && knownProjectType && conditionalProjectType != normalizedProjectType {
			continue
		}
		owner := stage
		for _, candidate := range candidates {
			if slices.Contains(candidate.Produces, consume.Artifact) || slices.Contains(candidate.OptionalProduces, consume.Artifact) {
				if err := validateStageIdentity(candidate); err != nil {
					return Paths{}, err
				}
				if err := validateUnsupportedPlacement(candidate); err != nil {
					return Paths{}, err
				}
				owner = candidate
				break
			}
		}
		resolved.Consumes = append(resolved.Consumes, Input{
			Artifact: consume.Artifact,
			Path:     path.Join(owner.Phase, owner.Slug, Filename(consume.Artifact)),
			Required: consume.Required,
		})
	}
	return resolved, nil
}

func validateResolvePathsMetadata(stage graph.Stage) error {
	if err := validateStageIdentity(stage); err != nil {
		return err
	}
	for index, artifact := range stage.Produces {
		if !artifactNamePattern.MatchString(artifact) {
			return fmt.Errorf("resolve paths: invalid produces[%d] %q: %w", index, artifact, ErrInvalidMetadata)
		}
	}
	for index, artifact := range stage.OptionalProduces {
		if !artifactNamePattern.MatchString(artifact) {
			return fmt.Errorf("resolve paths: invalid optional_produces[%d] %q: %w", index, artifact, ErrInvalidMetadata)
		}
	}
	for index, consume := range stage.Consumes {
		if !artifactNamePattern.MatchString(consume.Artifact) {
			return fmt.Errorf("resolve paths: invalid consumes[%d].artifact %q: %w", index, consume.Artifact, ErrInvalidMetadata)
		}
		if consume.ConditionalOn != "" && consume.ConditionalOn != "brownfield" && consume.ConditionalOn != "greenfield" {
			return fmt.Errorf("resolve paths: invalid consumes[%d].conditional_on %q: %w", index, consume.ConditionalOn, ErrInvalidMetadata)
		}
	}
	return nil
}

func validateStageIdentity(stage graph.Stage) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "phase", value: stage.Phase},
		{name: "slug", value: stage.Slug},
	} {
		if !artifactNamePattern.MatchString(field.value) {
			return fmt.Errorf("resolve paths: invalid %s %q: %w", field.name, field.value, ErrInvalidMetadata)
		}
	}
	return nil
}

func validateUnsupportedPlacement(stage graph.Stage) error {
	if stage.ForEach != "" {
		return fmt.Errorf("resolve paths: stage %q uses for_each: %w", stage.Slug, ErrUnsupportedPlacement)
	}
	switch stage.Slug {
	case "nfr-requirements", "nfr-design", "functional-design", "infrastructure-design", "code-generation", "reverse-engineering":
		return fmt.Errorf("resolve paths: stage %q has unsupported placement: %w", stage.Slug, ErrUnsupportedPlacement)
	}
	if stage.ProducesKinds != nil {
		return fmt.Errorf("resolve paths: stage %q has kind-aware outputs: %w", stage.Slug, ErrUnsupportedPlacement)
	}
	return nil
}
