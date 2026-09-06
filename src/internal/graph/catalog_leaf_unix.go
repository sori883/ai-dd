//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package graph

import (
	"os"
	"syscall"
)

func openCatalogLeaf(projectRoot *os.Root, name string) (*os.File, error) {
	return projectRoot.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
