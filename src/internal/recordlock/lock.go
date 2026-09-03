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
	"sync"
	"time"
	"unicode"

	"github.com/sori883/ai-dd/src/internal/pathnorm"
)

const (
	lockRootName       = "aidlc-record-lock"
	lockOwnerPrefix    = "owner-"
	lockOwnerSuffix    = ".json"
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
	canonical = pathnorm.NormalizeForPlatform(canonical, runtime.GOOS)
	return Identity{projectPath: canonical, space: space, intent: intent}, nil
}

func normalizeIdentityPathForPlatform(canonical, platform string) string {
	return pathnorm.NormalizeForPlatform(canonical, platform)
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

// lockRootLocation returns the trusted temporary-directory Root and the
// descriptor-relative path for one generated lock pathname. Keeping this
// derivation narrow prevents a caller or a test seam from turning the Root
// open into an arbitrary pathname traversal.
func lockRootLocation(path string) (base, relative string, err error) {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) || clean != path {
		return "", "", fmt.Errorf("recordlock: lock path %q is not canonical: %w", path, ErrInvalidIdentity)
	}
	hashName := filepath.Base(clean)
	lockDir := filepath.Dir(clean)
	if filepath.Base(lockDir) != lockRootName || len(hashName) != sha256.Size*2 {
		return "", "", fmt.Errorf("recordlock: lock path %q has unexpected layout: %w", path, ErrInvalidIdentity)
	}
	if _, decodeErr := hex.DecodeString(hashName); decodeErr != nil {
		return "", "", fmt.Errorf("recordlock: lock path %q has invalid generation: %w", path, ErrInvalidIdentity)
	}
	base = filepath.Dir(lockDir)
	if base == "" || base == "." {
		return "", "", fmt.Errorf("recordlock: lock path %q has no trusted base: %w", path, ErrInvalidIdentity)
	}
	relative, err = filepath.Rel(base, clean)
	if err != nil || relative != filepath.Join(lockRootName, hashName) || filepath.IsAbs(relative) {
		return "", "", fmt.Errorf("recordlock: lock path %q has unexpected relative layout: %w", path, ErrInvalidIdentity)
	}
	return base, relative, nil
}

func ownerMarkerName(token string) string {
	return lockOwnerPrefix + token + lockOwnerSuffix
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
	tempDir          func() string
	mkdirAll         func(string, fs.FileMode) error
	mkdir            func(string, fs.FileMode) error
	openFile         func(string, int, fs.FileMode) (*os.File, error)
	openRoot         func(string) (*os.Root, error)
	rootLstat        func(*os.Root, string) (fs.FileInfo, error)
	rootCreate       func(*os.Root, string) (*os.File, error)
	rootOpen         func(*os.Root, string) (*os.File, error)
	rootRemove       func(*os.Root, string) error
	rootClose        func(*os.Root) error
	write            func(*os.File, []byte) (int, error)
	close            func(*os.File) error
	lstat            func(string) (fs.FileInfo, error)
	remove           func(string) error
	beforeOwnerProof func(string) error
	now              func() time.Time
	pid              func() int
	random           io.Reader
	wait             func(context.Context, time.Duration) error
}

