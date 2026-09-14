package workflow

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestRenderRejectsDuplicateCompletedPaths(t *testing.T) {
	t.Parallel()
	files := fstest.MapFS{
		"workflow/stages/discovery.md":      {Data: []byte("original")},
		"workflow/stages/discovery.md.tmpl": {Data: []byte("replacement")},
	}
	if _, err := Render(files, nil); !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("duplicate completed path: %v", err)
	}
}
