package steering

import (
	"bytes"
	cryptorand "crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestReadOrCreateContinuationKeyReusesFreshCanonicalFile(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "project")
	recordPath := filepath.Join(root, "record")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("create project directory: %v", err)
	}
	if err := os.MkdirAll(recordPath, 0o755); err != nil {
		t.Fatalf("create record directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recordPath, "aidlc-state.md"), []byte("# state\n"), 0o644); err != nil {
		t.Fatalf("create state file: %v", err)
	}

	projectRoot, err := os.OpenRoot(projectPath)
	if err != nil {
		t.Fatalf("open project root: %v", err)
	}
	defer projectRoot.Close()
	recordRoot, err := os.OpenRoot(recordPath)
	if err != nil {
		t.Fatalf("open record root: %v", err)
	}
	defer recordRoot.Close()

	keyPath := filepath.Join(recordPath, ".aidlc-steering-token-key")
	keyA := bytes.Repeat([]byte{0x11}, 32)
	keyB := bytes.Repeat([]byte{0xee}, 32)
	writeCanonicalKey := func(key []byte) []byte {
		t.Helper()
		data := []byte(base64.RawURLEncoding.EncodeToString(key) + "\n")
		if err := os.WriteFile(keyPath, data, 0o640); err != nil {
			t.Fatalf("write canonical key file: %v", err)
		}
		return data
	}

	wantFileA := writeCanonicalKey(keyA)
	beforeInfo, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat initial key file: %v", err)
	}
	beforeMode := beforeInfo.Mode().Perm()

	gotA, err := ReadOrCreateContinuationKey(projectRoot, recordRoot)
	if err != nil {
		t.Fatalf("ReadOrCreateContinuationKey() with existing key error = %v", err)
	}
	if !bytes.Equal(gotA, keyA) {
		t.Fatalf("first key = %x, want %x", gotA, keyA)
	}
	gotA[0] ^= 0xff
	fileBytes, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read first key file: %v", err)
	}
	if !bytes.Equal(fileBytes, wantFileA) {
		t.Errorf("first key file changed after return mutation = %q, want %q", fileBytes, wantFileA)
	}
	afterInfo, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat first key file after call: %v", err)
	}
	if got := afterInfo.Mode().Perm(); got != beforeMode {
		t.Errorf("first key file mode = %o, want unchanged %o", got, beforeMode)
	}

	wantFileB := writeCanonicalKey(keyB)
	gotB, err := ReadOrCreateContinuationKey(projectRoot, recordRoot)
	if err != nil {
		t.Fatalf("ReadOrCreateContinuationKey() after key replacement error = %v", err)
	}
	if !bytes.Equal(gotB, keyB) {
		t.Fatalf("second key = %x, want %x", gotB, keyB)
	}
	gotB[0] ^= 0xff
	fileBytes, err = os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read second key file: %v", err)
	}
	if !bytes.Equal(fileBytes, wantFileB) {
		t.Errorf("second key file changed after return mutation = %q, want %q", fileBytes, wantFileB)
	}

	gotAgain, err := ReadOrCreateContinuationKey(projectRoot, recordRoot)
	if err != nil {
		t.Fatalf("ReadOrCreateContinuationKey() after return mutation error = %v", err)
	}
	if !bytes.Equal(gotAgain, keyB) {
		t.Errorf("third key = %x, want fresh %x", gotAgain, keyB)
	}

	projectKeyPath := filepath.Join(projectPath, "aidlc", ".aidlc-sessions", ".aidlc-steering-token-key")
	if _, err := os.Stat(projectKeyPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("project session key stat error = %v, want os.ErrNotExist", err)
	}
	if _, err := projectRoot.Stat("."); err != nil {
		t.Errorf("project root unavailable after calls: %v", err)
	}
	if _, err := recordRoot.Stat("aidlc-state.md"); err != nil {
		t.Errorf("record root unavailable after calls: %v", err)
	}
}

