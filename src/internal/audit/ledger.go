// Package audit appends the small, per-clone AI-DLC audit ledger.
package audit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/sori883/ai-dd/src/internal/pathnorm"
	"github.com/sori883/ai-dd/src/internal/recordlock"
)

const (
	cloneIDDirectory  = "aidlc"
	cloneIDFile       = ".aidlc-clone-id"
	cloneIDLength     = 12
	auditDirectory    = "audit"
	cloneIDMaxRetries = 20
	cloneIDRetryDelay = 5 * time.Millisecond
)

var (
	ErrInvalidEvent    = errors.New("audit: invalid event")
	ErrInvalidField    = errors.New("audit: invalid field")
	ErrInvalidBatch    = errors.New("audit: invalid event batch")
	ErrInvalidCloneID  = errors.New("audit: invalid clone id")
	ErrGuardNotHeld    = errors.New("audit: guard is not held")
	ErrGuardIdentity   = errors.New("audit: guard identity mismatch")
	ErrInvalidRoot     = errors.New("audit: invalid root")
	ErrNoWriteProgress = errors.New("audit: write made no progress")
)

// Event is one canonical audit block. Event is the preferred event type field;
// EventType is accepted as a descriptive alias for callers that use the JSON
// vocabulary from the fixed AI-DLC snapshot. Supplying both requires equality.
// Timestamp is retained for source compatibility but is never authoritative;
// Append always obtains its timestamp from the ledger's private clock seam.
type Event struct {
	Event     string
	EventType string
	Timestamp time.Time
	Fields    map[string]string
}

var eventHeadings = map[string]string{
	"HUMAN_TURN":              "Human Turn",
	"STAGE_AWAITING_APPROVAL": "Stage Awaiting Approval",
	"GATE_APPROVED":           "Gate Approved",
	"GATE_REJECTED":           "Gate Rejected",
	"STAGE_REVISING":          "Stage Revising",
	"STAGE_COMPLETED":         "Stage Completion",
	"PHASE_COMPLETED":         "Phase Completion",
	"PHASE_VERIFIED":          "Phase Verification",
	"PHASE_STARTED":           "Phase Start",
	"STAGE_STARTED":           "Stage Start",
	"WORKFLOW_COMPLETED":      "Workflow Completion",
}

var validFieldKey = func(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		if index == 0 {
			if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')) {
				return false
			}
			continue
		}
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') || strings.ContainsRune(" ._()/-", char) {
			continue
		}
		return false
	}
	return true
}

func validateEvent(event Event) (string, error) {
	eventType := event.Event
	if eventType == "" {
		eventType = event.EventType
	} else if event.EventType != "" && event.EventType != event.Event {
		return "", fmt.Errorf("audit: Event and EventType differ: %w", ErrInvalidEvent)
	}
	if _, ok := eventHeadings[eventType]; !ok {
		return "", fmt.Errorf("audit: event %q: %w", eventType, ErrInvalidEvent)
	}
	for key := range event.Fields {
		if key == "Timestamp" || key == "Event" || !validFieldKey(key) {
			return "", fmt.Errorf("audit: field %q: %w", key, ErrInvalidField)
		}
	}
	return eventType, nil
}

type renderedEvent struct {
	eventType string
	timestamp string
	block     []byte
}

func renderEvents(events []Event, now func() time.Time) ([]renderedEvent, error) {
	if len(events) == 0 {
		return nil, ErrInvalidBatch
	}
	if now == nil {
		return nil, fmt.Errorf("audit: append clock is required: %w", ErrInvalidBatch)
	}
	rendered := make([]renderedEvent, len(events))
	for index, event := range events {
		eventType, err := validateEvent(event)
		if err != nil {
			return nil, err
		}
		at := now()
		at = at.UTC()
		rendered[index] = renderedEvent{
			eventType: eventType,
			timestamp: formatTimestamp(at),
			block:     []byte(renderBlock(eventType, at, event.Fields)),
		}
	}
	return rendered, nil
}

