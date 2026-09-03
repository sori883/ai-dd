// Package recordlock provides a small, cross-process lock for one AI-DLC
// record.  A lock is intentionally identified by the project path, space,
// and intent together; callers pass the held Guard to operations which must
// stay in the same critical section.
package recordlock

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	lockRootName       = "aidlc-record-lock"
	lockOwnerName      = "owner.json"
	lockMaxRetries     = 100
	lockRetryInterval  = 10 * time.Millisecond
	lockTokenByteCount = 16
)

var (
	// ErrInvalidIdentity means that one or more identity components cannot be
	// represented unambiguously by the lock or audit path contracts.
	ErrInvalidIdentity = errors.New("recordlock: invalid identity")
	// ErrNotHeld means that a Guard is nil, released, or was never acquired.
	ErrNotHeld = errors.New("recordlock: guard is not held")
	// ErrOwnerMismatch means that the on-disk lock is no longer owned by this
	// Guard.  The lock path is intentionally left untouched in this case.
	ErrOwnerMismatch = errors.New("recordlock: lock owner mismatch")
	// ErrInvalidCallback means that With was given no callback.
	ErrInvalidCallback = errors.New("recordlock: nil callback")
)

// Identity is the canonical identity of one record.  Use NewIdentity to
// construct it; its fields are private so an unvalidated value cannot be
// assembled by another internal package.
type Identity struct {
	projectPath string
	space       string
	intent      string
}

// NewIdentity validates and canonicalizes the project path and its two
// record-name components.  Existing symlinks in projectPath are resolved when
// possible; a missing path retains its absolute lexical form so callers can
// acquire a lock before creating a record.
func NewIdentity(projectPath, space, intent string) (Identity, error) {
	if projectPath == "" || hasUnsafePathText(projectPath) {
		return Identity{}, fmt.Errorf("recordlock: project path: %w", ErrInvalidIdentity)
	}
	if err := validateComponent(space, "space"); err != nil {
		return Identity{}, err
	}
	if err := validateComponent(intent, "intent"); err != nil {
		return Identity{}, err
	}
	absolute, err := filepath.Abs(projectPath)
	if err != nil {
		return Identity{}, fmt.Errorf("recordlock: resolve project path %q: %w: %w", projectPath, ErrInvalidIdentity, err)
	}
	canonical := filepath.Clean(absolute)
	if resolved, resolveErr := filepath.EvalSymlinks(canonical); resolveErr == nil {
		if resolvedAbsolute, absoluteErr := filepath.Abs(resolved); absoluteErr == nil {
			canonical = filepath.Clean(resolvedAbsolute)
		}
	}
	if runtime.GOOS == "windows" {
		canonical = strings.ToLower(canonical)
	}
	return Identity{projectPath: canonical, space: space, intent: intent}, nil
}

// ProjectRoot returns the canonical project path used by this identity.
func (id Identity) ProjectRoot() string { return id.projectPath }

// ProjectPath is an alias for ProjectRoot for callers that name the input as
// a path rather than a root.
func (id Identity) ProjectPath() string { return id.projectPath }

// Path is a short alias for ProjectRoot.
func (id Identity) Path() string { return id.projectPath }

// Space returns the validated space component.
func (id Identity) Space() string { return id.space }

// SpaceName is a descriptive alias for Space.
func (id Identity) SpaceName() string { return id.space }

// Intent returns the validated intent component.
func (id Identity) Intent() string { return id.intent }

// IntentName is a descriptive alias for Intent.
func (id Identity) IntentName() string { return id.intent }

func (id Identity) String() string {
	if id.projectPath == "" {
		return ""
	}
	return id.projectPath + "\x00" + id.space + "\x00" + id.intent
}

func (id Identity) valid() bool {
	return id.projectPath != "" && !hasUnsafePathText(id.projectPath) &&
		filepath.IsAbs(id.projectPath) && componentIsValid(id.space) && componentIsValid(id.intent)
}

