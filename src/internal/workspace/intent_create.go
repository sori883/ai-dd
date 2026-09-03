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

// IntentInitializer performs follow-up initialization while the workspace
// lock and the project and record roots are still held by CreateIntent.
type IntentInitializer func(
	projectRoot *os.Root,
	recordRoot *os.Root,
	created CreatedIntent,
) error

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
	activeSpace         func(*os.Root) string
	completeActiveSpace func(*os.Root, string) error
	saveActiveIntent    func(*os.Root, string) error
}

type intentCreateLockedOps struct {
	openProject         func(string) (*os.Root, error)
	openChild           func(*os.Root, string) (*os.Root, error)
	closeRoot           func(*os.Root) error
	readRegistry        func(*os.Root) ([]json.RawMessage, error)
	now                 func() time.Time
	uuid                func(time.Time) (string, error)
	resolveDir          func(*os.Root, string) (string, error)
	createRecord        func(*os.Root, string) error
	writeRegistry       func(*os.Root, []json.RawMessage, intentRegistryEntry) error
	activeSpace         func(*os.Root) string
	completeActiveSpace func(*os.Root, string) error
	saveActiveIntent    func(*os.Root, string) error
}

type preparedIntentCreate struct {
	projectPath string
	spacePath   string
	input       IntentCreateInput
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
	return createIntentWithInitializer(ctx, root, input, systemIntentCreateOps(), nil)
}

// CreateIntentWithInitializer creates an intent and invokes initialize after
// the registry and cursor work has been attempted. The initializer runs while
// the workspace lock is held and receives explicit project and record roots;
// both roots remain owned and are closed by this function.
func CreateIntentWithInitializer(
	ctx context.Context,
	root RootInput,
	input IntentCreateInput,
	initialize IntentInitializer,
) (CreatedIntent, error) {
	return createIntentWithInitializer(
		ctx,
		root,
		input,
		systemIntentCreateOps(),
		initialize,
	)
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
		activeSpace: func(root *os.Root) string {
			return ActiveSpace(root.FS())
		},
		completeActiveSpace: completeActiveSpaceCursor,
		saveActiveIntent:    saveIntentCursor,
	}
}

func (ops intentCreateOps) lockedOperations() intentCreateLockedOps {
	return intentCreateLockedOps{
		openProject:         ops.openProject,
		openChild:           ops.openChild,
		closeRoot:           ops.closeRoot,
		readRegistry:        ops.readRegistry,
		now:                 ops.now,
		uuid:                ops.uuid,
		resolveDir:          ops.resolveDir,
		createRecord:        ops.createRecord,
		writeRegistry:       ops.writeRegistry,
		activeSpace:         ops.activeSpace,
		completeActiveSpace: ops.completeActiveSpace,
		saveActiveIntent:    ops.saveActiveIntent,
	}
}

func createIntent(
	ctx context.Context,
	root RootInput,
	input IntentCreateInput,
	ops intentCreateOps,
) (CreatedIntent, error) {
	return createIntentWithInitializer(ctx, root, input, ops, nil)
}

func createIntentWithCallback(
	ctx context.Context,
	root RootInput,
	input IntentCreateInput,
	ops intentCreateOps,
	after func(CreatedIntent) error,
) (CreatedIntent, error) {
	var initialize IntentInitializer
	if after != nil {
		initialize = func(_ *os.Root, _ *os.Root, created CreatedIntent) error {
			return after(created)
		}
	}
	return createIntentWithInitializer(ctx, root, input, ops, initialize)
}

func createIntentWithInitializer(
	ctx context.Context,
	root RootInput,
	input IntentCreateInput,
	ops intentCreateOps,
	initialize IntentInitializer,
) (CreatedIntent, error) {
	if ctxErr := context.Cause(ctx); ctxErr != nil {
		return CreatedIntent{}, ctxErr
	}
	prepared, err := prepareIntentCreate(root, input)
	if err != nil {
		return CreatedIntent{}, err
	}
	return withIntentCreateLock(ctx, prepared.projectPath, ops, func() (CreatedIntent, bool, error) {
		return createIntentLockedWithInitializer(
			prepared,
			ops.lockedOperations(),
			initialize,
		)
	})
}

func prepareIntentCreate(root RootInput, input IntentCreateInput) (preparedIntentCreate, error) {
	projectPath := ResolveRoot(root)
	if !filepath.IsAbs(projectPath) {
		return preparedIntentCreate{}, fmt.Errorf(
			"resolve project root %q: %w",
			projectPath,
			fs.ErrInvalid,
		)
	}
	spacePath, err := localizeSpace(input.SpaceName)
	if err != nil {
		return preparedIntentCreate{}, err
	}
	input.Repos = append([]string{}, input.Repos...)
	if input.Scope != nil {
		value := *input.Scope
		input.Scope = &value
	}
	return preparedIntentCreate{projectPath: projectPath, spacePath: spacePath, input: input}, nil
}