func renderBlock(eventType string, timestamp time.Time, fields map[string]string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "\n## %s\n", eventHeadings[eventType])
	fmt.Fprintf(&builder, "**Timestamp**: %s\n", formatTimestamp(timestamp))
	fmt.Fprintf(&builder, "**Event**: %s\n", eventType)
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&builder, "**%s**: %s\n", key, escapeLineTerminators(fields[key]))
	}
	builder.WriteString("\n---\n")
	return builder.String()
}

// formatTimestamp emits authority timestamps at UTC-second precision.  The
// caller-supplied Event.Timestamp is intentionally not part of this path.
func formatTimestamp(timestamp time.Time) string {
	return timestamp.UTC().Truncate(time.Second).Format(time.RFC3339)
}

func escapeLineTerminators(value string) string {
	return strings.NewReplacer(
		"\r\n", "\\n",
		"\r", "\\n",
		"\n", "\\n",
		"\u2028", "\\n",
		"\u2029", "\\n",
	).Replace(value)
}

type ledgerOps struct {
	projectLstat      func(string) (fs.FileInfo, error)
	projectMkdir      func(string, fs.FileMode) error
	projectOpen       func(string, int, fs.FileMode) (*os.File, error)
	projectRead       func(string) ([]byte, error)
	projectRemove     func(string) error
	recordLstat       func(string) (fs.FileInfo, error)
	recordMkdir       func(string, fs.FileMode) error
	recordOpen        func(string, int, fs.FileMode) (*os.File, error)
	write             func(*os.File, []byte) (int, error)
	close             func(*os.File) error
	hostname          func() (string, error)
	random            io.Reader
	wait              func(context.Context, time.Duration) error
	now               func() time.Time
	beforeCloneCreate func(context.Context) error
}

func systemLedgerOps(projectRoot, recordRoot *os.Root) ledgerOps {
	return ledgerOps{
		projectLstat:  projectRoot.Lstat,
		projectMkdir:  projectRoot.Mkdir,
		projectOpen:   projectRoot.OpenFile,
		projectRead:   projectRoot.ReadFile,
		projectRemove: projectRoot.Remove,
		recordLstat:   recordRoot.Lstat,
		recordMkdir:   recordRoot.Mkdir,
		recordOpen:    recordRoot.OpenFile,
		write:         (*os.File).Write,
		close:         (*os.File).Close,
		hostname:      os.Hostname,
		random:        rand.Reader,
		wait:          waitForCloneID,
		now:           time.Now,
	}
}

// Append validates and appends events to the per-clone shard under recordRoot.
// The caller owns both Roots and the Guard; this function never closes any of
// them. guard must be held and its canonical project identity must match
// projectRoot. The caller is responsible for holding that Guard across any
// related state/audit transaction.
func Append(ctx context.Context, guard *recordlock.Guard, projectRoot, recordRoot *os.Root, events []Event) error {
	if guard == nil {
		return ErrGuardNotHeld
	}
	return AppendForIdentity(ctx, guard.Identity(), guard, projectRoot, recordRoot, events)
}

// AppendForIdentity is the explicit form used when a caller has a record
// identity separate from the Guard. Both identities must match exactly; this
// prevents a held lock for a sibling intent from authorizing this record's
// append.
func AppendForIdentity(ctx context.Context, identity recordlock.Identity, guard *recordlock.Guard, projectRoot, recordRoot *os.Root, events []Event) error {
	if ctx == nil {
		return fmt.Errorf("audit: append nil context: %w", fs.ErrInvalid)
	}
	return appendForIdentityWithOps(ctx, identity, guard, projectRoot, recordRoot, events, nil)
}

