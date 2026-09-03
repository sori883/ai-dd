package state

import (
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

func TestWriteInitialRejectsNilRoot(t *testing.T) {
	t.Parallel()

	if err := WriteInitial(nil, Initial{}); err == nil {
		t.Fatal("WriteInitial() error = nil, want nil root error")
	}
}

func TestWriteStateRejectsNilRoot(t *testing.T) {
	t.Parallel()

	err := WriteState(nil, []byte(canonicalStateContent()))
	if !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("WriteState() error = %v, want fs.ErrInvalid", err)
	}
}

func TestWriteStateRejectsMalformedReplacementBeforeFilesystem(t *testing.T) {
	t.Parallel()

	recordDir := t.TempDir()
	root, err := os.OpenRoot(recordDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("Root.Close() error = %v", err)
		}
	})
	oldState := []byte(canonicalStateContent())
	if err := root.WriteFile(stateFile, oldState, 0o600); err != nil {
		t.Fatal(err)
	}

	err = WriteState(root, []byte("not a canonical state"))
	if !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("WriteState() error = %v, want fs.ErrInvalid", err)
	}
	gotState, err := root.ReadFile(stateFile)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gotState, oldState) {
		t.Errorf("state bytes = %q, want unchanged bytes", gotState)
	}
	if _, err := root.Stat(stateFile); err != nil {
		t.Errorf("Root was closed after malformed replacement: %v", err)
	}
}

func TestWriteStateParsesReplacementBeforeTargetInspection(t *testing.T) {
	t.Parallel()

	inspected := false
	ops := successfulStateWriteOps()
	ops.lstat = func(string) (fs.FileInfo, error) {
		inspected = true
		return writeTestFileInfo{mode: 0o600}, nil
	}

	err := writeStateWithOps([]byte("not a canonical state"), ops)
	if !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("writeStateWithOps() error = %v, want fs.ErrInvalid", err)
	}
	if inspected {
		t.Error("target was inspected before replacement parsing")
	}
}

func TestWriteStateChecksWriteBarrierBeforeTemporaryReplacement(t *testing.T) {
	t.Parallel()

	steps := []string{}
	ops := successfulStateWriteOps()
	ops.lstat = func(name string) (fs.FileInfo, error) {
		steps = append(steps, "inspect "+name)
		return writeTestFileInfo{mode: 0o600}, nil
	}
	ops.openFile = func(name string, flags int, mode fs.FileMode) (*os.File, error) {
		steps = append(steps, fmt.Sprintf("open %s %d %04o", name, flags, mode))
		return &os.File{}, nil
	}
	ops.write = func(_ *os.File, data []byte) (int, error) {
		steps = append(steps, "write "+string(data))
		return len(data), nil
	}
	ops.close = func(*os.File) error {
		steps = append(steps, "close")
		return nil
	}
	ops.rename = func(from, to string) error {
		steps = append(steps, "rename "+from+" "+to)
		return nil
	}

	replacement := []byte(canonicalStateContent())
	if err := writeStateWithOps(replacement, ops); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"inspect aidlc-state.md",
		fmt.Sprintf("open aidlc-state.md %d 0000", os.O_WRONLY),
		"close",
		fmt.Sprintf("open .aidlc-state.md.tmp %d 0666", os.O_WRONLY|os.O_CREATE|os.O_EXCL),
		"write " + string(replacement),
		"close",
		"rename .aidlc-state.md.tmp aidlc-state.md",
	}
	if !slices.Equal(steps, want) {
		t.Errorf("steps = %q, want barrier-before-replacement order %q", steps, want)
	}
}

