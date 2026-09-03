package artifact_test

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/artifact"
	"github.com/sori883/ai-dd/src/internal/graph"
)

func TestHasRequiredOutputEmptyProduces(t *testing.T) {
	t.Parallel()

	got, err := artifact.HasRequiredOutput(nil, graph.Stage{})
	if err != nil {
		t.Fatalf("HasRequiredOutput() error = %v, want nil", err)
	}
	if !got {
		t.Error("HasRequiredOutput() = false, want true")
	}
}

func TestHasRequiredOutputCanonicalArtifactPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file *fstest.MapFile
		want bool
	}{
		{
			name: "regular file",
			file: &fstest.MapFile{Data: []byte("content")},
			want: true,
		},
		{
			name: "missing file",
			want: false,
		},
		{
			name: "directory",
			file: &fstest.MapFile{Mode: 0o755 | fs.ModeDir},
			want: false,
		},
		{
			name: "named pipe",
			file: &fstest.MapFile{Mode: fs.ModeNamedPipe},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			files := fstest.MapFS{}
			if tt.file != nil {
				files["ideation/requirements/intent-statement.md"] = tt.file
			}
			got, err := artifact.HasRequiredOutput(files, graph.Stage{
				Phase:    "ideation",
				Slug:     "requirements",
				Produces: []string{"intent-statement"},
			})
			if err != nil {
				t.Fatalf("HasRequiredOutput() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Errorf("HasRequiredOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasRequiredOutputUsesAnyRequiredArtifact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		produces         []string
		optionalProduces []string
		files            fstest.MapFS
		want             bool
	}{
		{
			name:     "second required artifact exists",
			produces: []string{"first-artifact", "second-artifact"},
			files: fstest.MapFS{
				"ideation/requirements/second-artifact.md": {Data: []byte("content")},
			},
			want: true,
		},
		{
			name:             "optional artifact does not satisfy required output",
			produces:         []string{"required-artifact"},
			optionalProduces: []string{"optional-artifact"},
			files: fstest.MapFS{
				"ideation/requirements/optional-artifact.md": {Data: []byte("content")},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := artifact.HasRequiredOutput(tt.files, graph.Stage{
				Phase:            "ideation",
				Slug:             "requirements",
				Produces:         tt.produces,
				OptionalProduces: tt.optionalProduces,
			})
			if err != nil {
				t.Fatalf("HasRequiredOutput() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Errorf("HasRequiredOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasRequiredOutputContinuesAfterStatError(t *testing.T) {
	t.Parallel()

	fsys := &scriptedStatFS{
		files: fstest.MapFS{
			"ideation/requirements/second-artifact.md": {Data: []byte("content")},
		},
		errors: map[string]error{
			"ideation/requirements/first-artifact.md": fs.ErrPermission,
		},
	}

	got, err := artifact.HasRequiredOutput(fsys, graph.Stage{
		Phase:    "ideation",
		Slug:     "requirements",
		Produces: []string{"first-artifact", "second-artifact"},
	})
	if err != nil {
		t.Fatalf("HasRequiredOutput() error = %v, want nil", err)
	}
	if !got {
		t.Error("HasRequiredOutput() = false, want true")
	}
	if !slices.Equal(fsys.stats, []string{
		"ideation/requirements/first-artifact.md",
		"ideation/requirements/second-artifact.md",
	}) {
		t.Errorf("Stat paths = %q, want both required artifacts in order", fsys.stats)
	}
}

func TestHasRequiredOutputArtifactFilenameExceptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		artifact string
		filename string
	}{
		{
			name:     "traceability uses json filename",
			artifact: "traceability",
			filename: "traceability.json",
		},
		{
			name:     "build test results uses shared filename",
			artifact: "build-test-results",
			filename: "test-results.md",
		},
		{
			name:     "load test results uses shared filename",
			artifact: "load-test-results",
			filename: "test-results.md",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			files := fstest.MapFS{
				"construction/test-stage/" + tt.filename: {Data: []byte("content")},
			}
			got, err := artifact.HasRequiredOutput(files, graph.Stage{
				Phase:    "construction",
				Slug:     "test-stage",
				Produces: []string{tt.artifact},
			})
			if err != nil {
				t.Fatalf("HasRequiredOutput() error = %v, want nil", err)
			}
			if !got {
				t.Errorf("HasRequiredOutput() = false, want true for %q", tt.filename)
			}
		})
	}
}

func TestHasRequiredOutputRejectsInvalidMetadataBeforeFilesystem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		stage     graph.Stage
		wantError string
	}{
		{
			name: "invalid phase",
			stage: graph.Stage{
				Phase:    "Ideation",
				Slug:     "requirements",
				Produces: []string{"artifact"},
			},
			wantError: "invalid phase",
		},
		{
			name: "invalid slug",
			stage: graph.Stage{
				Phase:    "ideation",
				Slug:     "requirements_v2",
				Produces: []string{"artifact"},
			},
			wantError: "invalid slug",
		},
		{
			name: "invalid later artifact",
			stage: graph.Stage{
				Phase:    "ideation",
				Slug:     "requirements",
				Produces: []string{"valid-artifact", "Invalid-artifact"},
			},
			wantError: "invalid produces[1]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fsys := &recordingStatFS{}
			got, err := artifact.HasRequiredOutput(fsys, tt.stage)
			if got {
				t.Error("HasRequiredOutput() = true, want false")
			}
			if err == nil {
				t.Fatal("HasRequiredOutput() error = nil, want invalid metadata")
			}
			if !errors.Is(err, artifact.ErrInvalidMetadata) {
				t.Errorf("HasRequiredOutput() error = %v, want ErrInvalidMetadata", err)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("HasRequiredOutput() error = %q, want %q context", err, tt.wantError)
			}
			if fsys.statCalls != 0 || fsys.openCalls != 0 {
				t.Errorf("filesystem calls = (stat %d, open %d), want no I/O", fsys.statCalls, fsys.openCalls)
			}
		})
	}
}

func TestHasRequiredOutputInvalidMetadataTakesPriorityOverNilFilesystem(t *testing.T) {
	t.Parallel()

	got, err := artifact.HasRequiredOutput(nil, graph.Stage{
		Phase:    "Ideation",
		Slug:     "requirements",
		Produces: []string{"artifact"},
	})
	if got {
		t.Error("HasRequiredOutput() = true, want false")
	}
	if !errors.Is(err, artifact.ErrInvalidMetadata) {
		t.Errorf("HasRequiredOutput() error = %v, want ErrInvalidMetadata", err)
	}
	if errors.Is(err, artifact.ErrInvalidFilesystem) {
		t.Errorf("HasRequiredOutput() error = %v, unexpectedly matches ErrInvalidFilesystem", err)
	}
}

func TestHasRequiredOutputRejectsNilFilesystem(t *testing.T) {
	t.Parallel()

	got, err := artifact.HasRequiredOutput(nil, graph.Stage{
		Phase:    "ideation",
		Slug:     "requirements",
		Produces: []string{"artifact"},
	})
	if got {
		t.Error("HasRequiredOutput() = true, want false")
	}
	if !errors.Is(err, artifact.ErrInvalidFilesystem) {
		t.Errorf("HasRequiredOutput() error = %v, want ErrInvalidFilesystem", err)
	}
}

func TestHasRequiredOutputIsReadOnlyAndStatsOnly(t *testing.T) {
	t.Parallel()

	produces := []string{"artifact"}
	stage := graph.Stage{
		Phase:    "ideation",
		Slug:     "requirements",
		Produces: produces,
	}
	fsys := &statOnlyFS{
		files: fstest.MapFS{
			"ideation/requirements/artifact.md": {Data: []byte("content")},
		},
	}

	got, err := artifact.HasRequiredOutput(fsys, stage)
	if err != nil {
		t.Fatalf("HasRequiredOutput() error = %v, want nil", err)
	}
	if !got {
		t.Error("HasRequiredOutput() = false, want true")
	}
	if !slices.Equal(produces, []string{"artifact"}) {
		t.Errorf("Produces mutated to %q", produces)
	}
	if !slices.Equal(fsys.stats, []string{"ideation/requirements/artifact.md"}) {
		t.Errorf("Stat paths = %q, want canonical artifact path", fsys.stats)
	}
	if fsys.opens != 0 {
		t.Errorf("Open calls = %d, want zero", fsys.opens)
	}
}

type recordingStatFS struct {
	statCalls int
	openCalls int
}

type statOnlyFS struct {
	files fstest.MapFS
	stats []string
	opens int
}

type scriptedStatFS struct {
	files  fstest.MapFS
	errors map[string]error
	stats  []string
}

func (f *scriptedStatFS) Open(string) (fs.File, error) {
	return nil, errors.New("unexpected open")
}

func (f *scriptedStatFS) Stat(name string) (fs.FileInfo, error) {
	f.stats = append(f.stats, name)
	if err, ok := f.errors[name]; ok {
		return nil, err
	}
	return fs.Stat(f.files, name)
}

func (f *statOnlyFS) Open(string) (fs.File, error) {
	f.opens++
	return nil, errors.New("content read attempted")
}

func (f *statOnlyFS) Stat(name string) (fs.FileInfo, error) {
	f.stats = append(f.stats, name)
	return fs.Stat(f.files, name)
}

func (f *recordingStatFS) Open(string) (fs.File, error) {
	f.openCalls++
	return nil, fs.ErrNotExist
}

func (f *recordingStatFS) Stat(string) (fs.FileInfo, error) {
	f.statCalls++
	return nil, fs.ErrNotExist
}