func systemLockOps() lockOps {
	return lockOps{
		tempDir:    os.TempDir,
		mkdirAll:   os.MkdirAll,
		mkdir:      os.Mkdir,
		openFile:   os.OpenFile,
		openRoot:   openLockRoot,
		rootLstat:  (*os.Root).Lstat,
		rootCreate: openLockOwnerCreate,
		rootOpen:   openLockOwner,
		rootRemove: (*os.Root).Remove,
		rootClose:  (*os.Root).Close,
		write:      (*os.File).Write,
		close:      (*os.File).Close,
		lstat:      os.Lstat,
		remove:     os.Remove,
		now:        time.Now,
		pid:        os.Getpid,
		random:     rand.Reader,
		wait:       waitForLock,
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
	// lockRoot pins the directory generation acquired by this Guard. Release
	// must use this descriptor-relative root instead of reopening state.path,
	// which could have been replaced by a blocking or unrelated filesystem
	// object after its pathname inspection.
	lockRoot    *os.Root
	token       string
	held        bool
	releasing   bool
	leaseTokens chan struct{}
	ops         lockOps
}

// acquireLease enters the Guard's process-local critical section. Copies of
// the same Guard share this reservation, so concurrent nested operations are
// serialized and a release waits for the active operation to finish.
//
// The release closure is deliberately kept private. Exporting a lease value
// would let callers copy its one-shot state and overfill the shared token
// channel during double release.
func (g *Guard) acquireLease(ctx context.Context) (func() error, error) {
	if ctx == nil {
		return nil, fmt.Errorf("recordlock: lease: nil context: %w", fs.ErrInvalid)
	}
	if g == nil || g.state == nil {
		return nil, ErrNotHeld
	}
	if err := context.Cause(ctx); err != nil {
		return nil, fmt.Errorf("recordlock: lease: %w", err)
	}
	state := g.state
	state.mu.Lock()
	if !state.held || state.releasing {
		state.mu.Unlock()
		return nil, ErrNotHeld
	}
	state.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("recordlock: lease: %w", context.Cause(ctx))
	case <-state.leaseTokens:
	}
	if err := context.Cause(ctx); err != nil {
		state.leaseTokens <- struct{}{}
		return nil, fmt.Errorf("recordlock: lease: %w", err)
	}
	state.mu.Lock()
	valid := state.held && !state.releasing
	state.mu.Unlock()
	if !valid {
		state.leaseTokens <- struct{}{}
		return nil, ErrNotHeld
	}
	var once sync.Once
	return func() error {
		once.Do(func() {
			state.leaseTokens <- struct{}{}
		})
		return nil
	}, nil
}

// WithLease runs fn inside the Guard's process-local critical section and
// releases the lease even when fn returns an error.
func (g *Guard) WithLease(ctx context.Context, fn func() error) (err error) {
	if fn == nil {
		return ErrInvalidCallback
	}
	release, err := g.acquireLease(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, release())
	}()
	return fn()
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

func initializeGuard(identity Identity, path string, ops lockOps) (guard *Guard, err error) {
	token, err := randomToken(ops.random)
	if err != nil {
		return nil, fmt.Errorf("recordlock: generate owner token: %w", err)
	}
	openRoot := ops.openRoot
	if openRoot == nil {
		openRoot = openLockRoot
	}
	lockRoot, err := openRoot(path)
	if err != nil {
		return nil, fmt.Errorf("recordlock: open lock root %q: %w", path, err)
	}
	if lockRoot == nil {
		return nil, fmt.Errorf("recordlock: open lock root %q returned nil root: %w", path, ErrOwnerMismatch)
	}
	rootClose := ops.rootClose
	if rootClose == nil {
		rootClose = (*os.Root).Close
	}
	keepRoot := false
	defer func() {
		if keepRoot {
			return
		}
		if closeErr := rootClose(lockRoot); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("recordlock: close lock root %q: %w", path, closeErr))
		}
	}()
	startedAt := ops.now().UTC()
	owner := lockOwner{
		Token:       token,
		PID:         ops.pid(),
		StartedAt:   startedAt.Format(time.RFC3339Nano),
		StartedAtMS: startedAt.UnixMilli(),
	}
	ownerName := ownerMarkerName(token)
	ownerInfo, err := writeOwnerAtRoot(lockRoot, ownerName, owner, ops)
	if err != nil {
		return nil, errors.Join(err, cleanupInitializedLock(lockRoot, path, ownerName, ownerInfo, ops))
	}
	if _, err := verifyPinnedLockPath(path, lockRoot, ops); err != nil {
		return nil, errors.Join(err, cleanupInitializedLock(lockRoot, path, ownerName, ownerInfo, ops))
	}
	guard = &Guard{state: &guardState{
		identity: identity,
		path:     path,
		lockRoot: lockRoot,
		token:    token,
		held:     true,
		leaseTokens: func() chan struct{} {
			available := make(chan struct{}, 1)
			available <- struct{}{}
			return available
		}(),
		ops: ops,
	}}
	keepRoot = true
	return guard, nil
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