func TestWriteStateRejectsInvalidTargetBeforeTemporaryCreation(t *testing.T) {
	t.Parallel()

	barrierErr := errors.New("injected barrier error")
	inspectErr := errors.New("injected inspect error")
	tests := []struct {
		name        string
		infoMode    fs.FileMode
		lstatErr    error
		openErr     error
		closeErr    error
		wantErr     error
		wantBarrier bool
	}{
		{name: "missing", lstatErr: fs.ErrNotExist, wantErr: fs.ErrNotExist},
		{name: "inspect failure", lstatErr: inspectErr, wantErr: inspectErr},
		{name: "directory", infoMode: fs.ModeDir, wantErr: fs.ErrInvalid},
		{name: "symlink", infoMode: fs.ModeSymlink, wantErr: fs.ErrInvalid},
		{name: "named pipe", infoMode: fs.ModeNamedPipe, wantErr: fs.ErrInvalid},
		{name: "barrier open failure", infoMode: 0o600, openErr: barrierErr, wantErr: barrierErr, wantBarrier: true},
		{name: "barrier close failure", infoMode: 0o600, closeErr: barrierErr, wantErr: barrierErr, wantBarrier: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ops := successfulStateWriteOps()
			barrierOpened := false
			tempOpened := false
			ops.lstat = func(string) (fs.FileInfo, error) {
				if tt.lstatErr != nil {
					return nil, tt.lstatErr
				}
				return writeTestFileInfo{mode: tt.infoMode}, nil
			}
			ops.openFile = func(name string, _ int, _ fs.FileMode) (*os.File, error) {
				if name == stateFile {
					barrierOpened = true
					if tt.openErr != nil {
						return nil, tt.openErr
					}
					return &os.File{}, nil
				}
				tempOpened = true
				return &os.File{}, nil
			}
			ops.close = func(*os.File) error {
				if barrierOpened && tt.closeErr != nil {
					return tt.closeErr
				}
				return nil
			}

			err := writeStateWithOps([]byte(canonicalStateContent()), ops)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
			if barrierOpened != tt.wantBarrier {
				t.Errorf("barrier opened = %t, want %t", barrierOpened, tt.wantBarrier)
			}
			if tempOpened {
				t.Error("temporary file was opened after target rejection")
			}
		})
	}
}

func TestWriteStateFailuresKeepTargetAndJoinCleanupError(t *testing.T) {
	t.Parallel()

	primary := errors.New("injected state update failure")
	cleanup := errors.New("injected temporary cleanup failure")
	tests := []struct {
		name           string
		stage          string
		wantTemp       bool
		wantClose      bool
		wantCleanup    bool
		wantRename     bool
		wantCause      error
		wantCleanupErr bool
	}{
		{name: "create", stage: "create", wantCause: primary},
		{name: "write", stage: "write", wantTemp: true, wantClose: true, wantCleanup: true, wantCause: primary},
		{name: "short write", stage: "short write", wantTemp: true, wantClose: true, wantCleanup: true, wantCause: io.ErrShortWrite},
		{name: "close", stage: "close", wantTemp: true, wantClose: true, wantCleanup: true, wantCause: primary},
		{name: "rename", stage: "rename", wantTemp: true, wantClose: true, wantCleanup: true, wantCause: primary},
		{name: "cleanup", stage: "cleanup", wantTemp: true, wantClose: true, wantCleanup: true, wantCause: primary, wantCleanupErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ops := successfulStateWriteOps()
			tempOpened := false
			tempClosed := false
			closeCalls := 0
			cleanupCalled := false
			renameSucceeded := false
			ops.openFile = func(name string, _ int, _ fs.FileMode) (*os.File, error) {
				if name == stateFile {
					return &os.File{}, nil
				}
				if tt.stage == "create" {
					return nil, primary
				}
				tempOpened = true
				return &os.File{}, nil
			}
			ops.write = func(_ *os.File, data []byte) (int, error) {
				switch tt.stage {
				case "write":
					return 0, primary
				case "short write":
					return len(data) - 1, nil
				default:
					return len(data), nil
				}
			}
			ops.close = func(*os.File) error {
				closeCalls++
				if closeCalls == 1 {
					return nil
				}
				tempClosed = true
				if tt.stage == "close" {
					return primary
				}
				return nil
			}
			ops.rename = func(_, _ string) error {
				if tt.stage == "rename" || tt.stage == "cleanup" {
					return primary
				}
				renameSucceeded = true
				return nil
			}
			ops.remove = func(string) error {
				cleanupCalled = true
				if tt.stage == "cleanup" {
					return cleanup
				}
				return nil
			}

			replacement := []byte(canonicalStateContent())
			original := slices.Clone(replacement)
			err := writeStateWithOps(replacement, ops)
			if !errors.Is(err, tt.wantCause) {
				t.Errorf("error = %v, want cause %v", err, tt.wantCause)
			}
			if tt.wantCleanupErr && !errors.Is(err, cleanup) {
				t.Errorf("error = %v, want cleanup cause", err)
			}
			if tempOpened != tt.wantTemp {
				t.Errorf("temporary opened = %t, want %t", tempOpened, tt.wantTemp)
			}
			if tempClosed != tt.wantClose {
				t.Errorf("temporary closed = %t, want %t", tempClosed, tt.wantClose)
			}
			if cleanupCalled != tt.wantCleanup {
				t.Errorf("temporary cleanup called = %t, want %t", cleanupCalled, tt.wantCleanup)
			}
			if renameSucceeded != tt.wantRename {
				t.Errorf("rename succeeded = %t, want %t", renameSucceeded, tt.wantRename)
			}
			if !slices.Equal(replacement, original) {
				t.Error("replacement bytes were mutated")
			}
		})
	}
}

