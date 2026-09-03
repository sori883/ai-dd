//go:build !(aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris || windows)

package recordlock

import "os"

func openLockRoot(path string) (*os.Root, error) {
	base, relative, err := lockRootLocation(path)
	if err != nil {
		return nil, err
	}
	baseRoot, err := os.OpenRoot(base)
	if err != nil {
		return nil, err
	}
	root, err := baseRoot.OpenRoot(relative)
	closeErr := baseRoot.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		_ = root.Close()
		return nil, closeErr
	}
	return root, nil
}

func openLockOwner(root *os.Root, name string) (*os.File, error) {
	if root == nil {
		return nil, os.ErrInvalid
	}
	return root.Open(name)
}