func (id Identity) lockPath() string {
	digest := sha256.Sum256([]byte(id.String()))
	return filepath.Join(os.TempDir(), lockRootName, hex.EncodeToString(digest[:]))
}

// LockPath returns the deterministic system-temp path for a valid identity.
// It is useful for diagnostics and tests; callers must use Acquire rather
// than manipulating this path directly.
func (id Identity) LockPath() string {
	if !id.valid() {
		return ""
	}
	return id.lockPath()
}

func lockPathForTemp(id Identity, tempDir string) string {
	digest := sha256.Sum256([]byte(id.String()))
	return filepath.Join(tempDir, lockRootName, hex.EncodeToString(digest[:]))
}

func validateComponent(value, label string) error {
	if !componentIsValid(value) {
		return fmt.Errorf("recordlock: %s %q: %w", label, value, ErrInvalidIdentity)
	}
	return nil
}

func componentIsValid(value string) bool {
	if value == "" || value == "." || value == ".." || !fs.ValidPath(value) {
		return false
	}
	for _, char := range value {
		if char == '/' || char == '\\' || isUnsafeControl(char) {
			return false
		}
	}
	return true
}

func hasUnsafePathText(value string) bool {
	for _, char := range value {
		if isUnsafeControl(char) {
			return true
		}
	}
	return false
}

func isUnsafeControl(char rune) bool {
	return char <= 0x1f || char == 0x7f || char == '\u2028' || char == '\u2029' || unicode.Is(unicode.Cc, char)
}

type lockOwner struct {
	Token       string `json:"token"`
	PID         int    `json:"pid"`
	StartedAt   string `json:"startedAt"`
	StartedAtMS int64  `json:"startedAtMs"`
}

type lockSettings struct {
	maxRetries    int
	retryInterval time.Duration
}

type lockOps struct {
	tempDir  func() string
	mkdirAll func(string, fs.FileMode) error
	mkdir    func(string, fs.FileMode) error
	openFile func(string, int, fs.FileMode) (*os.File, error)
	write    func(*os.File, []byte) (int, error)
	close    func(*os.File) error
	readFile func(string) ([]byte, error)
	remove   func(string) error
	now      func() time.Time
	pid      func() int
	random   io.Reader
	wait     func(context.Context, time.Duration) error
}

func systemLockOps() lockOps {
	return lockOps{
		tempDir:  os.TempDir,
		mkdirAll: os.MkdirAll,
		mkdir:    os.Mkdir,
		openFile: os.OpenFile,
		write:    (*os.File).Write,
		close:    (*os.File).Close,
		readFile: os.ReadFile,
		remove:   os.Remove,
		now:      time.Now,
		pid:      os.Getpid,
		random:   rand.Reader,
		wait:     waitForLock,
	}
}

func waitForLock(ctx context.Context, duration time.Duration) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-timer.C:
		return context.Cause(ctx)
	}
}

// Guard proves that the caller currently owns the identity-bound record lock.
// Pass the same pointer to nested operations; they must not acquire the same
// identity again.
type Guard struct {
	state *guardState
}

type guardState struct {
	mu       sync.Mutex
	identity Identity
	path     string
	token    string
	held     bool
	ops      lockOps
}

// Identity returns the record identity associated with the Guard.  The zero
// value is returned for a nil or invalid Guard.
func (g *Guard) Identity() Identity {
	if g == nil || g.state == nil {
		return Identity{}
	}
	return g.state.identity
}

// Held reports whether the Guard still owns its lock generation.
func (g *Guard) Held() bool {
	if g == nil || g.state == nil {
		return false
	}
	g.state.mu.Lock()
	defer g.state.mu.Unlock()
	return g.state.held
}

// IsHeld is a descriptive alias for Held.
func (g *Guard) IsHeld() bool { return g.Held() }