func TestWriteStateRetriesTemporaryCollisionsWithoutRemovingCollision(t *testing.T) {
	t.Parallel()

	attempts := 0
	opened := []string{}
	removed := []string{}
	ops := successfulStateWriteOps()
	ops.tempName = func(string) string {
		attempts++
		if attempts == 1 {
			return ".occupied-aidlc-state.md.tmp"
		}
		return ".free-aidlc-state.md.tmp"
	}
	ops.openFile = func(name string, _ int, _ fs.FileMode) (*os.File, error) {
		opened = append(opened, name)
		if name == ".occupied-aidlc-state.md.tmp" {
			return nil, fs.ErrExist
		}
		return &os.File{}, nil
	}
	ops.remove = func(name string) error {
		removed = append(removed, name)
		return nil
	}

	if err := writeStateWithOps([]byte(canonicalStateContent()), ops); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Errorf("temporary attempts = %d, want one collision retry", attempts)
	}
	wantOpened := []string{stateFile, ".occupied-aidlc-state.md.tmp", ".free-aidlc-state.md.tmp"}
	if !slices.Equal(opened, wantOpened) {
		t.Errorf("opened paths = %q, want %q", opened, wantOpened)
	}
	if len(removed) != 0 {
		t.Errorf("removed paths = %q, want no cleanup after success", removed)
	}
}

func TestWriteStateStopsAfterTemporaryCollisionBudget(t *testing.T) {
	t.Parallel()

	attempts := 0
	removed := false
	ops := successfulStateWriteOps()
	ops.tempName = func(string) string {
		attempts++
		return ".occupied-aidlc-state.md.tmp"
	}
	ops.openFile = func(name string, _ int, _ fs.FileMode) (*os.File, error) {
		if name == stateFile {
			return &os.File{}, nil
		}
		return nil, fs.ErrExist
	}
	ops.remove = func(string) error {
		removed = true
		return nil
	}

	err := writeStateWithOps([]byte(canonicalStateContent()), ops)
	if !errors.Is(err, fs.ErrExist) {
		t.Errorf("error = %v, want fs.ErrExist", err)
	}
	if attempts != stateWriteTempAttempts {
		t.Errorf("collision attempts = %d, want %d", attempts, stateWriteTempAttempts)
	}
	if removed {
		t.Error("collision temporary was removed")
	}
}

