package workspace

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCreateSpaceRejectsRelativeRoot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input RootInput
	}{
		{name: "empty root input"},
		{name: "relative working directory", input: RootInput{WorkingDir: "relative"}},
		{name: "relative explicit without working directory", input: RootInput{ExplicitDir: "project"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := CreateSpace(tt.input, "Team Alpha")
			if !errors.Is(err, fs.ErrInvalid) || got != "" {
				t.Errorf("CreateSpace() = (%q, %v), want empty name and fs.ErrInvalid", got, err)
			}
		})
	}
}

func TestCreateSpaceValidatesNameBeforeOpen(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "empty"},
		{name: "raw short help", raw: "-h"},
		{name: "normalized reserved name", raw: " CREATE!! "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := createSpace(
				RootInput{ExplicitDir: filepath.Join(t.TempDir(), "unopened-project")},
				tt.raw,
				func(string) (*os.Root, error) {
					t.Error("project opened for invalid name")
					return nil, fs.ErrPermission
				},
				(*os.Root).Close,
				populateSpace,
			)
			if !errors.Is(err, fs.ErrInvalid) || got != "" {
				t.Errorf("createSpace() = (%q, %v), want empty name and fs.ErrInvalid", got, err)
			}
		})
	}
}

func TestCreateSpaceProjectOpenError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cause error
	}{
		{name: "missing project", cause: fs.ErrNotExist},
		{name: "permission denied", cause: fs.ErrPermission},
		{name: "other error", cause: errors.New("injected project open failure")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			projectPath := filepath.Join(base, "chosen")
			opened := []string{}
			got, err := createSpace(
				RootInput{ExplicitDir: projectPath, AIDLCProjectDir: base, WorkingDir: base},
				"Team Alpha",
				func(name string) (*os.Root, error) {
					opened = append(opened, name)
					return nil, tt.cause
				},
				func(*os.Root) error {
					t.Error("close called without an acquired project root")
					return nil
				},
				populateSpace,
			)
			if !errors.Is(err, tt.cause) || got != "" {
				t.Errorf(
					"createSpace() = (%q, %v), want empty name and cause %v",
					got,
					err,
					tt.cause,
				)
			}
			if !slices.Equal(opened, []string{projectPath}) {
				t.Errorf("opened = %q, want only %q", opened, projectPath)
			}
		})
	}
}

func TestReadDefaultOrganizationFailures(t *testing.T) {
	t.Parallel()

	readFailure := errors.New("injected organization read failure")
	closeFailure := errors.New("injected organization close failure")
	tests := []struct {
		name      string
		openErr   error
		readErr   error
		closeErr  error
		want      string
		wantCause []error
	}{
		{name: "confirmed absence", openErr: fs.ErrNotExist, want: "# Organization defaults\n"},
		{name: "open permission", openErr: fs.ErrPermission, wantCause: []error{fs.ErrPermission}},
		{name: "partial read data is discarded", readErr: readFailure, wantCause: []error{readFailure}},
		{name: "read not exist is not absence", readErr: fs.ErrNotExist, wantCause: []error{fs.ErrNotExist}},
		{name: "close failure discards data", closeErr: closeFailure, wantCause: []error{closeFailure}},
		{
			name:      "read and close failures joined",
			readErr:   readFailure,
			closeErr:  closeFailure,
			wantCause: []error{readFailure, closeFailure},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			closeCalls := 0
			got, err := readDefaultOrganization(func(name string) (io.ReadCloser, error) {
				if want := filepath.Join(
					"aidlc",
					"spaces",
					"default",
					"memory",
					"org.md",
				); name != want {
					t.Errorf("open path = %q, want %q", name, want)
				}
				if tt.openErr != nil {
					return nil, tt.openErr
				}
				return spaceTestReadCloser{
					read: func(p []byte) (int, error) {
						readErr := tt.readErr
						if readErr == nil {
							readErr = io.EOF
						}
						return copy(p, "partial organization"), readErr
					},
					close: func() error {
						closeCalls++
						return tt.closeErr
					},
				}, nil
			})
			if got != tt.want {
				t.Errorf("organization content = %q, want %q", got, tt.want)
			}
			if len(tt.wantCause) == 0 && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			for _, cause := range tt.wantCause {
				if !errors.Is(err, cause) {
					t.Errorf("error = %v, want cause %v", err, cause)
				}
			}
			wantCloseCalls := 1
			if tt.openErr != nil {
				wantCloseCalls = 0
			}
			if closeCalls != wantCloseCalls {
				t.Errorf("close calls = %d, want %d", closeCalls, wantCloseCalls)
			}
		})
	}
}