func TestReadContinuationKeyIsReadOnlyAndUsesCanonicalBounds(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "project")
	recordPath := filepath.Join(root, "record")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("create project directory: %v", err)
	}
	if err := os.MkdirAll(recordPath, 0o755); err != nil {
		t.Fatalf("create record directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recordPath, "aidlc-state.md"), []byte("# state\n"), 0o644); err != nil {
		t.Fatalf("create state file: %v", err)
	}
	projectRoot, err := os.OpenRoot(projectPath)
	if err != nil {
		t.Fatalf("open project root: %v", err)
	}
	defer projectRoot.Close()
	recordRoot, err := os.OpenRoot(recordPath)
	if err != nil {
		t.Fatalf("open record root: %v", err)
	}
	defer recordRoot.Close()

	key := bytes.Repeat([]byte{0x42}, 32)
	canonical := []byte(base64.RawURLEncoding.EncodeToString(key) + "\n")
	wrapped := append([]byte("\u2003"), canonical...)
	wrapped = append(wrapped, []byte("\u2003")...)
	keyPath := filepath.Join(recordPath, ".aidlc-steering-token-key")
	if err := os.WriteFile(keyPath, wrapped, 0o640); err != nil {
		t.Fatalf("write wrapped key: %v", err)
	}

	got, err := ReadContinuationKey(projectRoot, recordRoot)
	if err != nil {
		t.Fatalf("ReadContinuationKey() error = %v", err)
	}
	if !bytes.Equal(got, key) {
		t.Fatalf("ReadContinuationKey() = %x, want %x", got, key)
	}
	got[0] ^= 0xff
	fileBytes, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key after returned slice mutation: %v", err)
	}
	if !bytes.Equal(fileBytes, wrapped) {
		t.Errorf("read-only key file changed = %q, want %q", fileBytes, wrapped)
	}
	if _, err := os.Stat(filepath.Join(projectPath, "aidlc", ".aidlc-sessions")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("read-only session directory stat error = %v, want os.ErrNotExist", err)
	}

	if err := os.WriteFile(keyPath, bytes.Repeat([]byte{'A'}, continuationKeyFileMaxSize+1), 0o640); err != nil {
		t.Fatalf("write oversize key: %v", err)
	}
	if _, err := ReadContinuationKey(projectRoot, recordRoot); err == nil || !errors.Is(err, ErrInvalidContinuationKeyFile) {
		t.Fatalf("ReadContinuationKey(oversize) error = %v, want ErrInvalidContinuationKeyFile", err)
	}
}