// Acquire obtains an identity lock or returns an error without removing an
// existing owner's lock.  Waiting is bounded and observes ctx cancellation.
func Acquire(ctx context.Context, identity Identity) (*Guard, error) {
	return acquireWithOps(ctx, identity, lockSettings{
		maxRetries:    lockMaxRetries,
		retryInterval: lockRetryInterval,
	}, systemLockOps())
}

func acquireWithOps(ctx context.Context, identity Identity, settings lockSettings, ops lockOps) (*Guard, error) {
	if ctx == nil {
		return nil, fmt.Errorf("recordlock: acquire: nil context: %w", fs.ErrInvalid)
	}
	if !identity.valid() {
		return nil, fmt.Errorf("recordlock: acquire: %w", ErrInvalidIdentity)
	}
	if settings.maxRetries < 0 {
		return nil, fmt.Errorf("recordlock: acquire retries: %w", fs.ErrInvalid)
	}
	if settings.retryInterval < 0 {
		return nil, fmt.Errorf("recordlock: acquire retry interval: %w", fs.ErrInvalid)
	}
	if err := context.Cause(ctx); err != nil {
		return nil, fmt.Errorf("recordlock: acquire: %w", err)
	}
	tempDir := ops.tempDir()
	if tempDir == "" {
		return nil, fmt.Errorf("recordlock: acquire temp directory: %w", fs.ErrInvalid)
	}
	base := filepath.Join(tempDir, lockRootName)
	if err := ops.mkdirAll(base, 0o700); err != nil {
		return nil, fmt.Errorf("recordlock: create lock root %q: %w", base, err)
	}
	path := lockPathForTemp(identity, tempDir)
	for attempt := 0; attempt <= settings.maxRetries; attempt++ {
		if err := context.Cause(ctx); err != nil {
			return nil, fmt.Errorf("recordlock: acquire %q: %w", path, err)
		}
		err := ops.mkdir(path, 0o700)
		if err == nil {
			guard, initializeErr := initializeGuard(identity, path, ops)
			if initializeErr != nil {
				return nil, initializeErr
			}
			return guard, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("recordlock: create lock %q: %w", path, err)
		}
		if attempt == settings.maxRetries {
			if contextErr := context.Cause(ctx); contextErr != nil {
				return nil, fmt.Errorf("recordlock: acquire %q: %w", path, contextErr)
			}
			return nil, fmt.Errorf("recordlock: acquire %q after %d retries: %w", path, settings.maxRetries, fs.ErrExist)
		}
		if err := ops.wait(ctx, settings.retryInterval); err != nil {
			return nil, fmt.Errorf("recordlock: wait for %q: %w", path, err)
		}
	}
	panic("recordlock: unreachable retry loop")
}

func initializeGuard(identity Identity, path string, ops lockOps) (*Guard, error) {
	token, err := randomToken(ops.random)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("recordlock: generate owner token: %w", err),
			removeOwnedLock(path, "", ops),
		)
	}
	startedAt := ops.now().UTC()
	owner := lockOwner{
		Token:       token,
		PID:         ops.pid(),
		StartedAt:   startedAt.Format(time.RFC3339Nano),
		StartedAtMS: startedAt.UnixMilli(),
	}
	if err := writeOwner(filepath.Join(path, lockOwnerName), owner, ops); err != nil {
		return nil, errors.Join(err, removeOwnedLock(path, token, ops))
	}
	return &Guard{state: &guardState{
		identity: identity,
		path:     path,
		token:    token,
		held:     true,
		ops:      ops,
	}}, nil
}

