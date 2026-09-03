//go:build windows

package audit

import "os"

func openAuditLeaf(recordRoot *os.Root, name string) (*os.File, error) {
	if recordRoot == nil {
		return nil, os.ErrInvalid
	}
	return recordRoot.OpenFile(name, os.O_RDONLY, 0)
}