func TestReadOrCreateContinuationKeyRejectsCorruptFile(t *testing.T) {
	validKey := bytes.Repeat([]byte{0x42}, 32)
	validText := base64.RawURLEncoding.EncodeToString(validKey)
	standardText := base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{0xff}, 32))
	tests := []struct {
		name string
		file []byte
	}{
		{name: "padding", file: []byte(validText + "=\n")},
		{name: "short key", file: []byte(base64.RawURLEncoding.EncodeToString(validKey[:31]) + "\n")},
		{name: "long key", file: []byte(base64.RawURLEncoding.EncodeToString(append(append([]byte(nil), validKey...), 0x43)) + "\n")},
		{name: "standard base64 alphabet", file: []byte(standardText + "\n")},
		{name: "trailing non-whitespace", file: []byte(validText + "!\n")},
		{name: "invalid UTF-8", file: append([]byte{0xff}, []byte(validText+"\n")...)},
		{name: "empty and whitespace only", file: []byte(" \t\u2003\n")},
		{name: "bounded size exceeded", file: bytes.Repeat([]byte{'a'}, 1<<20)},
		{name: "non ECMAScript whitespace", file: []byte("\u0085" + validText + "\u0085")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			projectPath := filepath.Join(root, "project")
			recordPath := filepath.Join(root, "record")
			if err := os.MkdirAll(projectPath, 0o755); err != nil {
				t.Fatalf("create project directory: %v", err)
			}
			if err := os.MkdirAll(recordPath, 0o755); err != nil {
				t.Fatalf("create record directory: %v", err)
			}
			if err := os.WriteFile(filepath.Join(recordPath, "aidlc-state.md"), []byte("# state\n"), 0o644); err != nil {
				t.Fatalf("create state file: %v", err)
			}

			projectRoot, err := os.OpenRoot(projectPath)
			if err != nil {
				t.Fatalf("open project root: %v", err)
			}
			defer projectRoot.Close()
			recordRoot, err := os.OpenRoot(recordPath)
			if err != nil {
				t.Fatalf("open record root: %v", err)
			}
			defer recordRoot.Close()

			keyPath := filepath.Join(recordPath, ".aidlc-steering-token-key")
			wantFile := append([]byte(nil), test.file...)
			if err := os.WriteFile(keyPath, wantFile, 0o640); err != nil {
				t.Fatalf("write corrupt key file: %v", err)
			}

			got, err := ReadOrCreateContinuationKey(projectRoot, recordRoot)
			if err == nil {
				t.Fatal("ReadOrCreateContinuationKey() error = nil, want invalid key error")
			}
			if !errors.Is(err, ErrInvalidContinuationKeyFile) {
				t.Errorf("ReadOrCreateContinuationKey() error = %v, want ErrInvalidContinuationKeyFile", err)
			}
			if len(got) != 0 {
				t.Errorf("ReadOrCreateContinuationKey() key length = %d, want zero", len(got))
			}

			fileBytes, err := os.ReadFile(keyPath)
			if err != nil {
				t.Fatalf("read corrupt key file: %v", err)
			}
			if !bytes.Equal(fileBytes, wantFile) {
				t.Errorf("corrupt key file changed = %q, want %q", fileBytes, wantFile)
			}
			projectKeyPath := filepath.Join(projectPath, "aidlc", ".aidlc-sessions", ".aidlc-steering-token-key")
			if _, err := os.Stat(projectKeyPath); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("project session key stat error = %v, want os.ErrNotExist", err)
			}
			if _, err := projectRoot.Stat("."); err != nil {
				t.Errorf("project root unavailable after call: %v", err)
			}
			if _, err := recordRoot.Stat("aidlc-state.md"); err != nil {
				t.Errorf("record root unavailable after call: %v", err)
			}
		})
	}
}

