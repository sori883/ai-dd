//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package audit

import (
	"io/fs"
	"os"
	"syscall"
)

// openAuditParent opens a directory with both a directory-only and a
// nonblocking flag. os.Root.OpenRoot is descriptor-safe after opening, but on
// Unix its initial open does not always include O_DIRECTORY; a FIFO could
// therefore block before Root can reject it.
func openAuditParent(recordRoot *os.Root, name string) (*auditParent, error) {
	if recordRoot == nil || recordRoot.Name() == "" {
		return nil, fs.ErrInvalid
	}
	// Root.OpenFile keeps resolution relative to the caller-owned descriptor;
	// O_DIRECTORY rejects non-directories and O_NONBLOCK prevents a swapped
	// FIFO from stalling this proof path.
	file, err := recordRoot.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, fs.ErrInvalid
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if info == nil || !info.IsDir() {
		_ = file.Close()
		return nil, fs.ErrInvalid
	}
	return &auditParent{
		stat:  file.Stat,
		close: file.Close,
	}, nil
}