func writeOwnerAtRoot(root *os.Root, name string, owner lockOwner, ops lockOps) (ownerInfo fs.FileInfo, err error) {
	data, err := json.Marshal(owner)
	if err != nil {
		return nil, fmt.Errorf("recordlock: encode owner: %w", err)
	}
	if root == nil {
		return nil, fmt.Errorf("recordlock: open owner %q: %w", name, ErrOwnerMismatch)
	}
	rootCreate := ops.rootCreate
	if rootCreate == nil {
		rootCreate = openLockOwnerCreate
	}
	file, err := rootCreate(root, name)
	if err != nil {
		return nil, fmt.Errorf("recordlock: open owner %q: %w", name, err)
	}
	if file == nil {
		return nil, fmt.Errorf("recordlock: open owner %q returned nil file: %w", name, fs.ErrInvalid)
	}
	ownerInfo, err = file.Stat()
	if err != nil {
		closeErr := closeLockFile(file, ops)
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, fmt.Errorf("recordlock: stat owner %q: %w", name, err)
	}
	writeErr := writeLockBytes(file, data, ops.write)
	if writeErr != nil {
		err = fmt.Errorf("recordlock: write owner %q: %w", name, writeErr)
	}
	if closeErr := closeLockFile(file, ops); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("recordlock: close owner %q: %w", name, closeErr))
	}
	return ownerInfo, err
}

func cleanupInitializedLock(root *os.Root, path, ownerName string, ownerInfo fs.FileInfo, ops lockOps) (err error) {
	if root == nil {
		return fmt.Errorf("recordlock: cleanup lock %q has no pinned root: %w", path, ErrOwnerMismatch)
	}
	rootRemove := ops.rootRemove
	if rootRemove == nil {
		rootRemove = (*os.Root).Remove
	}
	if ownerInfo != nil {
		ownerLstat := ops.rootLstat
		if ownerLstat == nil {
			ownerLstat = (*os.Root).Lstat
		}
		current, lstatErr := ownerLstat(root, ownerName)
		if lstatErr != nil {
			if !errors.Is(lstatErr, fs.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("recordlock: inspect owner %q for cleanup: %w", ownerName, lstatErr))
			}
			return err
		}
		if current == nil || current.Mode()&fs.ModeSymlink != 0 || !current.Mode().IsRegular() || !sameFileInfo(ownerInfo, current) {
			return errors.Join(err, fmt.Errorf("recordlock: preserve replaced owner %q: %w", ownerName, ErrOwnerMismatch))
		}
		if removeErr := rootRemove(root, ownerName); removeErr != nil {
			return errors.Join(err, fmt.Errorf("recordlock: remove owner %q: %w", ownerName, removeErr))
		}
	}
	if _, verifyErr := verifyPinnedLockPath(path, root, ops); verifyErr != nil {
		return errors.Join(err, fmt.Errorf("recordlock: preserve replaced lock %q: %w", path, verifyErr))
	}
	remove := ops.remove
	if remove == nil {
		remove = os.Remove
	}
	if removeErr := remove(path); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
		err = errors.Join(err, fmt.Errorf("recordlock: remove lock %q: %w", path, removeErr))
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