func TestWriteInitialWritesSidecarBeforeState(t *testing.T) {
	t.Parallel()

	steps := []string{}
	ops := successfulInitialWriteOps()
	ops.lstat = func(name string) (fs.FileInfo, error) {
		steps = append(steps, "inspect "+name)
		return nil, fs.ErrNotExist
	}
	ops.openFile = func(name string, flags int, mode fs.FileMode) (*os.File, error) {
		steps = append(steps, fmt.Sprintf("open %s %d %04o", name, flags, mode))
		return nil, nil
	}
	ops.write = func(_ *os.File, data []byte) (int, error) {
		steps = append(steps, "write "+string(data))
		return len(data), nil
	}
	ops.close = func(*os.File) error {
		steps = append(steps, "close")
		return nil
	}
	ops.rename = func(from, to string) error {
		steps = append(steps, "rename "+from+" "+to)
		return nil
	}
	ops.remove = func(name string) error {
		steps = append(steps, "remove "+name)
		return nil
	}

	initial := Initial{
		ProjectDescriptionJSON: "description bytes",
		StateContent:           "state bytes",
	}
	if err := writeInitialWithOps(initial, ops); err != nil {
		t.Fatal(err)
	}
	want := []string{
		fmt.Sprintf("open .project-description.json.tmp %d 0666", os.O_WRONLY|os.O_CREATE|os.O_EXCL),
		"write description bytes",
		"close",
		"rename .project-description.json.tmp project-description.json",
		"inspect aidlc-state.md",
		fmt.Sprintf("open .aidlc-state.md.tmp %d 0666", os.O_WRONLY|os.O_CREATE|os.O_EXCL),
		"write state bytes",
		"close",
		"rename .aidlc-state.md.tmp aidlc-state.md",
	}
	if !slices.Equal(steps, want) {
		t.Errorf("steps = %q, want sidecar-before-state order %q", steps, want)
	}
}

func TestWriteInitialChecksExistingStateWriteBarrierAfterSidecar(t *testing.T) {
	t.Parallel()

	steps := []string{}
	ops := successfulInitialWriteOps()
	ops.lstat = func(name string) (fs.FileInfo, error) {
		steps = append(steps, "inspect "+name)
		return writeTestFileInfo{mode: 0o600}, nil
	}
	ops.openFile = func(name string, flags int, mode fs.FileMode) (*os.File, error) {
		steps = append(steps, fmt.Sprintf("open %s %d %04o", name, flags, mode))
		return nil, nil
	}
	ops.close = func(*os.File) error {
		steps = append(steps, "close")
		return nil
	}
	ops.write = func(_ *os.File, data []byte) (int, error) {
		steps = append(steps, "write "+string(data))
		return len(data), nil
	}
	ops.rename = func(from, to string) error {
		steps = append(steps, "rename "+from+" "+to)
		return nil
	}

	if err := writeInitialWithOps(Initial{
		ProjectDescriptionJSON: "description",
		StateContent:           "state",
	}, ops); err != nil {
		t.Fatal(err)
	}
	want := []string{
		fmt.Sprintf("open .project-description.json.tmp %d 0666", os.O_WRONLY|os.O_CREATE|os.O_EXCL),
		"write description",
		"close",
		"rename .project-description.json.tmp project-description.json",
		"inspect aidlc-state.md",
		fmt.Sprintf("open aidlc-state.md %d 0000", os.O_WRONLY),
		"close",
		fmt.Sprintf("open .aidlc-state.md.tmp %d 0666", os.O_WRONLY|os.O_CREATE|os.O_EXCL),
		"write state",
		"close",
		"rename .aidlc-state.md.tmp aidlc-state.md",
	}
	if !slices.Equal(steps, want) {
		t.Errorf("steps = %q, want sidecar then barrier then state %q", steps, want)
	}
}

