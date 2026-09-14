package okfmemory

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func metadataString(s string) *string { return &s }
func metadataInput() MetadataInput {
	return MetadataInput{Type: metadataString("Design"), Title: metadataString("Title: [quoted]\nsecond"), Description: metadataString("How # documented"), Actor: "process:codex"}
}
func TestMetadataInputBuildAndPreserve(t *testing.T) {
	now := time.Date(2026, 9, 8, 10, 11, 12, 0, time.FixedZone("JST", 9*3600))
	input := metadataInput()
	input.Tags = []string{"one", "two"}
	input.ExtraJSON = metadataString(`{"custom":{"n":9007199254740993,"enabled":true},"keep":"yes"}`)
	input.SourcesJSON = metadataString(`[{"resource":"src.go","usage_count":2,"last_modified":"2025-01-01T00:00:00Z","custom":"source"}]`)
	input.VerifiedJSON = metadataString(`{"by":"human:alice","at":"2026-01-01T00:00:00Z","note":"review"}`)
	input.IntentID = metadataString(strings.Repeat("a", 32))
	doc, err := BuildMetadata(nil, []byte("# Body\n\n---\nHorizontal line\n"), input, now)
	if err != nil {
		t.Fatal(err)
	}
	if doc.String("type") != "Design" || doc.String("title") != *input.Title || doc.String("status") != "stable" {
		t.Fatalf("metadata %+v", doc.Metadata)
	}
	raw, err := doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "9007199254740993") {
		t.Fatalf("number changed: %s", raw)
	}
	doc, err = Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	generated, ok := doc.Metadata["generated"].(map[string]any)
	if !ok || generated["by"] != "process:codex" || generated["at"] != now.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("generated %+v", doc.Metadata["generated"])
	}
	before, _ := doc.Bytes()
	patch := MetadataInput{Actor: "human:bob", Title: metadataString("Revised"), ExtraJSON: metadataString(`{"new":true}`)}
	updated, err := BuildMetadata(&doc, []byte(doc.Body), patch, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"custom", "keep", "sources", "verified", "intent_id", "tags", "description"} {
		if !reflect.DeepEqual(updated.Metadata[key], doc.Metadata[key]) {
			t.Errorf("lost %s", key)
		}
	}
	after, _ := doc.Bytes()
	if string(before) != string(after) {
		t.Fatal("mutated old document")
	}
	if updated.String("title") != "Revised" {
		t.Fatal("metadata-only change lost")
	}
	if _, err := BuildMetadata(&updated, []byte(updated.Body), MetadataInput{Actor: "process:different"}, now.Add(2*time.Hour)); err == nil {
		t.Fatal("generated-only no-op accepted")
	}
	cleared, err := BuildMetadata(&updated, []byte(updated.Body), MetadataInput{Actor: "process:a", ClearTags: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if tags, ok := cleared.Metadata["tags"].([]any); !ok || len(tags) != 0 {
		t.Fatalf("clear tags %+v", cleared.Metadata["tags"])
	}
}
func TestMetadataInputRejectsInvalid(t *testing.T) {
	for _, edit := range []func(*MetadataInput){
		func(i *MetadataInput) { i.Type = nil }, func(i *MetadataInput) { i.Title = metadataString(" ") }, func(i *MetadataInput) { i.Actor = "" },
		func(i *MetadataInput) { i.Status = metadataString("accepted") }, func(i *MetadataInput) { i.IntentID = metadataString("bad") },
		func(i *MetadataInput) { i.StaleAfter = metadataString("2026-01-01") },
		func(i *MetadataInput) { i.SourcesJSON = metadataString(`{}`) }, func(i *MetadataInput) { i.SourcesJSON = metadataString(`[{}]`) },
		func(i *MetadataInput) { i.VerifiedJSON = metadataString(`null`) }, func(i *MetadataInput) { i.VerifiedJSON = metadataString(`{"by":"human:a"}`) },
		func(i *MetadataInput) { i.ExtraJSON = metadataString(`[]`) },
		func(i *MetadataInput) { i.ExtraJSON = metadataString("{\"x\":\"" + string([]byte{0xff}) + "\"}") }, func(i *MetadataInput) { i.ExtraJSON = metadataString(`{"a":1,"a":2}`) },
		func(i *MetadataInput) { i.ExtraJSON = metadataString(`{"nested":{"a":1,"a":2}}`) }, func(i *MetadataInput) { i.ExtraJSON = metadataString(`{} {}`) },
		func(i *MetadataInput) { i.ExtraJSON = metadataString(`{"generated":{}}`) }, func(i *MetadataInput) { i.ExtraJSON = metadataString(`{"title":"x"}`) },
		func(i *MetadataInput) { i.ExtraJSON = metadataString(`{"intent_id":"x"}`) }, func(i *MetadataInput) { i.Tags = []string{"x"}; i.ClearTags = true },
	} {
		i := metadataInput()
		edit(&i)
		if _, err := BuildMetadata(nil, []byte("body"), i, time.Now()); err == nil {
			t.Errorf("invalid input accepted %+v", i)
		}
	}
	for _, body := range [][]byte{[]byte("---\ntype: Design\n---\nbody"), []byte("---\r\ntype: Design\r\n---\r\nbody"), {0xff}, []byte(strings.Repeat("x", MaxBytes+1))} {
		if _, err := BuildMetadata(nil, body, metadataInput(), time.Now()); err == nil {
			t.Error("invalid body accepted")
		}
	}
}

func TestMetadataInputExistingOptionalFields(t *testing.T) {
	old := Document{Metadata: map[string]any{"type": "Note", "custom": "keep"}, Body: "Old"}
	doc, err := BuildMetadata(&old, []byte("New"), MetadataInput{Actor: "process:test"}, time.Now())
	if err != nil {
		t.Fatal("update required absent optional legacy metadata", err)
	}
	if _, ok := doc.Metadata["title"]; ok {
		t.Fatal("invented legacy title")
	}
	if _, ok := doc.Metadata["description"]; ok {
		t.Fatal("invented legacy description")
	}
	if doc.String("custom") != "keep" {
		t.Fatal("lost extension")
	}
}
