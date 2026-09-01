package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestDecodeIntentRegistryForMutationPreservesValidRawRows(t *testing.T) {
	t.Parallel()

	data := []byte(`[
        {"uuid":"existing","slug":"keep","status":"planning","future":{"nested":true}}
    ]`)
	rows, err := decodeIntentRegistryForMutation(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want one existing row", len(rows))
	}
	want := `{"uuid":"existing","slug":"keep","status":"planning","future":{"nested":true}}`
	if string(rows[0]) != want {
		t.Errorf("raw row = %s, want unknown fields preserved as %s", rows[0], want)
	}
}

func TestDecodeIntentRegistryForMutationRejectsInvalidExistingData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
	}{
		{name: "blank", data: " \n\t"},
		{name: "malformed", data: "["},
		{name: "object", data: `{}`},
		{name: "null", data: `null`},
		{name: "invalid row", data: `[{"uuid":"one","slug":"one"}]`},
		{name: "repos null member", data: `[{"uuid":"one","slug":"one","status":"planning","repos":[null]}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rows, err := decodeIntentRegistryForMutation([]byte(tt.data))
			if rows != nil || !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("decodeIntentRegistryForMutation() = (%v, %v), want nil and fs.ErrInvalid", rows, err)
			}
		})
	}
}

func TestWriteIntentRegistryAppendsFormattedRowAndPreservesUnknownFields(t *testing.T) {
	t.Parallel()

	scope := "repository"
	rows := []json.RawMessage{
		json.RawMessage(`{"uuid":"existing","slug":"keep","status":"planning","future":true}`),
	}
	entry := intentRegistryEntry{
		UUID:    "0199-aaaa",
		Slug:    "build-auth",
		DirName: "260901-build-auth",
		Scope:   &scope,
		Repos:   []string{"api", "web"},
		Status:  "in-flight",
	}
	steps := []string{}
	var content []byte
	ops := registryWriteOps{
		tempName: func() string { return ".intents-test.tmp" },
		openFile: func(name string, flags int, mode fs.FileMode) (*os.File, error) {
			steps = append(steps, fmt.Sprintf("open %s %d %04o", name, flags, mode))
			return nil, nil
		},
		write: func(_ *os.File, data []byte) (int, error) {
			steps = append(steps, "write")
			content = append([]byte(nil), data...)
			return len(data), nil
		},
		close: func(*os.File) error {
			steps = append(steps, "close")
			return nil
		},
		rename: func(from, to string) error {
			steps = append(steps, "rename "+from+" "+to)
			return nil
		},
		remove: func(name string) error {
			steps = append(steps, "remove "+name)
			return nil
		},
	}
	if err := writeIntentRegistry(rows, entry, ops); err != nil {
		t.Fatal(err)
	}
	wantSteps := []string{
		fmt.Sprintf("open .intents-test.tmp %d 0666", os.O_WRONLY|os.O_CREATE|os.O_EXCL),
		"write",
		"close",
		"rename .intents-test.tmp intents.json",
	}
	if !slices.Equal(steps, wantSteps) {
		t.Errorf("steps = %q, want exclusive atomic replacement %q", steps, wantSteps)
	}
	want := `[
  {
    "uuid": "existing",
    "slug": "keep",
    "status": "planning",
    "future": true
  },
  {
    "uuid": "0199-aaaa",
    "slug": "build-auth",
    "dirName": "260901-build-auth",
    "scope": "repository",
    "repos": [
      "api",
      "web"
    ],
    "status": "in-flight"
  }
]
`
	if string(content) != want {
		t.Errorf("registry bytes =\n%s\nwant:\n%s", content, want)
	}
}

func TestReadIntentRegistryForMutationIsStrictAndRejectsNonRegularFiles(t *testing.T) {
	t.Parallel()

	valid := []byte(`[{"uuid":"one","slug":"one","status":"planning","future":true}]`)
	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		rows, err := readIntentRegistryForMutation(registryReadOps{
			lstat:    func(string) (fs.FileInfo, error) { return testFileInfo{mode: 0o600}, nil },
			readFile: func(string) ([]byte, error) { return valid, nil },
		})
		if err != nil || len(rows) != 1 {
			t.Errorf("readIntentRegistryForMutation() = (%v, %v), want one valid raw row", rows, err)
		}
	})

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		rows, err := readIntentRegistryForMutation(registryReadOps{
			lstat: func(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist },
			readFile: func(string) ([]byte, error) {
				t.Error("missing registry was read")
				return nil, nil
			},
		})
		if err != nil || rows == nil || len(rows) != 0 {
			t.Errorf("readIntentRegistryForMutation() = (%v, %v), want non-nil empty rows", rows, err)
		}
	})

	for _, mode := range []fs.FileMode{fs.ModeSymlink, fs.ModeDir, fs.ModeNamedPipe} {
		t.Run(mode.String(), func(t *testing.T) {
			t.Parallel()

			rows, err := readIntentRegistryForMutation(registryReadOps{
				lstat: func(string) (fs.FileInfo, error) { return testFileInfo{mode: mode}, nil },
				readFile: func(string) ([]byte, error) {
					t.Error("nonregular registry was read")
					return nil, nil
				},
			})
			if rows != nil || !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("readIntentRegistryForMutation() = (%v, %v), want fs.ErrInvalid", rows, err)
			}
		})
	}

	t.Run("partial read error", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("injected read failure")
		rows, err := readIntentRegistryForMutation(registryReadOps{
			lstat:    func(string) (fs.FileInfo, error) { return testFileInfo{mode: 0o600}, nil },
			readFile: func(string) ([]byte, error) { return valid, cause },
		})
		if rows != nil || !errors.Is(err, cause) {
			t.Errorf("readIntentRegistryForMutation() = (%v, %v), want discarded data and read cause", rows, err)
		}
	})
}

func TestWriteIntentRegistryOmitsEmptyOptionalFields(t *testing.T) {
	t.Parallel()

	var content []byte
	ops := successfulRegistryWriteOps()
	ops.write = func(_ *os.File, data []byte) (int, error) {
		content = append([]byte(nil), data...)
		return len(data), nil
	}
	entry := intentRegistryEntry{
		UUID: "0199-aaaa", Slug: "build-auth", DirName: "260901-build-auth", Status: "in-flight",
	}
	if err := writeIntentRegistry([]json.RawMessage{}, entry, ops); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(content, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want one", len(rows))
	}
	for _, field := range []string{"scope", "repos"} {
		if _, exists := rows[0][field]; exists {
			t.Errorf("empty optional field %q was persisted", field)
		}
	}
}

func TestWriteIntentRegistryMatchesJSONStringEscaping(t *testing.T) {
	t.Parallel()

	scope := "<shared & explicit>"
	var content []byte
	ops := successfulRegistryWriteOps()
	ops.write = func(_ *os.File, data []byte) (int, error) {
		content = append([]byte(nil), data...)
		return len(data), nil
	}
	entry := intentRegistryEntry{
		UUID: "uuid", Slug: "slug", DirName: "dir", Scope: &scope,
		Repos: []string{"api&web"}, Status: "in-flight",
	}
	if err := writeIntentRegistry([]json.RawMessage{}, entry, ops); err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, `"scope": "<shared & explicit>"`) ||
		!strings.Contains(text, `"api&web"`) {
		t.Errorf("registry escaped JSON.stringify-compatible values: %s", text)
	}
	if strings.Contains(text, `\u003c`) || strings.Contains(text, `\u003e`) || strings.Contains(text, `\u0026`) {
		t.Errorf("registry contains Go HTML escapes: %s", text)
	}
}

func TestWriteIntentRegistryFailureCausesAndOwnedCleanup(t *testing.T) {
	t.Parallel()

	for _, stage := range []string{"open", "write", "short write", "close", "rename", "cleanup"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()

			cause := errors.New("injected " + stage + " failure")
			cleanupCause := errors.New("injected cleanup failure")
			removed := false
			renamed := false
			expected := cause
			if stage == "short write" {
				expected = io.ErrShortWrite
			}
			replaceCause := cause
			if stage == "cleanup" {
				replaceCause = errors.New("trigger cleanup")
			}
			ops := successfulRegistryWriteOps()
			if stage == "open" {
				ops.openFile = func(string, int, fs.FileMode) (*os.File, error) {
					return nil, cause
				}
			}
			ops.write = func(_ *os.File, data []byte) (int, error) {
				switch stage {
				case "write", "cleanup":
					return 0, replaceCause
				case "short write":
					return len(data) - 1, nil
				default:
					return len(data), nil
				}
			}
			ops.close = func(*os.File) error {
				if stage == "close" {
					return cause
				}
				return nil
			}
			ops.rename = func(string, string) error {
				renamed = true
				if stage == "rename" {
					return cause
				}
				return nil
			}
			ops.remove = func(string) error {
				removed = true
				if stage == "cleanup" {
					return cleanupCause
				}
				return nil
			}
			err := writeIntentRegistry(
				[]json.RawMessage{},
				intentRegistryEntry{UUID: "uuid", Slug: "slug", DirName: "dir", Status: "in-flight"},
				ops,
			)
			if stage == "cleanup" {
				if !errors.Is(err, replaceCause) || !errors.Is(err, cleanupCause) {
					t.Errorf("error %v lost write/cleanup causes", err)
				}
			} else if !errors.Is(err, expected) {
				t.Errorf("error %v lost cause %v", err, expected)
			}
			if renamed != (stage == "rename") {
				t.Errorf("rename called = %t, want %t", renamed, stage == "rename")
			}
			wantRemoved := stage != "open"
			if removed != wantRemoved {
				t.Errorf("owned temp removed = %t, want %t", removed, wantRemoved)
			}
		})
	}
}

func successfulRegistryWriteOps() registryWriteOps {
	return registryWriteOps{
		tempName: func() string { return ".intents-test.tmp" },
		openFile: func(string, int, fs.FileMode) (*os.File, error) { return nil, nil },
		write:    func(_ *os.File, data []byte) (int, error) { return len(data), nil },
		close:    func(*os.File) error { return nil },
		rename:   func(string, string) error { return nil },
		remove:   func(string) error { return nil },
	}
}

type testFileInfo struct {
	mode fs.FileMode
}

func (info testFileInfo) Name() string       { return "test" }
func (info testFileInfo) Size() int64        { return 0 }
func (info testFileInfo) Mode() fs.FileMode  { return info.mode }
func (info testFileInfo) ModTime() time.Time { return time.Time{} }
func (info testFileInfo) IsDir() bool        { return info.mode.IsDir() }
func (info testFileInfo) Sys() any           { return nil }