func TestReadOrCreateContinuationKeyRereadsConcurrentWinner(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "project")
	recordPath := filepath.Join(root, "record")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("create project directory: %v", err)
	}
	if err := os.MkdirAll(recordPath, 0o755); err != nil {
		t.Fatalf("create record directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recordPath, "aidlc-state.md"), []byte("# state\n"), 0o644); err != nil {
		t.Fatalf("create state file: %v", err)
	}

	projectRoot, err := os.OpenRoot(projectPath)
	if err != nil {
		t.Fatalf("open project root: %v", err)
	}
	defer projectRoot.Close()
	recordRoot, err := os.OpenRoot(recordPath)
	if err != nil {
		t.Fatalf("open record root: %v", err)
	}
	defer recordRoot.Close()

	loserKey := bytes.Repeat([]byte{0xa5}, 32)
	winnerKey := bytes.Repeat([]byte{0x5a}, 32)
	winnerFile := []byte(base64.RawURLEncoding.EncodeToString(winnerKey) + "\n")
	started := make(chan struct{})
	release := make(chan struct{})
	randomReader := &blockingContinuationRandomReader{
		data:    loserKey,
		started: started,
		release: release,
	}
	originalReader := cryptorand.Reader
	cryptorand.Reader = randomReader
	defer func() {
		cryptorand.Reader = originalReader
	}()

	type readResult struct {
		key []byte
		err error
	}
	resultCh := make(chan readResult, 1)
	go func() {
		key, err := ReadOrCreateContinuationKey(projectRoot, recordRoot)
		resultCh <- readResult{key: key, err: err}
	}()

	waitForResult := func() readResult {
		t.Helper()
		select {
		case result := <-resultCh:
			return result
		case <-time.After(5 * time.Second):
			t.Fatal("ReadOrCreateContinuationKey() did not finish")
			return readResult{}
		}
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		_ = waitForResult()
		t.Fatal("ReadOrCreateContinuationKey() did not reach random read")
	}

	keyPath := filepath.Join(recordPath, ".aidlc-steering-token-key")
	winnerFileHandle, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		close(release)
		_ = waitForResult()
		t.Fatalf("create concurrent winner key file: %v", err)
	}
	if written, err := winnerFileHandle.Write(winnerFile); err != nil || written != len(winnerFile) {
		_ = winnerFileHandle.Close()
		close(release)
		_ = waitForResult()
		if err != nil {
			t.Fatalf("write concurrent winner key file: %v", err)
		}
		t.Fatalf("write concurrent winner key file = %d bytes, want %d", written, len(winnerFile))
	}
	if err := winnerFileHandle.Close(); err != nil {
		close(release)
		_ = waitForResult()
		t.Fatalf("close concurrent winner key file: %v", err)
	}

	close(release)
	result := waitForResult()
	if result.err != nil {
		t.Fatalf("ReadOrCreateContinuationKey() concurrent winner error = %v", result.err)
	}
	if !bytes.Equal(result.key, winnerKey) {
		t.Errorf("concurrent winner key = %x, want %x", result.key, winnerKey)
	}
	if bytes.Equal(result.key, loserKey) {
		t.Errorf("concurrent winner returned controlled loser key %x", result.key)
	}
	fileBytes, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read concurrent winner key file: %v", err)
	}
	if !bytes.Equal(fileBytes, winnerFile) {
		t.Errorf("concurrent winner key file = %q, want %q", fileBytes, winnerFile)
	}
	if _, err := projectRoot.Stat("."); err != nil {
		t.Errorf("project root unavailable after call: %v", err)
	}
	if _, err := recordRoot.Stat("aidlc-state.md"); err != nil {
		t.Errorf("record root unavailable after call: %v", err)
	}
}

type blockingContinuationRandomReader struct {
	data    []byte
	started chan<- struct{}
	release <-chan struct{}
	once    sync.Once
	offset  int
}

func (r *blockingContinuationRandomReader) Read(p []byte) (int, error) {
	r.once.Do(func() {
		close(r.started)
	})
	<-r.release
	if r.offset == len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}