func TestWriteInitialStateBarrierFailuresKeepStateUntouched(t *testing.T) {
	t.Parallel()

	cause := errors.New("injected state barrier failure")
	tests := []struct {
		name       string
		mode       fs.FileMode
		inspectErr error
		openErr    error
		closeErr   error
		wantErr    error
		wantOpen   bool
	}{
		{name: "directory", mode: fs.ModeDir, wantErr: fs.ErrInvalid},
		{name: "symlink", mode: fs.ModeSymlink, wantErr: fs.ErrInvalid},
		{name: "named pipe", mode: fs.ModeNamedPipe, wantErr: fs.ErrInvalid},
		{name: "inspect failure", inspectErr: cause, wantErr: cause},
		{name: "barrier open failure", mode: 0o600, openErr: cause, wantErr: cause, wantOpen: true},
		{name: "barrier close failure", mode: 0o600, closeErr: cause, wantErr: cause, wantOpen: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ops := successfulInitialWriteOps()
			opened := false
			barrierOpen := false
			sidecarCommitted := false
			stateMutated := false
			ops.lstat = func(string) (fs.FileInfo, error) {
				if tt.inspectErr != nil {
					return nil, tt.inspectErr
				}
				return writeTestFileInfo{mode: tt.mode}, nil
			}
			ops.openFile = func(name string, flags int, mode fs.FileMode) (*os.File, error) {
				if name != stateFile {
					return nil, nil
				}
				opened = true
				barrierOpen = true
				if tt.openErr != nil {
					return nil, tt.openErr
				}
				return nil, nil
			}
			ops.write = func(_ *os.File, data []byte) (int, error) {
				return len(data), nil
			}
			ops.close = func(*os.File) error {
				if barrierOpen && tt.closeErr != nil {
					return tt.closeErr
				}
				return nil
			}
			ops.rename = func(_, to string) error {
				switch to {
				case projectDescriptionFile:
					sidecarCommitted = true
				case stateFile:
					stateMutated = true
				}
				return nil
			}

			err := writeInitialWithOps(Initial{
				ProjectDescriptionJSON: "description",
				StateContent:           "state",
			}, ops)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want cause %v", err, tt.wantErr)
			}
			if opened != tt.wantOpen {
				t.Errorf("barrier opened = %t, want %t", opened, tt.wantOpen)
			}
			if !sidecarCommitted {
				t.Error("sidecar was not committed before state barrier failure")
			}
			if stateMutated {
				t.Error("state was mutated after barrier failure")
			}
		})
	}
}

func TestWriteInitialAcceptsEmptyPayloads(t *testing.T) {
	t.Parallel()

	var written []int
	ops := successfulInitialWriteOps()
	ops.lstat = func(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist }
	ops.write = func(_ *os.File, data []byte) (int, error) {
		written = append(written, len(data))
		return len(data), nil
	}
	if err := writeInitialWithOps(Initial{}, ops); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(written, []int{0, 0}) {
		t.Errorf("written lengths = %v, want empty sidecar and state payloads", written)
	}
}

func TestWriteInitialSidecarFailuresLeaveStateUntouched(t *testing.T) {
	t.Parallel()

	primary := errors.New("injected sidecar failure")
	cleanup := errors.New("injected cleanup failure")
	tests := []struct {
		name        string
		stage       string
		wantCleanup bool
	}{
		{name: "create", stage: "create"},
		{name: "write", stage: "write"},
		{name: "short write", stage: "short write"},
		{name: "close", stage: "close"},
		{name: "rename", stage: "rename"},
		{name: "cleanup", stage: "cleanup", wantCleanup: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ops := successfulInitialWriteOps()
			stateMutation := false
			sidecarCommitted := false
			currentTarget := ""
			ops.lstat = func(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist }
			ops.tempName = func(target string) string { return "." + target + ".tmp" }
			ops.openFile = func(name string, _ int, _ fs.FileMode) (*os.File, error) {
				switch name {
				case ".project-description.json.tmp":
					currentTarget = projectDescriptionFile
					if tt.stage == "create" {
						return nil, primary
					}
				case ".aidlc-state.md.tmp":
					currentTarget = stateFile
					stateMutation = true
				}
				return nil, nil
			}
			ops.write = func(_ *os.File, data []byte) (int, error) {
				if currentTarget == projectDescriptionFile {
					switch tt.stage {
					case "write":
						return 0, primary
					case "short write":
						return len(data) - 1, nil
					}
				}
				return len(data), nil
			}
			ops.close = func(*os.File) error {
				if currentTarget == projectDescriptionFile && tt.stage == "close" {
					return primary
				}
				return nil
			}
			ops.rename = func(from, to string) error {
				if to == stateFile {
					stateMutation = true
				}
				if to == projectDescriptionFile {
					if tt.stage == "rename" || tt.stage == "cleanup" {
						return primary
					}
					sidecarCommitted = true
				}
				return nil
			}
			ops.remove = func(name string) error {
				if name == ".aidlc-state.md.tmp" {
					stateMutation = true
				}
				if tt.wantCleanup {
					return cleanup
				}
				return nil
			}

			err := writeInitialWithOps(Initial{
				ProjectDescriptionJSON: "description",
				StateContent:           "state",
			}, ops)
			wantCause := primary
			if tt.stage == "short write" {
				wantCause = io.ErrShortWrite
			}
			if !errors.Is(err, wantCause) {
				t.Errorf("error = %v, want sidecar cause %v", err, wantCause)
			}
			if tt.wantCleanup && !errors.Is(err, cleanup) {
				t.Errorf("error = %v, want cleanup cause", err)
			}
			if stateMutation {
				t.Error("state was mutated after sidecar failure")
			}
			if sidecarCommitted {
				t.Error("sidecar was committed despite its failure")
			}
		})
	}
}