func appendForIdentityWithOps(ctx context.Context, expected recordlock.Identity, guard *recordlock.Guard, projectRoot, recordRoot *os.Root, events []Event, injected *ledgerOps) error {
	if err := context.Cause(ctx); err != nil {
		return fmt.Errorf("audit: append: %w", err)
	}
	if projectRoot == nil || recordRoot == nil {
		return fmt.Errorf("audit: project and record roots are required: %w", ErrInvalidRoot)
	}
	if guard == nil || !guard.Held() {
		return ErrGuardNotHeld
	}
	if guard.Identity() != expected {
		return fmt.Errorf("audit: guard identity differs from requested record: %w", ErrGuardIdentity)
	}
	ops := systemLedgerOps(projectRoot, recordRoot)
	if injected != nil {
		ops = mergeLedgerOps(ops, *injected)
	}
	rendered, err := renderEvents(events, ops.now)
	if err != nil {
		return err
	}
	return guard.WithLease(ctx, func() error {
		if !sameProjectRoot(projectRoot.Name(), expected.ProjectRoot()) {
			return fmt.Errorf("audit: project root does not match guard identity: %w", ErrGuardIdentity)
		}
		if err := bindRecordRoot(expected, projectRoot, recordRoot); err != nil {
			return err
		}
		cloneID, err := ensureCloneID(ctx, ops)
		if err != nil {
			return err
		}
		host, err := ops.hostname()
		if err != nil {
			return fmt.Errorf("audit: resolve host: %w", err)
		}
		shard := path.Join(auditDirectory, shardName(host, cloneID))
		if err := ensureAuditDirectory(ops); err != nil {
			return err
		}
		return appendRendered(shard, rendered, ops)
	})
}

func bindRecordRoot(identity recordlock.Identity, projectRoot, recordRoot *os.Root) error {
	expectedPath := path.Join(cloneIDDirectory, "spaces", identity.Space(), "intents", identity.Intent())
	expectedInfo, err := projectRoot.Lstat(expectedPath)
	if err != nil {
		return fmt.Errorf("audit: inspect expected record root %q: %w", expectedPath, err)
	}
	if expectedInfo == nil || expectedInfo.Mode()&fs.ModeSymlink != 0 || !expectedInfo.IsDir() {
		return fmt.Errorf("audit: expected record root %q must be a directory: %w", expectedPath, ErrInvalidRoot)
	}
	actualInfo, err := recordRoot.Stat(".")
	if err != nil {
		return fmt.Errorf("audit: inspect supplied record root: %w", err)
	}
	if actualInfo == nil || !actualInfo.IsDir() || !os.SameFile(expectedInfo, actualInfo) {
		return fmt.Errorf("audit: supplied record root is not expected intent %q: %w", expectedPath, ErrInvalidRoot)
	}
	return nil
}

func mergeLedgerOps(base, override ledgerOps) ledgerOps {
	if override.projectLstat != nil {
		base.projectLstat = override.projectLstat
	}
	if override.projectMkdir != nil {
		base.projectMkdir = override.projectMkdir
	}
	if override.projectOpen != nil {
		base.projectOpen = override.projectOpen
	}
	if override.projectRead != nil {
		base.projectRead = override.projectRead
	}
	if override.projectRemove != nil {
		base.projectRemove = override.projectRemove
	}
	if override.recordLstat != nil {
		base.recordLstat = override.recordLstat
	}
	if override.recordMkdir != nil {
		base.recordMkdir = override.recordMkdir
	}
	if override.recordOpen != nil {
		base.recordOpen = override.recordOpen
	}
	if override.write != nil {
		base.write = override.write
	}
	if override.close != nil {
		base.close = override.close
	}
	if override.hostname != nil {
		base.hostname = override.hostname
	}
	if override.random != nil {
		base.random = override.random
	}
	if override.wait != nil {
		base.wait = override.wait
	}
	if override.now != nil {
		base.now = override.now
	}
	if override.beforeCloneCreate != nil {
		base.beforeCloneCreate = override.beforeCloneCreate
	}
	return base
}

func sameProjectRoot(got, want string) bool {
	if got == "" || want == "" {
		return false
	}
	gotAbs, err := filepath.Abs(got)
	if err != nil {
		return false
	}
	wantAbs, err := filepath.Abs(want)
	if err != nil {
		return false
	}
	gotAbs = filepath.Clean(gotAbs)
	wantAbs = filepath.Clean(wantAbs)
	if gotReal, realErr := filepath.EvalSymlinks(gotAbs); realErr == nil {
		gotAbs = filepath.Clean(gotReal)
	}
	if wantReal, realErr := filepath.EvalSymlinks(wantAbs); realErr == nil {
		wantAbs = filepath.Clean(wantReal)
	}
	gotAbs = pathnorm.NormalizeForPlatform(gotAbs, runtime.GOOS)
	wantAbs = pathnorm.NormalizeForPlatform(wantAbs, runtime.GOOS)
	return gotAbs == wantAbs
}