func TestReadOrCreateContinuationKeyPropagatesIOFailure(t *testing.T) {
	validKeyFile := []byte(base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x37}, 32)) + "\n")
	randomErr := errors.New("random reader failure")
	existingOpenErr := errors.New("existing key open failure")
	existingReadErr := errors.New("existing key read failure")
	existingStatErr := errors.New("existing key stat failure")
	existingCloseErr := errors.New("existing key close failure")
	createOpenErr := errors.New("create key open failure")
	createWriteErr := errors.New("create key write failure")
	createCloseErr := errors.New("create key close failure")
	winnerOpenErr := errors.New("winner key open failure")
	winnerReadErr := errors.New("winner key read failure")
	winnerCloseErr := errors.New("winner key close failure")
	lstatErr := errors.New("state lstat failure")
	mkdirErr := errors.New("session mkdir failure")
	tests := []struct {
		name      string
		useRecord bool
		cause     error
		setup     func(*continuationFailureHarness)
	}{
		{
			name:      "random reader error",
			useRecord: true,
			cause:     randomErr,
			setup: func(h *continuationFailureHarness) {
				h.openErrs[1] = fs.ErrNotExist
				h.random = continuationErrorReader{err: randomErr}
			},
		},
		{
			name:      "existing open error",
			useRecord: true,
			cause:     existingOpenErr,
			setup: func(h *continuationFailureHarness) {
				h.openErrs[1] = existingOpenErr
			},
		},
		{
			name:      "existing read error",
			useRecord: true,
			cause:     existingReadErr,
			setup: func(h *continuationFailureHarness) {
				file := &continuationFailureFile{statInfo: h.stateInfo, readErr: existingReadErr}
				h.openFiles[1] = file
				h.files = append(h.files, file)
			},
		},
		{
			name:      "existing stat error",
			useRecord: true,
			cause:     existingStatErr,
			setup: func(h *continuationFailureHarness) {
				file := &continuationFailureFile{statErr: existingStatErr}
				h.openFiles[1] = file
				h.files = append(h.files, file)
			},
		},
		{
			name:      "existing close error",
			useRecord: true,
			cause:     existingCloseErr,
			setup: func(h *continuationFailureHarness) {
				file := &continuationFailureFile{
					data:     append([]byte(nil), validKeyFile...),
					statInfo: h.stateInfo,
					closeErr: existingCloseErr,
				}
				h.openFiles[1] = file
				h.files = append(h.files, file)
			},
		},
		{
			name:      "create open error",
			useRecord: true,
			cause:     createOpenErr,
			setup: func(h *continuationFailureHarness) {
				h.openErrs[1] = fs.ErrNotExist
				h.openErrs[2] = createOpenErr
			},
		},
		{
			name:      "create short write",
			useRecord: true,
			cause:     io.ErrShortWrite,
			setup: func(h *continuationFailureHarness) {
				h.openErrs[1] = fs.ErrNotExist
				file := &continuationFailureFile{shortWrite: true}
				h.openFiles[2] = file
				h.files = append(h.files, file)
			},
		},
		{
			name:      "create write error",
			useRecord: true,
			cause:     createWriteErr,
			setup: func(h *continuationFailureHarness) {
				h.openErrs[1] = fs.ErrNotExist
				file := &continuationFailureFile{writeErr: createWriteErr}
				h.openFiles[2] = file
				h.files = append(h.files, file)
			},
		},
		{
			name:      "create close error",
			useRecord: true,
			cause:     createCloseErr,
			setup: func(h *continuationFailureHarness) {
				h.openErrs[1] = fs.ErrNotExist
				file := &continuationFailureFile{closeErr: createCloseErr}
				h.openFiles[2] = file
				h.files = append(h.files, file)
			},
		},
		{
			name:      "winner open error",
			useRecord: true,
			cause:     winnerOpenErr,
			setup: func(h *continuationFailureHarness) {
				h.openErrs[1] = fs.ErrNotExist
				h.openErrs[2] = fs.ErrExist
				h.openErrs[3] = winnerOpenErr
			},
		},
		{
			name:      "winner read error",
			useRecord: true,
			cause:     winnerReadErr,
			setup: func(h *continuationFailureHarness) {
				h.openErrs[1] = fs.ErrNotExist
				h.openErrs[2] = fs.ErrExist
				file := &continuationFailureFile{statInfo: h.stateInfo, readErr: winnerReadErr}
				h.openFiles[3] = file
				h.files = append(h.files, file)
			},
		},
		{
			name:      "winner close error",
			useRecord: true,
			cause:     winnerCloseErr,
			setup: func(h *continuationFailureHarness) {
				h.openErrs[1] = fs.ErrNotExist
				h.openErrs[2] = fs.ErrExist
				file := &continuationFailureFile{
					data:     append([]byte(nil), validKeyFile...),
					statInfo: h.stateInfo,
					closeErr: winnerCloseErr,
				}
				h.openFiles[3] = file
				h.files = append(h.files, file)
			},
		},
		{
			name:      "state lstat error",
			useRecord: true,
			cause:     lstatErr,
			setup: func(h *continuationFailureHarness) {
				h.lstatErr = lstatErr
			},
		},
		{
			name:      "session mkdir error",
			useRecord: false,
			cause:     mkdirErr,
			setup: func(h *continuationFailureHarness) {
				h.mkdirErr = mkdirErr
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newContinuationFailureHarness()
			test.setup(h)
			projectRoot := new(os.Root)
			var recordRoot *os.Root
			if test.useRecord {
				recordRoot = new(os.Root)
			}

			got, err := readOrCreateContinuationKey(projectRoot, recordRoot, h.ops())
			if err == nil {
				t.Fatal("readOrCreateContinuationKey() error = nil, want injected failure")
			}
			if !errors.Is(err, test.cause) {
				t.Errorf("readOrCreateContinuationKey() error = %v, want cause %v", err, test.cause)
			}
			if len(got) != 0 {
				t.Errorf("readOrCreateContinuationKey() key length = %d, want zero", len(got))
			}
			for index, file := range h.files {
				if file.closeCalls == 0 {
					t.Errorf("fake file %d Close calls = 0, want at least one", index)
				}
			}
		})
	}
}

