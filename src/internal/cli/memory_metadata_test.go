package cli

import (
	"reflect"
	"strings"
	"testing"
)

func TestMemoryMetadataCLI(t *testing.T) {
	base := []string{"memory", "create", "knowledge/auth", "--space", "default", "--body-file", "body.md", "--actor", "process:codex", "--type", "Design", "--title", "Auth", "--description", "How"}
	r, err := ParseMinimal(append(append([]string{}, base...), "--tag", "one", "--tag", "two", "--status", "draft", "--intent-id", strings.Repeat("a", 32), "--resource", "source", "--stale-after", "2026-09-08T00:00:00Z", "--sources-json", "[]", "--verified-json", "[]", "--metadata-json", `{"custom":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.BodyFile != "body.md" || r.File != "" || r.Metadata.Type == nil || *r.Metadata.Type != "Design" || !reflect.DeepEqual(r.Metadata.Tags, []string{"one", "two"}) || r.Metadata.IntentID == nil || r.Metadata.ExtraJSON == nil {
		t.Fatalf("request %+v", r)
	}
	update := []string{"memory", "update", "knowledge/auth", "--space", "default", "--body-file", "body.md", "--actor", "human:me", "--expect", "hash"}
	r, err = ParseMinimal(update)
	if err != nil || r.Metadata.Type != nil || r.Metadata.Tags != nil || r.Metadata.Status != nil {
		t.Fatalf("omitted metadata lost: %+v %v", r, err)
	}
	r, err = ParseMinimal(append(update, "--clear-tags"))
	if err != nil || !r.Metadata.ClearTags {
		t.Fatalf("clear tags %+v %v", r, err)
	}
	for _, extra := range [][]string{{"--type", "Rule"}, {"--tag", "one", "--clear-tags"}, {"--status", "accepted"}, {"--intent-id", "bad"}, {"--file", "old.md"}, {"--title", ""}, {"--unknown", "x"}, {"--tag"}} {
		args := append(append([]string{}, base...), extra...)
		if _, err := ParseMinimal(args); err == nil {
			t.Errorf("accepted invalid %v", extra)
		}
	}
	for _, flag := range []string{"--body-file", "--actor", "--type", "--title", "--description"} {
		args := append([]string{}, base...)
		for i, a := range args {
			if a == flag {
				args = append(args[:i], args[i+2:]...)
				break
			}
		}
		if _, err := ParseMinimal(args); err == nil {
			t.Errorf("missing %s accepted", flag)
		}
	}
	if _, err := ParseMinimal([]string{"intent", "configure", "id", "--space", "default", "--expect", "1", "--file", "config.json"}); err != nil {
		t.Fatalf("Intent JSON flag changed: %v", err)
	}
	_, err = ParseMinimal([]string{"memory", "create", "x", "--space", "default", "--file", "old.md", "--actor", "process:a"})
	if err == nil || !strings.Contains(err.Error(), "--body-file") || !strings.Contains(err.Error(), "memory create --help") {
		t.Fatalf("legacy guidance %v", err)
	}
}