func ensureCloneID(ctx context.Context, ops ledgerOps) (string, error) {
	if err := ensureProjectDirectory(ops); err != nil {
		return "", err
	}
	cloneIDPath := path.Join(cloneIDDirectory, cloneIDFile)
	for attempt := 0; attempt <= cloneIDMaxRetries; attempt++ {
		info, lstatErr := ops.projectLstat(cloneIDPath)
		if lstatErr == nil && (info == nil || info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular()) {
			return "", fmt.Errorf("audit: clone id must be a regular non-symlink file: %w", ErrInvalidRoot)
		}
		if lstatErr != nil && !errors.Is(lstatErr, fs.ErrNotExist) {
			return "", fmt.Errorf("audit: inspect clone id: %w", lstatErr)
		}
		raw, err := ops.projectRead(cloneIDPath)
		if err == nil {
			cloneID, parseErr := parseCloneID(raw)
			if parseErr != nil {
				// A competing first writer can be visible through Lstat/ReadFile
				// while its exclusive file is still being filled. Retry that
				// transient observation, then fail closed for a genuinely
				// malformed persisted value.
				if attempt < cloneIDMaxRetries {
					if waitErr := ops.wait(ctx, cloneIDRetryDelay); waitErr != nil {
						return "", fmt.Errorf("audit: wait for clone id contents: %w", waitErr)
					}
					continue
				}
				return "", parseErr
			}
			return cloneID, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("audit: read clone id: %w", err)
		}
		cloneID, err := randomCloneID(ops.random)
		if err != nil {
			return "", err
		}
		if ops.beforeCloneCreate != nil {
			if err := ops.beforeCloneCreate(ctx); err != nil {
				return "", fmt.Errorf("audit: wait before clone id creation: %w", err)
			}
		}
		file, openErr := ops.projectOpen(cloneIDPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if openErr == nil {
			if file == nil {
				return "", fmt.Errorf("audit: create clone id returned nil file: %w", ErrInvalidRoot)
			}
			writeErr := writeAndClose(file, []byte(cloneID+"\n"), "clone id", ops.write, ops.close)
			if writeErr != nil {
				if cleanupErr := ops.projectRemove(cloneIDPath); cleanupErr != nil && !errors.Is(cleanupErr, fs.ErrNotExist) {
					writeErr = errors.Join(writeErr, fmt.Errorf("audit: remove incomplete clone id: %w", cleanupErr))
				}
				return "", writeErr
			}
			return cloneID, nil
		}
		if !errors.Is(openErr, fs.ErrExist) {
			return "", fmt.Errorf("audit: create clone id: %w", openErr)
		}
		if attempt == cloneIDMaxRetries {
			return "", fmt.Errorf("audit: clone id creation contention: %w", fs.ErrExist)
		}
		if err := ops.wait(ctx, cloneIDRetryDelay); err != nil {
			return "", fmt.Errorf("audit: wait for clone id: %w", err)
		}
	}
	panic("audit: unreachable clone id loop")
}

func ensureProjectDirectory(ops ledgerOps) error {
	info, err := ops.projectLstat("aidlc")
	if errors.Is(err, fs.ErrNotExist) {
		if err := ops.projectMkdir("aidlc", 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("audit: create aidlc directory: %w", err)
		}
		info, err = ops.projectLstat("aidlc")
	}
	if err != nil {
		return fmt.Errorf("audit: inspect aidlc directory: %w", err)
	}
	if info == nil || info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("audit: aidlc must be a directory: %w", ErrInvalidRoot)
	}
	return nil
}

