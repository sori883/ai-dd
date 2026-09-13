package workflow

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestRenderRejectsDuplicateCompletedPaths(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"stages/discovery.md", "stage-graph.json"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			files := fstest.MapFS{
				"workflow/" + name:           {Data: []byte("original")},
				"workflow/" + name + ".tmpl": {Data: []byte("replacement")},
			}
			_, err := Render(files, nil)
			if !errors.Is(err, fs.ErrInvalid) {
				t.Fatalf("duplicate completed path %q: got %v, want fs.ErrInvalid", name, err)
			}
		})
	}
}
