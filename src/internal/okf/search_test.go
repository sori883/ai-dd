package okf

import (
	"reflect"
	"testing"
	"time"
)

func TestSearchFilters(t *testing.T) {
	t.Parallel()
	b := Bundle{Concepts: []Concept{
		{ID: "a", Type: "Guide", Title: "Go Database 日本語", Tags: []string{"One", "Two"}},
		{ID: "b", Type: "Guide", Title: "Gopher", Description: "database", Tags: []string{"One"}},
		{ID: "c", Type: "API", Title: "GO", Tags: []string{"One", "Two"}},
	}}
	for _, tt := range []struct {
		name    string
		options SearchOptions
		ids     []string
	}{
		{name: "tags and", options: SearchOptions{Tags: []string{"One", "Two", "One"}}, ids: []string{"a", "c"}},
		{name: "types or", options: SearchOptions{Types: []string{"Guide", "API"}}, ids: []string{"a", "b", "c"}},
		{name: "case sensitive", options: SearchOptions{Tags: []string{"one"}}, ids: []string{}},
		{name: "mixed and", options: SearchOptions{Types: []string{"Guide"}, Tags: []string{"Two"}}, ids: []string{"a"}},
		{name: "distinct query scoring", options: SearchOptions{Query: "GO go database!", HasQuery: true}, ids: []string{"a", "b", "c"}},
		{name: "not substring", options: SearchOptions{Query: "go", HasQuery: true}, ids: []string{"a", "c"}},
		{name: "japanese", options: SearchOptions{Query: "日本語", HasQuery: true}, ids: []string{"a"}},
		{name: "query and", options: SearchOptions{Query: "go", HasQuery: true, Types: []string{"API"}}, ids: []string{"c"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Search(b, tt.options, time.Time{})
			if err != nil {
				t.Fatal(err)
			}
			ids := []string{}
			for _, r := range got.Results {
				ids = append(ids, r.ConceptID)
			}
			if !reflect.DeepEqual(ids, tt.ids) {
				t.Fatalf("ids %v want %v", ids, tt.ids)
			}
		})
	}
}

func TestSearchLifecycleAndOwnership(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	old := now.Add(-time.Hour)
	b := Bundle{Concepts: []Concept{
		{ID: "draft-stale", Type: "A", Status: "draft", StaleAfter: &now},
		{ID: "draft-fresh", Type: "A", Status: "draft"},
		{ID: "stable-stale", Type: "A", StaleAfter: &now},
		{ID: "old", Type: "A", GeneratedAt: &old},
		{ID: "new", Type: "A", GeneratedAt: &now, Tags: []string{"x"}},
		{ID: "\ue000", Type: "A"}, {ID: "😀", Type: "A"},
		{ID: "deprecated", Type: "A", Status: "deprecated"},
	}, Warnings: []Warning{{Path: "bad", Reason: "bad yaml"}}}
	got, err := Search(b, SearchOptions{Types: []string{"A"}, Limit: 100}, now)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, r := range got.Results {
		ids = append(ids, r.ConceptID)
	}
	want := []string{"new", "old", "😀", "\ue000", "stable-stale", "draft-fresh", "draft-stale"}
	if !reflect.DeepEqual(ids, want) || !got.Results[4].Stale || got.Results[0].Stale {
		t.Fatalf("results %+v", got.Results)
	}
	got.Results[0].Tags[0] = "changed"
	*got.Results[0].GeneratedAt = "changed"
	got.Warnings[0].Reason = "changed"
	if b.Concepts[4].Tags[0] != "x" || !b.Concepts[4].GeneratedAt.Equal(now) || b.Warnings[0].Reason != "bad yaml" {
		t.Fatal("aliased input")
	}
	got, err = Search(b, SearchOptions{Types: []string{"A"}}, now.Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 4 {
		t.Fatalf("default limit %d", len(got.Results))
	}
}

func TestSearchInvalid(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		options SearchOptions
	}{
		{name: "no filter"}, {name: "empty query", options: SearchOptions{HasQuery: true}},
		{name: "punctuation", options: SearchOptions{HasQuery: true, Query: "!?"}},
		{name: "low limit", options: SearchOptions{Types: []string{"A"}, Limit: -1}},
		{name: "high limit", options: SearchOptions{Types: []string{"A"}, Limit: 101}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Search(Bundle{}, tt.options, time.Time{}); err == nil {
				t.Fatal("accepted invalid query")
			}
		})
	}
}