func TestWriteSpaceFileFailures(t *testing.T) {
	t.Parallel()

	writeFailure := errors.New("injected space write failure")
	closeFailure := errors.New("injected space file close failure")
	tests := []struct {
		name      string
		openErr   error
		writeErr  error
		closeErr  error
		short     bool
		wantCause []error
	}{
		{name: "success"},
		{name: "existing target", openErr: fs.ErrExist, wantCause: []error{fs.ErrExist}},
		{name: "open permission", openErr: fs.ErrPermission, wantCause: []error{fs.ErrPermission}},
		{name: "partial write failure", writeErr: writeFailure, wantCause: []error{writeFailure}},
		{name: "close failure", closeErr: closeFailure, wantCause: []error{closeFailure}},
		{
			name:      "write and close failures joined",
			writeErr:  writeFailure,
			closeErr:  closeFailure,
			wantCause: []error{writeFailure, closeFailure},
		},
		{name: "short write", short: true, wantCause: []error{io.ErrShortWrite}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			closeCalls := 0
			writeCalls := 0
			err := writeSpaceFile("new-file", "content", func(name string, flags int, mode fs.FileMode) (io.WriteCloser, error) {
				if name != "new-file" || flags != os.O_CREATE|os.O_EXCL|os.O_WRONLY || mode != 0o666 {
					t.Errorf(
						"OpenFile(%q, %d, %v) did not request exclusive creation",
						name,
						flags,
						mode,
					)
				}
				if tt.openErr != nil {
					return nil, tt.openErr
				}
				return spaceTestWriteCloser{
					write: func(p []byte) (int, error) {
						writeCalls++
						if string(p) != "content" {
							t.Errorf("written content = %q, want content", p)
						}
						if tt.short || tt.writeErr != nil {
							return 2, tt.writeErr
						}
						return len(p), nil
					},
					close: func() error {
						closeCalls++
						return tt.closeErr
					},
				}, nil
			})
			if len(tt.wantCause) == 0 && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			for _, cause := range tt.wantCause {
				if !errors.Is(err, cause) {
					t.Errorf("error = %v, want cause %v", err, cause)
				}
			}
			wantCalls := 1
			if tt.openErr != nil {
				wantCalls = 0
			}
			if closeCalls != wantCalls || writeCalls != wantCalls {
				t.Errorf(
					"write/close calls = %d/%d, want %d each",
					writeCalls,
					closeCalls,
					wantCalls,
				)
			}
		})
	}
}

type spaceTestReadCloser struct {
	read  func([]byte) (int, error)
	close func() error
}

func (f spaceTestReadCloser) Read(p []byte) (int, error) { return f.read(p) }
func (f spaceTestReadCloser) Close() error               { return f.close() }

type spaceTestWriteCloser struct {
	write func([]byte) (int, error)
	close func() error
}

func (f spaceTestWriteCloser) Write(p []byte) (int, error) { return f.write(p) }
func (f spaceTestWriteCloser) Close() error                { return f.close() }

func TestNormalizeSpaceNameWords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		expected string
	}{
		{name: "existing slug", raw: "team-alpha", expected: "team-alpha"},
		{name: "default is allowed", raw: "Default", expected: "default"},
		{name: "raw help guard is case sensitive", raw: "-H", expected: "h"},
		{name: "uppercase words", raw: "Team Alpha", expected: "team-alpha"},
		{name: "collapsed and trimmed separators", raw: "  Platform__TEAM!! ", expected: "platform-team"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeSpaceName(tt.raw)
			if err != nil || got != tt.expected {
				t.Errorf(
					"normalizeSpaceName() = (%q, %v), want (%q, nil)",
					got,
					err,
					tt.expected,
				)
			}
		})
	}
}

func TestNormalizeSpaceNameLeadingLetter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		expected string
	}{
		{name: "numeric start", raw: "2026 Platform", expected: "intent-2026-platform"},
		{name: "whitespace only", raw: " \t\n", expected: "intent"},
		{name: "symbols only", raw: "!?__", expected: "intent"},
		{name: "non ascii only", raw: "東京", expected: "intent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeSpaceName(tt.raw)
			if err != nil || got != tt.expected {
				t.Errorf(
					"normalizeSpaceName() = (%q, %v), want (%q, nil)",
					got,
					err,
					tt.expected,
				)
			}
		})
	}
}

func TestNormalizeSpaceNameLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		expected string
	}{
		{name: "forty seven", raw: strings.Repeat("a", 47), expected: strings.Repeat("a", 47)},
		{name: "forty eight", raw: strings.Repeat("a", 48), expected: strings.Repeat("a", 48)},
		{name: "forty nine", raw: strings.Repeat("a", 49), expected: strings.Repeat("a", 48)},
		{
			name:     "trim separator after truncation",
			raw:      strings.Repeat("a", 47) + "-b",
			expected: strings.Repeat("a", 47),
		},
		{
			name:     "prefix follows truncation",
			raw:      strings.Repeat("7", 49),
			expected: "intent-" + strings.Repeat("7", 48),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeSpaceName(tt.raw)
			if err != nil || got != tt.expected {
				t.Errorf(
					"normalizeSpaceName() = (%q, %v), want (%q, nil)",
					got,
					err,
					tt.expected,
				)
			}
		})
	}
}

func TestNormalizeSpaceNameUnicodeLowercase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		expected string
	}{
		{name: "kelvin sign", raw: "AKB", expected: "akb"},
		{name: "dotted capital at start", raw: "İB", expected: "i-b"},
		{name: "dotted capital in middle", raw: "AİB", expected: "ai-b"},
		{name: "dotted capital word", raw: "İstanbul", expected: "i-stanbul"},
		{name: "non ascii letters remain separators", raw: "Äpfel und Öl", expected: "pfel-und-l"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeSpaceName(tt.raw)
			if err != nil || got != tt.expected {
				t.Errorf(
					"normalizeSpaceName() = (%q, %v), want (%q, nil)",
					got,
					err,
					tt.expected,
				)
			}
		})
	}
}

func TestNormalizeSpaceNameInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "empty"},
		{name: "raw help", raw: "help"},
		{name: "raw short help", raw: "-h"},
		{name: "normalized help", raw: " HELP!! "},
		{name: "list", raw: "list"},
		{name: "switch", raw: "switch"},
		{name: "create", raw: "create"},
		{name: "archive", raw: "archive"},
		{name: "rename", raw: "rename"},
		{name: "show", raw: "show"},
		{name: "birth", raw: "birth"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeSpaceName(tt.raw)
			if !errors.Is(err, fs.ErrInvalid) || got != "" {
				t.Errorf("normalizeSpaceName() = (%q, %v), want empty name and fs.ErrInvalid", got, err)
			}
		})
	}
}
