package workspace

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/sori883/ai-dd/src/internal/pathnorm"
)

const (
	workspaceLockSentinel      = "__workspace__"
	workspaceLockMaxRetries    = 600
	workspaceLockRetryInterval = 100 * time.Millisecond
	workspaceLockOwnerName     = "owner.json"
)

type workspaceLockReceipt struct {
	path  string
	token string
}

type workspaceLockOwner struct {
	PID                     int    `json:"pid"`
	StartedAtMS             int64  `json:"startedAtMs"`
	ReapLiveOwnerAfterStale bool   `json:"reapLiveOwnerAfterStale"`
	Token                   string `json:"token"`
}

type workspaceLockSettings struct {
	maxRetries    int
	retryInterval time.Duration
}

type workspaceLockOps struct {
	evalSymlinks func(string) (string, error)
	tempDir      func() string
	mkdir        func(string, fs.FileMode) error
	openFile     func(string, int, fs.FileMode) (*os.File, error)
	write        func(*os.File, []byte) (int, error)
	close        func(*os.File) error
	readFile     func(string) ([]byte, error)
	remove       func(string) error
	now          func() time.Time
	pid          func() int
	random       io.Reader
	wait         func(context.Context, time.Duration) error
}

func systemWorkspaceLockOps() workspaceLockOps {
	return workspaceLockOps{
		evalSymlinks: filepath.EvalSymlinks,
		tempDir:      os.TempDir,
		mkdir:        os.Mkdir,
		openFile:     os.OpenFile,
		write:        (*os.File).Write,
		close:        (*os.File).Close,
		readFile:     os.ReadFile,
		remove:       os.Remove,
		now:          time.Now,
		pid:          os.Getpid,
		random:       rand.Reader,
		wait:         waitForWorkspaceLock,
	}
}

