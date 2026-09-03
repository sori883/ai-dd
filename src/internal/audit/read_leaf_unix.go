//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package audit

import (
	"os"
	"syscall"
)

func openAuditLeaf(recordRoot *os.Root, name string) (*os.File, error) {
	if recordRoot == nil {
		return nil, os.ErrInvalid
	}
	// A regular-file Lstat is checked before this call. O_NONBLOCK keeps a
	// concurrent replacement by a FIFO from turning the reader into an
	// unbounded wait before the descriptor identity proof completes.
	return recordRoot.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
