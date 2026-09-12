package harness_test

import (
	"errors"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/harness"
)

func TestManifestRender(t *testing.T) {
	t.Parallel()
	source := fstest.MapFS{
		"entry.md":         {Data: []byte("run @@BINARY@@ twice @@BINARY@@")},
		"tree/z.md":        {Data: []byte("日本語")},
		"tree/nested/a.md": {Data: []byte("nested")},
	}
	mappings := []harness.Mapping{
		{Files: source, Source: "tree", Destination: "docs", Tree: true},
		{Files: source, Source: "entry.md", Destination: "entry.md"},
	}
	manifest := harness.Manifest{Mappings: mappings, Generated: []harness.Asset{{Path: "generated", Data: []byte("@@BINARY@@")}}}
	want := []harness.Asset{
		{Path: "docs/nested/a.md", Data: []byte("nested")},
		{Path: "docs/z.md", Data: []byte("日本語")},
		{Path: "entry.md", Data: []byte("run '/opt/it'\"'\"'s binary' twice '/opt/it'\"'\"'s binary'")},
		{Path: "generated", Data: []byte("@@BINARY@@")},
	}
	for i := range 2 {
		got, err := manifest.Render("/opt/it's binary")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("render %d = %+v, want %+v", i, got, want)
		}
		manifest.Mappings[0], manifest.Mappings[1] = manifest.Mappings[1], manifest.Mappings[0]
	}
}

func TestManifestRootTree(t *testing.T) {
	t.Parallel()
	manifest := harness.Manifest{Mappings: []harness.Mapping{{
		Files: fstest.MapFS{"nested/a": {Data: []byte("a")}}, Source: ".", Destination: "", Tree: true,
	}}}
	got, err := manifest.Render("binary")
	want := []harness.Asset{{Path: "nested/a", Data: []byte("a")}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("root projection = %+v, %v", got, err)
	}
}

func TestManifestRejectsInvalid(t *testing.T) {
	t.Parallel()
	source := fstest.MapFS{"file": {Data: []byte("x")}, "dir/file": {Data: []byte("y")}}
	single := harness.Mapping{Files: source, Source: "file", Destination: "valid"}
	tests := []struct {
		name     string
		manifest harness.Manifest
		want     error
	}{
		{name: "missing source", manifest: harness.Manifest{Mappings: []harness.Mapping{{Files: source, Source: "missing", Destination: "valid"}}}, want: fs.ErrNotExist},
		{name: "nil source", manifest: harness.Manifest{Mappings: []harness.Mapping{{Source: "file", Destination: "valid"}}}, want: fs.ErrInvalid},
		{name: "file as tree", manifest: harness.Manifest{Mappings: []harness.Mapping{{Files: source, Source: "file", Destination: "valid", Tree: true}}}, want: fs.ErrInvalid},
		{name: "directory as file", manifest: harness.Manifest{Mappings: []harness.Mapping{{Files: source, Source: "dir", Destination: "valid"}}}, want: fs.ErrInvalid},
		{name: "duplicate mapping", manifest: harness.Manifest{Mappings: []harness.Mapping{single, single}}, want: fs.ErrInvalid},
		{name: "generated collision", manifest: harness.Manifest{Mappings: []harness.Mapping{single}, Generated: []harness.Asset{{Path: "valid"}}}, want: fs.ErrInvalid},
		{name: "duplicate generated", manifest: harness.Manifest{Generated: []harness.Asset{{Path: "a"}, {Path: "a"}}}, want: fs.ErrInvalid},
		{name: "parent file", manifest: harness.Manifest{Generated: []harness.Asset{{Path: "a"}, {Path: "a-b"}, {Path: "a/b"}}}, want: fs.ErrInvalid},
		{name: "child before parent", manifest: harness.Manifest{Generated: []harness.Asset{{Path: "a/b"}, {Path: "a"}}}, want: fs.ErrInvalid},
		{name: "nonregular source", manifest: harness.Manifest{Mappings: []harness.Mapping{{Files: fstest.MapFS{"link": {Mode: fs.ModeSymlink}}, Source: "link", Destination: "valid"}}}, want: fs.ErrInvalid},
	}
	invalid := []string{"", "/absolute", "../escape", "a/../b", "a/./b", "a//b", "a/", ".", "C:/drive", "C:drive", `a\b`, "a\x00b"}
	for _, path := range invalid {
		mapping := single
		mapping.Destination = path
		tests = append(tests, struct {
			name     string
			manifest harness.Manifest
			want     error
		}{
			name: "destination " + path, manifest: harness.Manifest{Mappings: []harness.Mapping{mapping}}, want: fs.ErrInvalid,
		})
		mapping = single
		mapping.Source = path
		tests = append(tests, struct {
			name     string
			manifest harness.Manifest
			want     error
		}{
			name: "source " + path, manifest: harness.Manifest{Mappings: []harness.Mapping{mapping}}, want: fs.ErrInvalid,
		})
		tests = append(tests, struct {
			name     string
			manifest harness.Manifest
			want     error
		}{
			name: "generated " + path, manifest: harness.Manifest{Generated: []harness.Asset{{Path: path}}}, want: fs.ErrInvalid,
		})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.manifest.Render("binary")
			if !errors.Is(err, tt.want) || len(got) != 0 {
				t.Fatalf("invalid manifest returned %+v, %v; want %v and no assets", got, err, tt.want)
			}
		})
	}
}