func waitForWorkspaceLock(ctx context.Context, duration time.Duration) error {
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

func workspaceLockPath(
	projectPath string,
	tempDir string,
	evalSymlinks func(string) (string, error),
) string {
	return workspaceLockPathForPlatform(projectPath, tempDir, evalSymlinks, runtime.GOOS)
}

func workspaceLockPathForPlatform(
	projectPath string,
	tempDir string,
	evalSymlinks func(string) (string, error),
	platform string,
) string {
	canonical := filepath.Clean(projectPath)
	if resolved, err := evalSymlinks(canonical); err == nil {
		canonical = resolved
	}
	// MD5 is the upstream lock identity contract, not a security primitive.
	digest := md5.Sum([]byte(workspaceLockIdentity(canonical, platform)))
	name := fmt.Sprintf(".aidlc-audit-%x.lock", digest[:4])
	return filepath.Join(tempDir, name)
}

func workspaceLockIdentity(canonical, platform string) string {
	return normalizeWorkspaceLockCanonical(canonical, platform) + "\x00" + workspaceLockSentinel
}

func normalizeWorkspaceLockCanonical(canonical, platform string) string {
	return pathnorm.NormalizeForPlatform(canonical, platform)
}

// These package-local aliases preserve the workspace tests' focused property
// checks while keeping the implementation in the shared pathnorm package.
func ecmaScriptDefaultLower(value string) string { return pathnorm.ECMAScriptDefaultLower(value) }

func isCased(char rune) bool { return pathnorm.IsCased(char) }

func isCaseIgnorable(char rune) bool { return pathnorm.IsCaseIgnorable(char) }

func acquireWorkspaceLock(
	ctx context.Context,
	projectPath string,
	settings workspaceLockSettings,
	ops workspaceLockOps,
) (workspaceLockReceipt, error) {
	path := workspaceLockPath(projectPath, ops.tempDir(), ops.evalSymlinks)
	for attempt := 0; attempt <= settings.maxRetries; attempt++ {
		if err := context.Cause(ctx); err != nil {
			return workspaceLockReceipt{}, fmt.Errorf("acquire workspace lock %q: %w", path, err)
		}
		err := ops.mkdir(path, 0o700)
		if err == nil {
			return initializeWorkspaceLock(path, ops)
		}
		if !errors.Is(err, fs.ErrExist) {
			return workspaceLockReceipt{}, fmt.Errorf("create workspace lock %q: %w", path, err)
		}
		if attempt == settings.maxRetries {
			if ctxErr := context.Cause(ctx); ctxErr != nil {
				return workspaceLockReceipt{}, fmt.Errorf("acquire workspace lock %q: %w", path, ctxErr)
			}
			return workspaceLockReceipt{}, fmt.Errorf(
				"acquire workspace lock %q after %d retries: %w",
				path,
				settings.maxRetries,
				fs.ErrExist,
			)
		}
		if err := ops.wait(ctx, settings.retryInterval); err != nil {
			return workspaceLockReceipt{}, fmt.Errorf("wait for workspace lock %q: %w", path, err)
		}
	}
	panic("unreachable workspace lock retry loop")
}

func initializeWorkspaceLock(path string, ops workspaceLockOps) (receipt workspaceLockReceipt, err error) {
	token, err := randomUUIDV4(ops.random)
	if err != nil {
		return workspaceLockReceipt{}, errors.Join(
			fmt.Errorf("create workspace lock token: %w", err),
			cleanupWorkspaceLock(path, "", ops),
		)
	}
	if err := ops.mkdir(filepath.Join(path, token), 0o700); err != nil {
		return workspaceLockReceipt{}, errors.Join(
			fmt.Errorf("create workspace lock generation: %w", err),
			cleanupWorkspaceLock(path, token, ops),
		)
	}
	owner := workspaceLockOwner{
		PID:                     ops.pid(),
		StartedAtMS:             ops.now().UnixMilli(),
		ReapLiveOwnerAfterStale: false,
		Token:                   token,
	}
	if err := writeWorkspaceLockOwner(filepath.Join(path, workspaceLockOwnerName), owner, ops); err != nil {
		return workspaceLockReceipt{}, errors.Join(err, cleanupWorkspaceLock(path, token, ops))
	}
	return workspaceLockReceipt{path: path, token: token}, nil
}

func randomUUIDV4(random io.Reader) (string, error) {
	var value [16]byte
	if _, err := io.ReadFull(random, value[:]); err != nil {
		return "", fmt.Errorf("read UUIDv4 entropy: %w", err)
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		value[:4],
		value[4:6],
		value[6:8],
		value[8:10],
		value[10:],
	), nil
}

func writeWorkspaceLockOwner(
	name string,
	owner workspaceLockOwner,
	ops workspaceLockOps,
) (err error) {
	data, err := json.Marshal(owner)
	if err != nil {
		return fmt.Errorf("encode workspace lock owner: %w", err)
	}
	file, err := ops.openFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("open workspace lock owner %q: %w", name, err)
	}
	n, writeErr := ops.write(file, data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr != nil {
		err = fmt.Errorf("write workspace lock owner %q: %w", name, writeErr)
	}
	if closeErr := ops.close(file); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("close workspace lock owner %q: %w", name, closeErr))
	}
	return err
}

func cleanupWorkspaceLock(path, token string, ops workspaceLockOps) error {
	paths := []string{filepath.Join(path, workspaceLockOwnerName)}
	if token != "" {
		paths = append(paths, filepath.Join(path, token))
	}
	paths = append(paths, path)
	var cleanupErr error
	for _, name := range paths {
		if err := ops.remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove workspace lock path %q: %w", name, err))
		}
	}
	return cleanupErr
}

func releaseWorkspaceLock(receipt workspaceLockReceipt, ops workspaceLockOps) error {
	ownerPath := filepath.Join(receipt.path, workspaceLockOwnerName)
	data, err := ops.readFile(ownerPath)
	if err != nil {
		return fmt.Errorf("read workspace lock owner %q: %w", ownerPath, err)
	}
	var owner workspaceLockOwner
	if err := json.Unmarshal(data, &owner); err != nil || owner.Token == "" {
		return fmt.Errorf("decode workspace lock owner %q: %w", ownerPath, fs.ErrInvalid)
	}
	if owner.Token != receipt.token {
		return fmt.Errorf("workspace lock owner changed at %q: %w", receipt.path, fs.ErrPermission)
	}
	for _, name := range []string{
		filepath.Join(receipt.path, receipt.token),
		ownerPath,
		receipt.path,
	} {
		if err := ops.remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("release workspace lock path %q: %w", name, err)
		}
	}
	return nil
}
