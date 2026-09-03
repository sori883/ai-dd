//go:build windows

package state

import "os"

func openStateLeaf(recordRoot *os.Root, name string) (*os.File, error) {
	if recordRoot == nil {
		return nil, os.ErrInvalid
	}
	return recordRoot.OpenFile(name, os.O_RDONLY, 0)
}
