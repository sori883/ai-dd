package workspace

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// IntentCreateInput contains caller-resolved creation values for one existing space.
type IntentCreateInput struct {
	SpaceName string
	Label     string
	Scope     *string
	Repos     []string
}

// CreatedIntent identifies an intent whose registry row reached the durable commit boundary.
type CreatedIntent struct {
	UUID      string
	Slug      string
	DirName   string
	RecordDir string
	SpaceName string
}

type intentCreateOps struct {
	acquireLock         func(context.Context, string) (workspaceLockReceipt, error)
	releaseLock         func(workspaceLockReceipt) error
	openProject         func(string) (*os.Root, error)
	openChild           func(*os.Root, string) (*os.Root, error)
	closeRoot           func(*os.Root) error
	readRegistry        func(*os.Root) ([]json.RawMessage, error)
	now                 func() time.Time
	uuid                func(time.Time) (string, error)
	resolveDir          func(*os.Root, string) (string, error)
	createRecord        func(*os.Root, string) error
	writeRegistry       func(*os.Root, []json.RawMessage, intentRegistryEntry) error
	completeActiveSpace func(*os.Root, string) error
	saveActiveIntent    func(*os.Root, string) error
}

// CreateIntent creates an intent core transaction inside an existing space.
// The caller supplies already-resolved scope and repository values. A registry
// replacement is the commit boundary: failures before it return a zero result,
// while later cursor, close, or lock-release failures return both the committed
// result and an error. The function does not roll back partial artifacts.
func CreateIntent(
	ctx context.Context,
	root RootInput,
	input IntentCreateInput,
) (CreatedIntent, error) {
	return createIntent(ctx, root, input, systemIntentCreateOps())
}

func systemIntentCreateOps() intentCreateOps {
	lockOps := systemWorkspaceLockOps()
	return intentCreateOps{
		acquireLock: func(ctx context.Context, projectPath string) (workspaceLockReceipt, error) {
			return acquireWorkspaceLock(
				ctx,
				projectPath,
				workspaceLockSettings{
					maxRetries:    workspaceLockMaxRetries,
					retryInterval: workspaceLockRetryInterval,
				},
				lockOps,
			)
		},
		releaseLock: func(receipt workspaceLockReceipt) error {
			return releaseWorkspaceLock(receipt, lockOps)
		},
		openProject: os.OpenRoot,
		openChild:   (*os.Root).OpenRoot,
		closeRoot:   (*os.Root).Close,
		readRegistry: func(root *os.Root) ([]json.RawMessage, error) {
			return readIntentRegistryForMutation(registryReadOperations(root))
		},
		now: time.Now,
		uuid: func(now time.Time) (string, error) {
			return uuidV7(now, rand.Reader)
		},
		resolveDir: func(root *os.Root, base string) (string, error) {
			return resolveIntentDirName(base, root.Lstat)
		},
		createRecord: func(root *os.Root, dirName string) error {
			return createIntentRecord(dirName, intentRecordOps{
				mkdir:    root.Mkdir,
				openFile: root.OpenFile,
				write:    (*os.File).WriteString,
				close:    (*os.File).Close,
			})
		},
		writeRegistry: func(
			root *os.Root,
			rows []json.RawMessage,
			entry intentRegistryEntry,
		) error {
			return writeIntentRegistry(rows, entry, registryWriteOperations(root))
		},
		completeActiveSpace: completeActiveSpaceCursor,
		saveActiveIntent:    saveIntentCursor,
	}
}