// Release verifies the persisted owner token before removing this Guard's
// generation.  A mismatch never removes the lock path.
func (g *Guard) Release() (err error) {
	if g == nil || g.state == nil {
		return ErrNotHeld
	}
	state := g.state
	state.mu.Lock()
	if !state.held || state.releasing {
		state.mu.Unlock()
		return ErrNotHeld
	}
	state.releasing = true
	lockRoot := state.lockRoot
	state.mu.Unlock()
	<-state.leaseTokens
	defer func() { state.leaseTokens <- struct{}{} }()

	rootClose := state.ops.rootClose
	if rootClose == nil {
		rootClose = (*os.Root).Close
	}
	defer func() {
		if lockRoot == nil {
			return
		}
		state.mu.Lock()
		held := state.held
		state.mu.Unlock()
		// A retryable failure leaves the pinned acquisition root open for the
		// next Release attempt.
		if held {
			return
		}
		if closeErr := rootClose(lockRoot); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("recordlock: close lock root %q: %w", state.path, closeErr))
		}
		state.mu.Lock()
		if state.lockRoot == lockRoot {
			state.lockRoot = nil
		}
		state.mu.Unlock()
	}()

	lockInfo, err := state.ops.lstat(state.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return finishReleaseError(state, fmt.Errorf("recordlock: lock disappeared at %q: %w", state.path, ErrOwnerMismatch))
		}
		return finishReleaseError(state, fmt.Errorf("recordlock: inspect lock %q: %w", state.path, err))
	}
	if lockInfo == nil || lockInfo.Mode()&fs.ModeSymlink != 0 || !lockInfo.IsDir() {
		return finishReleaseError(state, fmt.Errorf("recordlock: lock path changed at %q: %w", state.path, ErrOwnerMismatch))
	}
	if lockRoot == nil {
		return finishReleaseError(state, fmt.Errorf("recordlock: pinned lock root %q is unavailable: %w", state.path, ErrOwnerMismatch))
	}
	openedLockInfo, err := lockRoot.Stat(".")
	if err != nil {
		return finishReleaseError(state, fmt.Errorf("recordlock: stat lock root %q: %w", state.path, err))
	}
	if openedLockInfo == nil || !openedLockInfo.IsDir() || !os.SameFile(lockInfo, openedLockInfo) {
		return finishReleaseError(state, fmt.Errorf("recordlock: lock root changed at %q: %w", state.path, ErrOwnerMismatch))
	}
	ownerName := ownerMarkerName(state.token)
	ownerPath := filepath.Join(state.path, ownerName)
	owner, ownerInfo, err := readOwnerProofAtRoot(lockRoot, ownerName, ownerPath, state.ops)
	if err != nil {
		return finishReleaseError(state, err)
	}
	if owner.Token != state.token {
		return finishReleaseError(state, fmt.Errorf("recordlock: owner changed at %q: %w", state.path, ErrOwnerMismatch))
	}
	if err := verifyOwnerAtRoot(lockRoot, ownerName, ownerInfo, state.ops); err != nil {
		return finishReleaseError(state, err)
	}
	// A token-derived marker name is not an identity proof by itself. Repeat
	// the pinned-root identity check at the removal boundary so a marker
	// replacement that is already observable cannot be removed as ours.
	if err := verifyOwnerAtRoot(lockRoot, ownerName, ownerInfo, state.ops); err != nil {
		return finishReleaseError(state, err)
	}
	rootRemove := state.ops.rootRemove
	if rootRemove == nil {
		rootRemove = (*os.Root).Remove
	}
	if err := rootRemove(lockRoot, ownerName); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return finishReleaseError(state, fmt.Errorf("recordlock: remove owner %q: %w", ownerPath, ErrOwnerMismatch))
		}
		return finishReleaseError(state, fmt.Errorf("recordlock: remove owner %q: %w", ownerPath, err))
	}
	if err := verifyLockPath(state.path, lockInfo, state.ops); err != nil {
		markUnheld(state)
		return err
	}
	removeErr := state.ops.remove(state.path)
	// Once the owner marker is gone, this generation cannot be safely retried;
	// callers receive the removal failure while the Guard becomes unheld.
	markUnheld(state)
	if removeErr != nil {
		return fmt.Errorf("recordlock: remove lock %q: %w", state.path, removeErr)
	}
	return nil
}

func resetReleasing(state *guardState) {
	state.mu.Lock()
	state.releasing = false
	state.mu.Unlock()
}