func TestWriteInitialStateFailuresKeepCommittedSidecarAndOldState(t *testing.T) {
	t.Parallel()

	primary := errors.New("injected state failure")
	cleanup := errors.New("injected cleanup failure")
	tests := []struct {
		name        string
		stage       string
		wantCleanup bool
	}{
		{name: "create", stage: "create"},
		{name: "write", stage: "write"},
		{name: "short write", stage: "short write"},
		{name: "close", stage: "close"},
		{name: "rename", stage: "rename"},
		{name: "cleanup", stage: "cleanup", wantCleanup: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ops := successfulInitialWriteOps()
			sidecarCommitted := false
			stateRenamed := false
			currentTarget := ""
			ops.lstat = func(string) (fs.FileInfo, error) {
				return writeTestFileInfo{mode: 0o600}, nil
			}
			ops.tempName = func(target string) string { return "." + target + ".tmp" }
			ops.openFile = func(name string, _ int, _ fs.FileMode) (*os.File, error) {
				switch name {
				case stateFile:
					return nil, nil
				case ".project-description.json.tmp":
					currentTarget = projectDescriptionFile
				case ".aidlc-state.md.tmp":
					currentTarget = stateFile
					if tt.stage == "create" {
						return nil, primary
					}
				}
				return nil, nil
			}
			ops.write = func(_ *os.File, data []byte) (int, error) {
				if currentTarget == stateFile {
					switch tt.stage {
					case "write":
						return 0, primary
					case "short write":
						return len(data) - 1, nil
					}
				}
				return len(data), nil
			}
			ops.close = func(*os.File) error {
				if currentTarget == stateFile && tt.stage == "close" {
					return primary
				}
				return nil
			}
			ops.rename = func(_, to string) error {
				switch to {
				case projectDescriptionFile:
					sidecarCommitted = true
				case stateFile:
					if tt.stage == "rename" || tt.stage == "cleanup" {
						return primary
					}
					stateRenamed = true
				}
				return nil
			}
			ops.remove = func(name string) error {
				if name == ".aidlc-state.md.tmp" && tt.wantCleanup {
					return cleanup
				}
				return nil
			}

			err := writeInitialWithOps(Initial{
				ProjectDescriptionJSON: "description",
				StateContent:           "state",
			}, ops)
			wantCause := primary
			if tt.stage == "short write" {
				wantCause = io.ErrShortWrite
			}
			if !errors.Is(err, wantCause) {
				t.Errorf("error = %v, want state cause %v", err, wantCause)
			}
			if tt.wantCleanup && !errors.Is(err, cleanup) {
				t.Errorf("error = %v, want cleanup cause", err)
			}
			if !sidecarCommitted {
				t.Error("sidecar was not committed before state failure")
			}
			if stateRenamed {
				t.Error("old state was replaced despite state failure")
			}
		})
	}
}

