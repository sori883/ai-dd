//go:build !(aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris || windows)

package graph

import "os"

func openCatalogLeaf(projectRoot *os.Root, name string) (*os.File, error) {
	return projectRoot.OpenFile(name, os.O_RDONLY, 0)
}