func finishReleaseError(state *guardState, err error) error {
	if errors.Is(err, ErrOwnerMismatch) {
		markUnheld(state)
		return err
	}
	resetReleasing(state)
	return err
}

func markUnheld(state *guardState) {
	state.mu.Lock()
	state.held = false
	state.releasing = false
	state.mu.Unlock()
}

func readOwnerProofAtRoot(root *os.Root, ownerName, ownerPath string, ops lockOps) (lockOwner, fs.FileInfo, error) {
	if root == nil {
		return lockOwner{}, nil, fmt.Errorf("recordlock: owner root is nil: %w", ErrOwnerMismatch)
	}
	rootLstat := ops.rootLstat
	if rootLstat == nil {
		rootLstat = (*os.Root).Lstat
	}
	rootOpen := ops.rootOpen
	if rootOpen == nil {
		rootOpen = openLockOwner
	}
	before, err := rootLstat(root, ownerName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return lockOwner{}, nil, fmt.Errorf("recordlock: owner disappeared at %q: %w", ownerPath, ErrOwnerMismatch)
		}
		return lockOwner{}, nil, fmt.Errorf("recordlock: inspect owner %q: %w", ownerPath, err)
	}
	if before == nil || before.Mode()&fs.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return lockOwner{}, nil, fmt.Errorf("recordlock: owner path changed at %q: %w", ownerPath, ErrOwnerMismatch)
	}
	// Test seams may replace the pathname here; production reads remain
	// descriptor-relative so a swapped FIFO or symlink cannot block or redirect
	// the owner proof.
	if ops.beforeOwnerProof != nil {
		if err := ops.beforeOwnerProof(ownerPath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return lockOwner{}, nil, fmt.Errorf("recordlock: owner disappeared at %q: %w", ownerPath, ErrOwnerMismatch)
			}
			return lockOwner{}, nil, fmt.Errorf("recordlock: inspect owner %q: %w", ownerPath, err)
		}
	}
	afterRead, err := rootLstat(root, ownerName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return lockOwner{}, nil, fmt.Errorf("recordlock: owner disappeared at %q: %w", ownerPath, ErrOwnerMismatch)
		}
		return lockOwner{}, nil, fmt.Errorf("recordlock: verify owner %q: %w", ownerPath, err)
	}
	if !sameFileInfo(before, afterRead) || afterRead.Mode()&fs.ModeSymlink != 0 || !afterRead.Mode().IsRegular() {
		return lockOwner{}, nil, fmt.Errorf("recordlock: owner path changed at %q: %w", ownerPath, ErrOwnerMismatch)
	}
	ownerFile, err := rootOpen(root, ownerName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return lockOwner{}, nil, fmt.Errorf("recordlock: owner disappeared at %q: %w", ownerPath, ErrOwnerMismatch)
		}
		return lockOwner{}, nil, fmt.Errorf("recordlock: open owner %q: %w", ownerPath, err)
	}
	if ownerFile == nil {
		return lockOwner{}, nil, fmt.Errorf("recordlock: open owner %q returned nil file: %w", ownerPath, ErrOwnerMismatch)
	}
	ownerInfo, err := ownerFile.Stat()
	if err != nil {
		_ = closeLockFile(ownerFile, ops)
		return lockOwner{}, nil, fmt.Errorf("recordlock: stat owner %q: %w", ownerPath, err)
	}
	if ownerInfo == nil || !ownerInfo.Mode().IsRegular() || !sameFileInfo(afterRead, ownerInfo) {
		_ = closeLockFile(ownerFile, ops)
		return lockOwner{}, nil, fmt.Errorf("recordlock: owner descriptor changed at %q: %w", ownerPath, ErrOwnerMismatch)
	}
	data, readErr := io.ReadAll(ownerFile)
	closeErr := closeLockFile(ownerFile, ops)
	if readErr != nil {
		return lockOwner{}, nil, fmt.Errorf("recordlock: read owner descriptor %q: %w", ownerPath, readErr)
	}
	if closeErr != nil {
		return lockOwner{}, nil, fmt.Errorf("recordlock: close owner %q: %w", ownerPath, closeErr)
	}
	var owner lockOwner
	if err := json.Unmarshal(data, &owner); err != nil || owner.Token == "" {
		return lockOwner{}, nil, fmt.Errorf("recordlock: decode owner %q: %w", ownerPath, ErrOwnerMismatch)
	}
	return owner, ownerInfo, nil
}