func TestWriteInitialRetriesTempCollisionsWithoutRemovingCollisions(t *testing.T) {
	t.Parallel()

	attempts := map[string]int{}
	opened := []string{}
	ops := successfulInitialWriteOps()
	ops.lstat = func(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist }
	ops.tempName = func(target string) string {
		attempts[target]++
		if attempts[target] == 1 {
			return ".occupied-" + target + ".tmp"
		}
		return ".free-" + target + ".tmp"
	}
	ops.openFile = func(name string, _ int, _ fs.FileMode) (*os.File, error) {
		opened = append(opened, name)
		if strings.HasPrefix(name, ".occupied-") {
			return nil, fs.ErrExist
		}
		return nil, nil
	}
	ops.remove = func(name string) error {
		if strings.HasPrefix(name, ".occupied-") {
			t.Errorf("collision temp %q was removed", name)
		}
		return nil
	}

	if err := writeInitialWithOps(Initial{
		ProjectDescriptionJSON: "description",
		StateContent:           "state",
	}, ops); err != nil {
		t.Fatal(err)
	}
	if attempts[projectDescriptionFile] != 2 || attempts[stateFile] != 2 {
		t.Errorf("temp attempts = %v, want one collision retry per file", attempts)
	}
	wantOpened := []string{
		".occupied-project-description.json.tmp",
		".free-project-description.json.tmp",
		".occupied-aidlc-state.md.tmp",
		".free-aidlc-state.md.tmp",
	}
	if !slices.Equal(opened, wantOpened) {
		t.Errorf("opened temps = %q, want collision-preserving retries %q", opened, wantOpened)
	}
}

func TestWriteInitialStopsAfterTempCollisionBudgetWithoutCleanup(t *testing.T) {
	t.Parallel()

	attempts := 0
	removed := false
	ops := successfulInitialWriteOps()
	ops.lstat = func(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist }
	ops.tempName = func(target string) string { return ".occupied-" + target + ".tmp" }
	ops.openFile = func(string, int, fs.FileMode) (*os.File, error) {
		attempts++
		return nil, fs.ErrExist
	}
	ops.remove = func(string) error {
		removed = true
		return nil
	}

	err := writeInitialWithOps(Initial{
		ProjectDescriptionJSON: "description",
		StateContent:           "state",
	}, ops)
	if !errors.Is(err, fs.ErrExist) {
		t.Errorf("error = %v, want collision cause", err)
	}
	if attempts != initialWriteTempAttempts {
		t.Errorf("collision attempts = %d, want %d", attempts, initialWriteTempAttempts)
	}
	if removed {
		t.Error("unowned collision temp was removed")
	}
}

func successfulInitialWriteOps() initialWriteOps {
	return initialWriteOps{
		tempName: func(target string) string { return "." + target + ".tmp" },
		lstat:    func(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist },
		openFile: func(string, int, fs.FileMode) (*os.File, error) { return nil, nil },
		write:    func(_ *os.File, data []byte) (int, error) { return len(data), nil },
		close:    func(*os.File) error { return nil },
		rename:   func(string, string) error { return nil },
		remove:   func(string) error { return nil },
	}
}

func successfulStateWriteOps() stateWriteOps {
	return stateWriteOps{
		tempName: func(string) string { return ".aidlc-state.md.tmp" },
		lstat:    func(string) (fs.FileInfo, error) { return writeTestFileInfo{mode: 0o600}, nil },
		openFile: func(string, int, fs.FileMode) (*os.File, error) { return &os.File{}, nil },
		write:    func(_ *os.File, data []byte) (int, error) { return len(data), nil },
		close:    func(*os.File) error { return nil },
		rename:   func(string, string) error { return nil },
		remove:   func(string) error { return nil },
	}
}

type writeTestFileInfo struct {
	mode fs.FileMode
}

func (info writeTestFileInfo) Name() string       { return "test" }
func (info writeTestFileInfo) Size() int64        { return 0 }
func (info writeTestFileInfo) Mode() fs.FileMode  { return info.mode }
func (info writeTestFileInfo) ModTime() time.Time { return time.Time{} }
func (info writeTestFileInfo) IsDir() bool        { return info.mode.IsDir() }
func (info writeTestFileInfo) Sys() any           { return nil }
