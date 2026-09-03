//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package recordlock

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// openLockRoot performs a nonblocking directory-only probe from a trusted
// temporary-directory Root before creating the descriptor-relative Root used
// for owner operations. Release normally never calls this function: the
// resulting Root is pinned in Guard during acquisition.
func openLockRoot(path string) (*os.Root, error) {
	base, relative, err := lockRootLocation(path)
	if err != nil {
		return nil, err
	}
	baseRoot, err := os.OpenRoot(base)
	if err != nil {
		return nil, err
	}
	defer func() { _ = baseRoot.Close() }()
	probe, err := baseRoot.OpenFile(relative, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NONBLOCK, 0)
	if err != nil {
		if info, lstatErr := os.Lstat(path); lstatErr == nil && (info == nil || info.Mode()&fs.ModeSymlink != 0 || !info.IsDir()) {
			return nil, fmt.Errorf("recordlock: lock root %q is not a directory: %w", path, ErrOwnerMismatch)
		}
		return nil, err
	}
	if probe == nil {
		return nil, fmt.Errorf("recordlock: open lock root %q returned nil file: %w", path, ErrOwnerMismatch)
	}
	probeInfo, statErr := probe.Stat()
	probeCloseErr := probe.Close()
	if statErr != nil {
		return nil, fmt.Errorf("recordlock: stat lock root probe %q: %w", path, statErr)
	}
	if probeCloseErr != nil {
		return nil, fmt.Errorf("recordlock: close lock root probe %q: %w", path, probeCloseErr)
	}
	if probeInfo == nil || !probeInfo.IsDir() {
		return nil, fmt.Errorf("recordlock: lock root %q is not a directory: %w", path, ErrOwnerMismatch)
	}

	root, err := baseRoot.OpenRoot(relative)
	if err != nil {
		return nil, err
	}
	rootInfo, err := root.Stat(".")
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	if rootInfo == nil || !rootInfo.IsDir() || !os.SameFile(probeInfo, rootInfo) {
		_ = root.Close()
		return nil, fmt.Errorf("recordlock: lock root %q changed during open: %w", path, ErrOwnerMismatch)
	}
	return root, nil
}

// openLockOwner uses Root-relative resolution while preserving nonblocking
// behavior for a marker that is replaced by a FIFO after Lstat.
func openLockOwner(root *os.Root, name string) (*os.File, error) {
	if root == nil {
		return nil, fmt.Errorf("recordlock: owner root is nil: %w", ErrOwnerMismatch)
	}
	return root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}

func openLockOwnerCreate(root *os.Root, name string) (*os.File, error) {
	if root == nil {
		return nil, fmt.Errorf("recordlock: owner root is nil: %w", ErrOwnerMismatch)
	}
	return root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NONBLOCK, 0o600)
}