func closeLockFile(file *os.File, ops lockOps) error {
	if ops.close != nil {
		return ops.close(file)
	}
	return file.Close()
}

func verifyOwnerAtRoot(root *os.Root, ownerName string, expected fs.FileInfo, ops lockOps) error {
	rootLstat := ops.rootLstat
	if rootLstat == nil {
		rootLstat = (*os.Root).Lstat
	}
	current, err := rootLstat(root, ownerName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("recordlock: owner disappeared at %q: %w", ownerName, ErrOwnerMismatch)
		}
		return fmt.Errorf("recordlock: verify owner %q: %w", ownerName, err)
	}
	if current == nil || current.Mode()&fs.ModeSymlink != 0 || !current.Mode().IsRegular() || !sameFileInfo(current, expected) {
		return fmt.Errorf("recordlock: owner path changed at %q: %w", ownerName, ErrOwnerMismatch)
	}
	return nil
}

func verifyOwnerPath(ownerPath string, expected fs.FileInfo, ops lockOps) error {
	current, err := ops.lstat(ownerPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("recordlock: owner disappeared at %q: %w", ownerPath, ErrOwnerMismatch)
		}
		return fmt.Errorf("recordlock: verify owner %q: %w", ownerPath, err)
	}
	if current == nil || current.Mode()&fs.ModeSymlink != 0 || !current.Mode().IsRegular() || !sameFileInfo(current, expected) {
		return fmt.Errorf("recordlock: owner path changed at %q: %w", ownerPath, ErrOwnerMismatch)
	}
	return nil
}

func verifyLockPath(lockPath string, expected fs.FileInfo, ops lockOps) error {
	current, err := ops.lstat(lockPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("recordlock: lock disappeared at %q: %w", lockPath, ErrOwnerMismatch)
		}
		return fmt.Errorf("recordlock: verify lock %q: %w", lockPath, err)
	}
	if current == nil || current.Mode()&fs.ModeSymlink != 0 || !current.IsDir() || !sameFileInfo(current, expected) {
		return fmt.Errorf("recordlock: lock path changed at %q: %w", lockPath, ErrOwnerMismatch)
	}
	return nil
}

func sameFileInfo(first, second fs.FileInfo) bool {
	return first != nil && second != nil && os.SameFile(first, second)
}

func verifyPinnedLockPath(lockPath string, root *os.Root, ops lockOps) (fs.FileInfo, error) {
	if root == nil {
		return nil, fmt.Errorf("recordlock: pinned lock root is nil: %w", ErrOwnerMismatch)
	}
	lstat := ops.lstat
	if lstat == nil {
		lstat = os.Lstat
	}
	current, err := lstat(lockPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("recordlock: lock disappeared at %q: %w", lockPath, ErrOwnerMismatch)
		}
		return nil, fmt.Errorf("recordlock: inspect lock %q: %w", lockPath, err)
	}
	if current == nil || current.Mode()&fs.ModeSymlink != 0 || !current.IsDir() {
		return nil, fmt.Errorf("recordlock: lock path changed at %q: %w", lockPath, ErrOwnerMismatch)
	}
	opened, err := root.Stat(".")
	if err != nil {
		return nil, fmt.Errorf("recordlock: stat pinned lock root %q: %w", lockPath, err)
	}
	if opened == nil || !opened.IsDir() || !os.SameFile(current, opened) {
		return nil, fmt.Errorf("recordlock: lock path changed at %q: %w", lockPath, ErrOwnerMismatch)
	}
	return current, nil
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