func createIntent(
	ctx context.Context,
	root RootInput,
	input IntentCreateInput,
	ops intentCreateOps,
) (created CreatedIntent, err error) {
	if ctxErr := context.Cause(ctx); ctxErr != nil {
		return CreatedIntent{}, ctxErr
	}
	projectPath := ResolveRoot(root)
	if !filepath.IsAbs(projectPath) {
		return CreatedIntent{}, fmt.Errorf("resolve project root %q: %w", projectPath, fs.ErrInvalid)
	}
	spacePath, err := localizeSpace(input.SpaceName)
	if err != nil {
		return CreatedIntent{}, err
	}
	repos := append([]string{}, input.Repos...)
	var scope *string
	if input.Scope != nil {
		value := *input.Scope
		scope = &value
	}
	committed := false
	lock, err := ops.acquireLock(ctx, projectPath)
	if err != nil {
		return CreatedIntent{}, err
	}
	defer func() {
		if releaseErr := ops.releaseLock(lock); releaseErr != nil {
			if !committed {
				created = CreatedIntent{}
			}
			err = errors.Join(err, fmt.Errorf("release workspace lock: %w", releaseErr))
		}
	}()

	projectRoot, err := ops.openProject(projectPath)
	if err != nil {
		return CreatedIntent{}, fmt.Errorf("open project root %q: %w", projectPath, err)
	}
	defer func() {
		if closeErr := ops.closeRoot(projectRoot); closeErr != nil {
			if !committed {
				created = CreatedIntent{}
			}
			err = errors.Join(err, fmt.Errorf("close project root %q: %w", projectPath, closeErr))
		}
	}()

	childPath := filepath.Join("aidlc", "spaces", spacePath, "intents")
	intentsRoot, err := ops.openChild(projectRoot, childPath)
	if err != nil {
		return CreatedIntent{}, fmt.Errorf("open intents root %q: %w", childPath, err)
	}
	defer func() {
		if closeErr := ops.closeRoot(intentsRoot); closeErr != nil {
			if !committed {
				created = CreatedIntent{}
			}
			err = errors.Join(err, fmt.Errorf("close intents root %q: %w", childPath, closeErr))
		}
	}()

	rows, err := ops.readRegistry(intentsRoot)
	if err != nil {
		return CreatedIntent{}, err
	}
	now := ops.now()
	uuid, err := ops.uuid(now)
	if err != nil {
		return CreatedIntent{}, err
	}
	base, err := intentDirBase(input.Label, now)
	if err != nil {
		return CreatedIntent{}, err
	}
	dirName, err := ops.resolveDir(intentsRoot, base)
	if err != nil {
		return CreatedIntent{}, err
	}
	if err := ops.createRecord(intentsRoot, dirName); err != nil {
		return CreatedIntent{}, err
	}
	slug := intentSlug(input.Label)
	entry := intentRegistryEntry{
		UUID: uuid, Slug: slug, DirName: dirName, Scope: scope, Repos: repos, Status: "in-flight",
	}
	if err := ops.writeRegistry(intentsRoot, rows, entry); err != nil {
		return CreatedIntent{}, err
	}
	committed = true
	created = CreatedIntent{
		UUID:      uuid,
		Slug:      slug,
		DirName:   dirName,
		RecordDir: filepath.Join(projectPath, childPath, dirName),
		SpaceName: input.SpaceName,
	}
	if err := ops.completeActiveSpace(projectRoot, input.SpaceName); err != nil {
		return created, err
	}
	if err := ops.saveActiveIntent(intentsRoot, dirName); err != nil {
		return created, err
	}
	return created, nil
}

func uuidV7(now time.Time, random io.Reader) (string, error) {
	var value [16]byte
	if _, err := io.ReadFull(random, value[6:]); err != nil {
		return "", fmt.Errorf("read UUIDv7 entropy: %w", err)
	}
	milliseconds := uint64(now.UnixMilli())
	for index := 5; index >= 0; index-- {
		value[index] = byte(milliseconds)
		milliseconds >>= 8
	}
	value[6] = value[6]&0x0f | 0x70
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

func intentSlug(raw string) string {
	return workspaceSlug(raw, 24)
}

func normalizeIntentLabel(raw string) (string, error) {
	slug := intentSlug(raw)
	switch slug {
	case "help", "list", "switch", "create", "archive", "rename", "show", "birth":
		return "", fmt.Errorf("reserved intent label %q: %w", slug, fs.ErrInvalid)
	default:
		return slug, nil
	}
}

func intentDirBase(rawLabel string, now time.Time) (string, error) {
	slug, err := normalizeIntentLabel(rawLabel)
	if err != nil {
		return "", err
	}
	return now.UTC().Format("060102") + "-" + slug, nil
}

func resolveIntentDirName(
	base string,
	lstat func(string) (fs.FileInfo, error),
) (string, error) {
	for suffix := 1; suffix <= 999; suffix++ {
		candidate := base
		if suffix > 1 {
			candidate += "-" + strconv.Itoa(suffix)
		}
		_, err := lstat(candidate)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return candidate, nil
		case err != nil:
			return "", fmt.Errorf("inspect intent directory %q: %w", candidate, err)
		}
	}
	return "", fmt.Errorf("find free intent directory for %q through suffix -999: %w", base, fs.ErrExist)
}

const intentStateStub = "# AI-DLC State Tracking\n"

type intentRecordOps struct {
	mkdir    func(string, fs.FileMode) error
	openFile func(string, int, fs.FileMode) (*os.File, error)
	write    func(*os.File, string) (int, error)
	close    func(*os.File) error
}

func createIntentRecord(dirName string, ops intentRecordOps) (err error) {
	if err := ops.mkdir(dirName, 0o777); err != nil {
		return fmt.Errorf("create intent directory %q: %w", dirName, err)
	}
	statePath := filepath.Join(dirName, "aidlc-state.md")
	file, err := ops.openFile(statePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return fmt.Errorf("open new intent state %q: %w", statePath, err)
	}
	defer func() {
		if closeErr := ops.close(file); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close new intent state %q: %w", statePath, closeErr))
		}
	}()
	n, err := ops.write(file, intentStateStub)
	if err == nil && n != len(intentStateStub) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return fmt.Errorf("write new intent state %q: %w", statePath, err)
	}
	return nil
}
