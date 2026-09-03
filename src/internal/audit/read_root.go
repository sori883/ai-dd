package audit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/sori883/ai-dd/src/internal/recordlock"
)

// ReadEvents reads the audit shards belonging to identity from recordRoot.
// It validates the caller-owned roots and Guard before and after every
// descriptor-backed read. The Guard's lease is deliberately not acquired
// here: callers may already be inside the record transaction, while Append
// acquires its own non-reentrant lease at the final write boundary.
func ReadEvents(ctx context.Context, identity recordlock.Identity, guard *recordlock.Guard, projectRoot, recordRoot *os.Root) (records []AuditRecord, err error) {
	if err := checkReadContext(ctx); err != nil {
		return nil, err
	}
	if projectRoot == nil || recordRoot == nil {
		return nil, fmt.Errorf("audit: project and record roots are required: %w", ErrInvalidRoot)
	}
	if err := verifyReadBindings(identity, guard, projectRoot, recordRoot); err != nil {
		return nil, err
	}

	ops := systemLedgerOps(projectRoot, recordRoot)
	auditInfo, err := recordRoot.Lstat(auditDirectory)
	if errors.Is(err, fs.ErrNotExist) {
		if verifyErr := verifyReadBindings(identity, guard, projectRoot, recordRoot); verifyErr != nil {
			return nil, verifyErr
		}
		return []AuditRecord{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("audit: inspect audit directory: %w", err)
	}
	if auditInfo == nil || auditInfo.Mode()&fs.ModeSymlink != 0 || !auditInfo.IsDir() {
		return nil, fmt.Errorf("audit: audit directory must be a directory: %w", ErrInvalidRoot)
	}

	parent, err := openAuditParent(recordRoot, auditDirectory)
	if err != nil {
		return nil, fmt.Errorf("audit: open audit parent %q: %w", auditDirectory, errors.Join(ErrInvalidRoot, err))
	}
	if parent == nil || parent.close == nil {
		return nil, fmt.Errorf("audit: open audit parent %q returned an unclosable handle: %w", auditDirectory, ErrInvalidRoot)
	}
	defer func() {
		if closeErr := parent.close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("audit: close audit parent %q: %w", auditDirectory, closeErr))
		}
	}()
	if err := verifyDirectoryBinding(auditDirectory, parent, ops, auditInfo); err != nil {
		return nil, err
	}
	if err := verifyReadBindings(identity, guard, projectRoot, recordRoot); err != nil {
		return nil, err
	}
	if err := checkReadContext(ctx); err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(recordRoot.FS(), auditDirectory)
	if err != nil {
		return nil, fmt.Errorf("audit: read audit directory: %w", err)
	}
	if err := verifyDirectoryBinding(auditDirectory, parent, ops, auditInfo); err != nil {
		return nil, err
	}
	if err := verifyReadBindings(identity, guard, projectRoot, recordRoot); err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if err := checkReadContext(ctx); err != nil {
			return nil, err
		}
		if err := verifyReadBindings(identity, guard, projectRoot, recordRoot); err != nil {
			return nil, err
		}
		if err := verifyDirectoryBinding(auditDirectory, parent, ops, auditInfo); err != nil {
			return nil, err
		}
		shard := path.Join(auditDirectory, entry.Name())
		content, err := readAuditLeaf(recordRoot, shard)
		if err != nil {
			return nil, err
		}
		if err := verifyReadBindings(identity, guard, projectRoot, recordRoot); err != nil {
			return nil, err
		}
		shardRecords, err := parseAuditShard(shard, content)
		if err != nil {
			return nil, err
		}
		records = append(records, shardRecords...)
		if err := verifyDirectoryBinding(auditDirectory, parent, ops, auditInfo); err != nil {
			return nil, err
		}
		if err := verifyReadBindings(identity, guard, projectRoot, recordRoot); err != nil {
			return nil, err
		}
	}
	if err := verifyDirectoryBinding(auditDirectory, parent, ops, auditInfo); err != nil {
		return nil, err
	}
	if err := verifyReadBindings(identity, guard, projectRoot, recordRoot); err != nil {
		return nil, err
	}
	return records, nil
}

// ValidateRecordBinding validates only the identity, Guard, and supplied root
// bindings. It does not inspect or parse the audit ledger and never closes a
// caller-owned root.
func ValidateRecordBinding(ctx context.Context, identity recordlock.Identity, guard *recordlock.Guard, projectRoot, recordRoot *os.Root) error {
	if err := checkReadContext(ctx); err != nil {
		return err
	}
	if projectRoot == nil || recordRoot == nil {
		return fmt.Errorf("audit: project and record roots are required: %w", ErrInvalidRoot)
	}
	return verifyReadBindings(identity, guard, projectRoot, recordRoot)
}

func verifyReadBindings(identity recordlock.Identity, guard *recordlock.Guard, projectRoot, recordRoot *os.Root) error {
	if guard == nil || !guard.Held() {
		return ErrGuardNotHeld
	}
	if guard.Identity() != identity {
		return fmt.Errorf("audit: guard identity differs from requested record: %w", ErrGuardIdentity)
	}
	return verifyRootBindings(identity, projectRoot, recordRoot)
}

func checkReadContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("audit: read nil context: %w", fs.ErrInvalid)
	}
	if err := context.Cause(ctx); err != nil {
		return fmt.Errorf("audit: read: %w", err)
	}
	return nil
}

func readAuditLeaf(recordRoot *os.Root, shard string) ([]byte, error) {
	if recordRoot == nil {
		return nil, fmt.Errorf("audit: record root is required: %w", ErrInvalidRoot)
	}
	pathInfo, err := recordRoot.Lstat(shard)
	if err != nil {
		return nil, fmt.Errorf("audit: inspect shard %q: %w", shard, err)
	}
	if pathInfo == nil || pathInfo.Mode()&fs.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("audit: shard %q must be a regular non-symlink file: %w", shard, ErrInvalidRoot)
	}

	file, err := openAuditLeaf(recordRoot, shard)
	if err != nil {
		return nil, fmt.Errorf("audit: open shard %q: %w", shard, err)
	}
	if file == nil {
		return nil, fmt.Errorf("audit: open shard %q returned nil file: %w", shard, ErrInvalidRoot)
	}
	var result []byte
	defer func() {
		_ = file.Close()
	}()
	opened, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("audit: stat shard %q: %w", shard, err)
	}
	if opened == nil || !opened.Mode().IsRegular() || !os.SameFile(pathInfo, opened) {
		return nil, fmt.Errorf("audit: shard %q changed identity: %w", shard, ErrInvalidRoot)
	}
	result, err = io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("audit: read shard %q: %w", shard, err)
	}
	final, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("audit: stat shard %q after read: %w", shard, err)
	}
	current, err := recordRoot.Lstat(shard)
	if err != nil {
		return nil, fmt.Errorf("audit: verify shard %q: %w", shard, err)
	}
	if final == nil || current == nil || !final.Mode().IsRegular() || current.Mode()&fs.ModeSymlink != 0 || !current.Mode().IsRegular() || !os.SameFile(final, current) || !os.SameFile(pathInfo, current) {
		return nil, fmt.Errorf("audit: shard %q changed identity: %w", shard, ErrInvalidRoot)
	}
	return result, nil
}