func parseCloneID(raw []byte) (string, error) {
	if len(raw) > 0 && raw[len(raw)-1] == '\n' {
		raw = raw[:len(raw)-1]
	}
	if len(raw) != cloneIDLength {
		return "", fmt.Errorf("audit: clone id must be %d lowercase hex characters: %w", cloneIDLength, ErrInvalidCloneID)
	}
	for _, char := range string(raw) {
		if !(char >= '0' && char <= '9') && !(char >= 'a' && char <= 'f') {
			return "", fmt.Errorf("audit: clone id is not lowercase hex: %w", ErrInvalidCloneID)
		}
	}
	return string(raw), nil
}

func randomCloneID(random io.Reader) (string, error) {
	if random == nil {
		return "", fmt.Errorf("audit: random source is nil: %w", ErrInvalidRoot)
	}
	var value [cloneIDLength / 2]byte
	if _, err := io.ReadFull(random, value[:]); err != nil {
		return "", fmt.Errorf("audit: generate clone id: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func normalizeHost(host string) string {
	var builder strings.Builder
	lastWasInvalid := false
	for _, char := range strings.ToLower(host) {
		valid := (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-'
		if valid {
			builder.WriteRune(char)
			lastWasInvalid = false
			continue
		}
		if !lastWasInvalid {
			builder.WriteByte('-')
			lastWasInvalid = true
		}
	}
	name := strings.Trim(builder.String(), "-")
	if len(name) > 48 {
		name = strings.TrimRight(name[:48], "-")
	}
	if name == "" {
		return "host"
	}
	return name
}

func shardName(host, cloneID string) string {
	return normalizeHost(host) + "-" + cloneID + ".md"
}

func shardNameForHost(cloneID string) string {
	host, err := os.Hostname()
	if err != nil {
		host = "host"
	}
	return shardName(host, cloneID)
}

func ensureAuditDirectory(ops ledgerOps) error {
	info, err := ops.recordLstat(auditDirectory)
	if errors.Is(err, fs.ErrNotExist) {
		if err := ops.recordMkdir(auditDirectory, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("audit: create audit directory: %w", err)
		}
		info, err = ops.recordLstat(auditDirectory)
	}
	if err != nil {
		return fmt.Errorf("audit: inspect audit directory: %w", err)
	}
	if info == nil || info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("audit: audit directory must be a directory: %w", ErrInvalidRoot)
	}
	return nil
}

func appendRendered(shard string, rendered []renderedEvent, ops ledgerOps) (err error) {
	parentPath := path.Dir(shard)
	parentInfo, parentLstatErr := ops.recordLstat(parentPath)
	if parentLstatErr != nil {
		return fmt.Errorf("audit: inspect audit parent %q: %w", parentPath, parentLstatErr)
	}
	if parentInfo == nil || parentInfo.Mode()&fs.ModeSymlink != 0 || !parentInfo.IsDir() {
		return fmt.Errorf("audit: audit parent %q must be a directory: %w", parentPath, ErrInvalidRoot)
	}
	parent, err := ops.recordOpen(parentPath, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("audit: open audit parent %q: %w", parentPath, err)
	}
	if parent == nil {
		return fmt.Errorf("audit: open audit parent %q returned nil file: %w", parentPath, ErrInvalidRoot)
	}
	defer func() {
		if closeErr := ops.close(parent); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("audit: close audit parent %q: %w", parentPath, closeErr))
		}
	}()
	if err := verifyDirectoryBinding(parentPath, parent, ops, parentInfo); err != nil {
		return err
	}

	info, lstatErr := ops.recordLstat(shard)
	if lstatErr != nil && !errors.Is(lstatErr, fs.ErrNotExist) {
		return fmt.Errorf("audit: inspect shard %q: %w", shard, lstatErr)
	}
	if lstatErr == nil && (info == nil || info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return fmt.Errorf("audit: shard %q must be a regular non-symlink file: %w", shard, ErrInvalidRoot)
	}
	flags := os.O_RDWR | os.O_APPEND | os.O_CREATE
	if errors.Is(lstatErr, fs.ErrNotExist) {
		flags |= os.O_EXCL
	}
	file, err := ops.recordOpen(shard, flags, 0o666)
	if err != nil {
		if flags&os.O_EXCL != 0 && errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("audit: shard %q was created during validation: %w", shard, ErrInvalidRoot)
		}
		return fmt.Errorf("audit: open shard %q: %w", shard, err)
	}
	if file == nil {
		return fmt.Errorf("audit: open shard %q returned nil file: %w", shard, ErrInvalidRoot)
	}
	defer func() {
		if closeErr := ops.close(file); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("audit: close shard %q: %w", shard, closeErr))
		}
	}()
	opened, err := verifyShardBinding(shard, parentPath, file, parent, info, parentInfo, ops)
	if err != nil {
		return err
	}
	var payload strings.Builder
	if opened.Size() == 0 {
		payload.WriteString("# AI-DLC Audit Log\n")
	}
	for _, event := range rendered {
		payload.Write(event.block)
	}
	if _, err := verifyShardBinding(shard, parentPath, file, parent, info, parentInfo, ops); err != nil {
		return err
	}
	if err := writeAll(file, []byte(payload.String()), ops.write); err != nil {
		return fmt.Errorf("audit: write shard %q: %w", shard, err)
	}
	if _, err := verifyShardBinding(shard, parentPath, file, parent, info, parentInfo, ops); err != nil {
		return err
	}
	return nil
}