type continuationFailureHarness struct {
	stateInfo fs.FileInfo
	random    io.Reader
	lstatErr  error
	mkdirErr  error
	openCalls int
	openErrs  map[int]error
	openFiles map[int]*continuationFailureFile
	files     []*continuationFailureFile
}

func newContinuationFailureHarness() *continuationFailureHarness {
	return &continuationFailureHarness{
		stateInfo: continuationFailureFileInfo{mode: 0o600},
		random:    bytes.NewReader(bytes.Repeat([]byte{0x37}, 32)),
		openErrs:  make(map[int]error),
		openFiles: make(map[int]*continuationFailureFile),
	}
}

func (h *continuationFailureHarness) ops() continuationKeyOps {
	return continuationKeyOps{
		random: h.random,
		lstat: func(*os.Root, string) (fs.FileInfo, error) {
			if h.lstatErr != nil {
				return nil, h.lstatErr
			}
			return h.stateInfo, nil
		},
		mkdirAll: func(*os.Root, string, fs.FileMode) error {
			return h.mkdirErr
		},
		openFile: func(*os.Root, string, int, fs.FileMode) (continuationKeyFile, error) {
			h.openCalls++
			if err := h.openErrs[h.openCalls]; err != nil {
				return nil, err
			}
			if file := h.openFiles[h.openCalls]; file != nil {
				return file, nil
			}
			return nil, fs.ErrNotExist
		},
	}
}

type continuationErrorReader struct {
	err error
}

func (r continuationErrorReader) Read([]byte) (int, error) {
	return 0, r.err
}

type continuationFailureFileInfo struct {
	mode fs.FileMode
}

func (i continuationFailureFileInfo) Name() string       { return "continuation-key" }
func (i continuationFailureFileInfo) Size() int64        { return 0 }
func (i continuationFailureFileInfo) Mode() fs.FileMode  { return i.mode }
func (i continuationFailureFileInfo) ModTime() time.Time { return time.Time{} }
func (i continuationFailureFileInfo) IsDir() bool        { return i.mode.IsDir() }
func (i continuationFailureFileInfo) Sys() any           { return nil }

type continuationFailureFile struct {
	data       []byte
	readErr    error
	writeErr   error
	shortWrite bool
	statInfo   fs.FileInfo
	statErr    error
	closeErr   error
	closeCalls int
}

func (f *continuationFailureFile) Read(p []byte) (int, error) {
	if f.readErr != nil {
		return 0, f.readErr
	}
	if len(f.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, f.data)
	f.data = f.data[n:]
	return n, nil
}

func (f *continuationFailureFile) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	if f.shortWrite {
		return 0, nil
	}
	return len(p), nil
}

func (f *continuationFailureFile) Stat() (fs.FileInfo, error) {
	return f.statInfo, f.statErr
}

func (f *continuationFailureFile) Close() error {
	f.closeCalls++
	return f.closeErr
}
