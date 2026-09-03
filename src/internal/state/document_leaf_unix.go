//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package state

import (
	"os"
	"syscall"
)

func openStateLeaf(recordRoot *os.Root, name string) (*os.File, error) {
	if recordRoot == nil {
		return nil, os.ErrInvalid
	}
	return recordRoot.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