func verifyDirectoryBinding(parentPath string, parent *os.File, ops ledgerOps, expected fs.FileInfo) error {
	opened, err := parent.Stat()
	if err != nil {
		return fmt.Errorf("audit: stat audit parent %q: %w", parentPath, err)
	}
	if opened == nil || !opened.IsDir() {
		return fmt.Errorf("audit: audit parent %q descriptor is not a directory: %w", parentPath, ErrInvalidRoot)
	}
	pathInfo, err := ops.recordLstat(parentPath)
	if err != nil {
		return fmt.Errorf("audit: verify audit parent %q: %w", parentPath, err)
	}
	if pathInfo == nil || pathInfo.Mode()&fs.ModeSymlink != 0 || !pathInfo.IsDir() || !os.SameFile(pathInfo, opened) || (expected != nil && !os.SameFile(pathInfo, expected)) {
		return fmt.Errorf("audit: audit parent %q changed identity: %w", parentPath, ErrInvalidRoot)
	}
	return nil
}

func verifyShardBinding(shard, parentPath string, file, parent *os.File, expected, expectedParent fs.FileInfo, ops ledgerOps) (fs.FileInfo, error) {
	if err := verifyDirectoryBinding(parentPath, parent, ops, expectedParent); err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("audit: stat shard %q: %w", shard, err)
	}
	if opened == nil || !opened.Mode().IsRegular() {
		return nil, fmt.Errorf("audit: shard %q descriptor is not regular: %w", shard, ErrInvalidRoot)
	}
	pathInfo, err := ops.recordLstat(shard)
	if err != nil {
		return nil, fmt.Errorf("audit: verify shard %q: %w", shard, err)
	}
	if pathInfo == nil || pathInfo.Mode()&fs.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() || !os.SameFile(pathInfo, opened) || (expected != nil && !os.SameFile(pathInfo, expected)) {
		return nil, fmt.Errorf("audit: shard %q changed identity: %w", shard, ErrInvalidRoot)
	}
	return opened, nil
}

func writeAndClose(file *os.File, data []byte, what string, write func(*os.File, []byte) (int, error), close func(*os.File) error) (err error) {
	if err := writeAll(file, data, write); err != nil {
		err = fmt.Errorf("audit: write %s: %w", what, err)
	}
	if closeErr := close(file); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("audit: close %s: %w", what, closeErr))
	}
	return err
}

func writeAll(file *os.File, data []byte, write func(*os.File, []byte) (int, error)) error {
	for len(data) > 0 {
		written, err := write(file, data)
		if written < 0 || written > len(data) {
			return fmt.Errorf("audit: invalid write count %d: %w", written, ErrNoWriteProgress)
		}
		if err != nil {
			return err
		}
		if written == 0 {
			return ErrNoWriteProgress
		}
		data = data[written:]
	}
	return nil
}

func waitForCloneID(ctx context.Context, duration time.Duration) error {
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