func randomToken(random io.Reader) (string, error) {
	if random == nil {
		return "", fmt.Errorf("recordlock: random source is nil: %w", fs.ErrInvalid)
	}
	var value [lockTokenByteCount]byte
	if _, err := io.ReadFull(random, value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func writeOwner(path string, owner lockOwner, ops lockOps) (err error) {
	data, err := json.Marshal(owner)
	if err != nil {
		return fmt.Errorf("recordlock: encode owner: %w", err)
	}
	file, err := ops.openFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("recordlock: open owner %q: %w", path, err)
	}
	if file == nil {
		return fmt.Errorf("recordlock: open owner %q returned nil file: %w", path, fs.ErrInvalid)
	}
	writeErr := writeLockBytes(file, data, ops.write)
	if writeErr != nil {
		err = fmt.Errorf("recordlock: write owner %q: %w", path, writeErr)
	}
	if closeErr := ops.close(file); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("recordlock: close owner %q: %w", path, closeErr))
	}
	return err
}

func writeLockBytes(file *os.File, data []byte, write func(*os.File, []byte) (int, error)) error {
	for len(data) > 0 {
		n, err := write(file, data)
		if n < 0 || n > len(data) {
			return fmt.Errorf("recordlock: invalid owner write count %d: %w", n, io.ErrShortWrite)
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func removeOwnedLock(path, token string, ops lockOps) (err error) {
	if token != "" {
		if removeErr := ops.remove(filepath.Join(path, lockOwnerName)); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("recordlock: remove owner %q: %w", path, removeErr))
		}
	}
	if removeErr := ops.remove(path); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
		err = errors.Join(err, fmt.Errorf("recordlock: remove lock %q: %w", path, removeErr))
	}
	return err
}

// Release verifies the persisted owner token before removing this Guard's
// generation.  A mismatch never removes the lock path.
func (g *Guard) Release() error {
	if g == nil || g.state == nil {
		return ErrNotHeld
	}
	g.state.mu.Lock()
	defer g.state.mu.Unlock()
	if !g.state.held {
		return ErrNotHeld
	}
	ownerPath := filepath.Join(g.state.path, lockOwnerName)
	data, err := g.state.ops.readFile(ownerPath)
	if err != nil {
		return fmt.Errorf("recordlock: read owner %q: %w", ownerPath, err)
	}
	var owner lockOwner
	if err := json.Unmarshal(data, &owner); err != nil || owner.Token == "" {
		return fmt.Errorf("recordlock: decode owner %q: %w", ownerPath, ErrOwnerMismatch)
	}
	if owner.Token != g.state.token {
		return fmt.Errorf("recordlock: owner changed at %q: %w", g.state.path, ErrOwnerMismatch)
	}
	if err := g.state.ops.remove(ownerPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("recordlock: remove owner %q: %w", ownerPath, ErrOwnerMismatch)
		}
		return fmt.Errorf("recordlock: remove owner %q: %w", ownerPath, err)
	}
	removeErr := g.state.ops.remove(g.state.path)
	// Once the owner marker is gone, this generation cannot be safely retried;
	// callers receive the removal failure while the Guard becomes unheld.
	g.state.held = false
	if removeErr != nil {
		return fmt.Errorf("recordlock: remove lock %q: %w", g.state.path, removeErr)
	}
	return nil
}

// With acquires one record lock, invokes fn with its identity-bound Guard, and
// releases the lock on every normal or panic path.  Callback and release
// errors are joined; a panic is re-raised after release is attempted.
func With(ctx context.Context, identity Identity, fn func(*Guard) error) (err error) {
	return withLockOps(ctx, identity, fn, lockSettings{
		maxRetries:    lockMaxRetries,
		retryInterval: lockRetryInterval,
	}, systemLockOps())
}

func withLockOps(ctx context.Context, identity Identity, fn func(*Guard) error, settings lockSettings, ops lockOps) (err error) {
	if fn == nil {
		return ErrInvalidCallback
	}
	guard, err := acquireWithOps(ctx, identity, settings, ops)
	if err != nil {
		return err
	}
	defer func() {
		releaseErr := guard.Release()
		if recovered := recover(); recovered != nil {
			panic(recovered)
		}
		err = errors.Join(err, releaseErr)
	}()
	err = fn(guard)
	return err
}
