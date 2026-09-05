//go:build !(aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package delivery

import "os"

func openContextReadFile(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY, 0)
}