func withIntentCreateLock(
	ctx context.Context,
	projectPath string,
	ops intentCreateOps,
	run func() (CreatedIntent, bool, error),
) (created CreatedIntent, err error) {
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
	created, committed, err = run()
	return created, err
}

func createIntentLocked(
	prepared preparedIntentCreate,
	ops intentCreateLockedOps,
) (created CreatedIntent, committed bool, err error) {
	return createIntentLockedWithInitializer(prepared, ops, nil)
}

func createIntentLockedWithInitializer(
	prepared preparedIntentCreate,
	ops intentCreateLockedOps,
	initialize IntentInitializer,
) (created CreatedIntent, committed bool, err error) {
	projectRoot, err := ops.openProject(prepared.projectPath)
	if err != nil {
		return CreatedIntent{}, false, fmt.Errorf(
			"open project root %q: %w",
			prepared.projectPath,
			err,
		)
	}
	defer func() {
		if closeErr := ops.closeRoot(projectRoot); closeErr != nil {
			if !committed {
				created = CreatedIntent{}
			}
			err = errors.Join(
				err,
				fmt.Errorf("close project root %q: %w", prepared.projectPath, closeErr),
			)
		}
	}()

	childPath := filepath.Join("aidlc", "spaces", prepared.spacePath, "intents")
	intentsRoot, err := ops.openChild(projectRoot, childPath)
	if err != nil {
		return CreatedIntent{}, false, fmt.Errorf("open intents root %q: %w", childPath, err)
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
		return CreatedIntent{}, false, err
	}
	now := ops.now()
	uuid, err := ops.uuid(now)
	if err != nil {
		return CreatedIntent{}, false, err
	}
	base, err := intentDirBase(prepared.input.Label, now)
	if err != nil {
		return CreatedIntent{}, false, err
	}
	dirName, err := ops.resolveDir(intentsRoot, base)
	if err != nil {
		return CreatedIntent{}, false, err
	}
	if err := ops.createRecord(intentsRoot, dirName); err != nil {
		return CreatedIntent{}, false, err
	}
	slug := intentSlug(prepared.input.Label)
	entry := intentRegistryEntry{
		UUID: uuid, Slug: slug, DirName: dirName,
		Scope: prepared.input.Scope, Repos: prepared.input.Repos, Status: "in-flight",
	}
	if err := ops.writeRegistry(intentsRoot, rows, entry); err != nil {
		return CreatedIntent{}, false, err
	}
	committed = true
	created = CreatedIntent{
		UUID:      uuid,
		Slug:      slug,
		DirName:   dirName,
		RecordDir: filepath.Join(prepared.projectPath, childPath, dirName),
		SpaceName: prepared.input.SpaceName,
	}
	sharedSpaceName := ops.activeSpace(projectRoot)
	if err := ops.completeActiveSpace(projectRoot, sharedSpaceName); err != nil {
		if initialize == nil {
			return created, committed, err
		}
		return initializeIntentAfterCursor(
			created,
			projectRoot,
			intentsRoot,
			dirName,
			err,
			ops,
			initialize,
		)
	}
	if err := ops.saveActiveIntent(intentsRoot, dirName); err != nil {
		if initialize == nil {
			return created, committed, err
		}
		return initializeIntentAfterCursor(
			created,
			projectRoot,
			intentsRoot,
			dirName,
			err,
			ops,
			initialize,
		)
	}
	if initialize == nil {
		return created, committed, nil
	}
	return initializeIntentAfterCursor(
		created,
		projectRoot,
		intentsRoot,
		dirName,
		nil,
		ops,
		initialize,
	)
}

func initializeIntentAfterCursor(
	created CreatedIntent,
	projectRoot *os.Root,
	intentsRoot *os.Root,
	dirName string,
	cursorErr error,
	ops intentCreateLockedOps,
	initialize IntentInitializer,
) (result CreatedIntent, committed bool, err error) {
	recordRoot, err := ops.openChild(intentsRoot, dirName)
	if err != nil {
		return created, true, errors.Join(
			cursorErr,
			fmt.Errorf("open intent record root %q: %w", dirName, err),
		)
	}
	defer func() {
		if closeErr := ops.closeRoot(recordRoot); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close intent record root %q: %w", dirName, closeErr))
		}
	}()

	initializeErr := initialize(projectRoot, recordRoot, created)
	if initializeErr != nil {
		initializeErr = fmt.Errorf("initialize intent %q: %w", dirName, initializeErr)
	}
	return created, true, errors.Join(cursorErr, initializeErr)
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
