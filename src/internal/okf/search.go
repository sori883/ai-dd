package okf

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode"
)

type SearchOptions struct {
	Tags, Types []string
	Query       string
	HasQuery    bool
	Limit       int
}
type Result struct {
	ConceptID   string   `json:"concept_id"`
	Path        string   `json:"path"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Resource    string   `json:"resource"`
	Tags        []string `json:"tags"`
	Status      string   `json:"status"`
	GeneratedAt *string  `json:"generated_at"`
	Stale       bool     `json:"stale"`
	TrustTier   string   `json:"trust_tier"`
}
type SearchResult struct {
	Results  []Result  `json:"results"`
	Warnings []Warning `json:"warnings"`
}

func Search(bundle Bundle, options SearchOptions, now time.Time) (SearchResult, error) {
	if err := ValidateSearch(options); err != nil {
		return SearchResult{}, err
	}
	limit := options.Limit
	if limit == 0 {
		limit = 4
	}
	query := tokens(options.Query)
	type candidate struct {
		concept          Concept
		score, lifecycle int
		isStale          bool
	}
	candidates := []candidate{}
	for _, c := range bundle.Concepts {
		if c.Status == "deprecated" {
			continue
		}
		if len(options.Types) > 0 && !slices.Contains(options.Types, c.Type) {
			continue
		}
		hasTags := true
		for _, tag := range options.Tags {
			if !slices.Contains(c.Tags, tag) {
				hasTags = false
				break
			}
		}
		if !hasTags {
			continue
		}
		metadata := tokens(strings.Join(append([]string{c.Type, c.Title, c.Description}, c.Tags...), " "))
		score := 0
		for token := range query {
			if _, ok := metadata[token]; ok {
				score++
			}
		}
		if options.HasQuery && score == 0 {
			continue
		}
		isStale := c.StaleAfter != nil && !now.Before(*c.StaleAfter)
		lifecycle := 0
		if c.Status == "draft" {
			lifecycle = 2
		}
		if isStale {
			lifecycle++
		}
		candidates = append(candidates, candidate{concept: c, score: score, lifecycle: lifecycle, isStale: isStale})
	}
	slices.SortFunc(candidates, func(a, b candidate) int {
		if a.score != b.score {
			return b.score - a.score
		}
		if a.lifecycle != b.lifecycle {
			return a.lifecycle - b.lifecycle
		}
		x, y := a.concept.GeneratedAt, b.concept.GeneratedAt
		if x == nil && y != nil {
			return 1
		}
		if x != nil && y == nil {
			return -1
		}
		if x != nil && y != nil {
			if order := y.Compare(*x); order != 0 {
				return order
			}
		}
		return compareUTF16(a.concept.ID, b.concept.ID)
	})
	result := SearchResult{Results: []Result{}, Warnings: append([]Warning{}, bundle.Warnings...)}
	for _, candidate := range candidates[:min(limit, len(candidates))] {
		c := candidate.concept
		status := c.Status
		if status == "" {
			status = "stable"
		}
		trust := c.TrustTier
		if trust == "" {
			trust = "unverified"
		}
		var generated *string
		if c.GeneratedAt != nil {
			value := c.GeneratedAt.UTC().Format(time.RFC3339Nano)
			generated = &value
		}
		result.Results = append(result.Results, Result{
			ConceptID: c.ID, Path: c.Path, Type: c.Type, Title: c.Title, Description: c.Description, Resource: c.Resource,
			Tags: append([]string{}, c.Tags...), Status: status, GeneratedAt: generated, Stale: candidate.isStale, TrustTier: trust,
		})
	}
	return result, nil
}

// ValidateSearch distinguishes invalid search syntax before filesystem access.
func ValidateSearch(options SearchOptions) error {
	if len(options.Tags) == 0 && len(options.Types) == 0 && !options.HasQuery {
		return errors.New("at least one tag, type, or query filter is required")
	}
	if options.HasQuery && len(tokens(options.Query)) == 0 {
		return errors.New("query must contain a letter or number")
	}
	if options.Limit < 0 || options.Limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}
	return nil
}

func tokens(value string) map[string]struct{} {
	result := map[string]struct{}{}
	for _, token := range strings.FieldsFunc(value, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }) {
		result[strings.ToLower(token)] = struct{}{}
	}
	return result
}
