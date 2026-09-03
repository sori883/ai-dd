//go:build !(aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris || windows)

package audit

import "os"

func openAuditParent(recordRoot *os.Root, name string) (*auditParent, error) {
	if recordRoot == nil {
		return nil, os.ErrInvalid
	}
	root, err := recordRoot.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	if root == nil {
		return nil, os.ErrInvalid
	}
	return &auditParent{
		stat:  func() (os.FileInfo, error) { return root.Stat(".") },
		close: root.Close,
	}, nil
}
