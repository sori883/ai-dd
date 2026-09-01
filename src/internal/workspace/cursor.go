package workspace

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

const cursorTempAttempts = 10

// cursorOps is a private failure-injection seam for one-file replacement.
// Filesystem operations stay bound to the same cursor root.
type cursorOps struct {
	lstat    func(string) (fs.FileInfo, error)
	tempName func() string
	openFile func(string, int, fs.FileMode) (*os.File, error)
	write    func(*os.File, string) (int, error)
	chmod    func(*os.File, fs.FileMode) error
	close    func(*os.File) error
	rename   func(string, string) error
	link     func(string, string) error
	remove   func(string) error
}

func saveCursorInRoot(
	value string,
	cursorName string,
	tempPrefix string,
	parentName string,
	open func() (*os.Root, error),
	closeRoot func(*os.Root) error,
) (err error) {
	root, err := open()
	if err != nil {
		return fmt.Errorf("open cursor parent %q: %w", parentName, err)
	}
	defer func() {
		if closeErr := closeRoot(root); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close cursor parent %q: %w", parentName, closeErr))
		}
	}()
	return replaceCursor(cursorName, value, cursorOperations(root, tempPrefix))
}

func cursorOperations(root *os.Root, tempPrefix string) cursorOps {
	return cursorOps{
		lstat:    root.Lstat,
		tempName: func() string { return tempPrefix + rand.Text() },
		openFile: root.OpenFile,
		write:    (*os.File).WriteString,
		chmod:    (*os.File).Chmod,
		close:    (*os.File).Close,
		rename:   root.Rename,
		link:     root.Link,
		remove:   root.Remove,
	}
}

func completeCursorNoReplace(cursorName, value string, ops cursorOps) (err error) {
	var temp string
	var file *os.File
	for range cursorTempAttempts {
		temp = ops.tempName()
		file, err = ops.openFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if !errors.Is(err, fs.ErrExist) {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("create temporary cursor %q: %w", temp, err)
	}
	defer func() {
		if cleanupErr := ops.remove(temp); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove temporary cursor %q: %w", temp, cleanupErr))
		}
	}()
	if err := writeCursorFile(file, value+"\n", nil, ops); err != nil {
		return err
	}
	if err := ops.link(temp, cursorName); err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("complete %s cursor: %w", cursorName, err)
	}
	return nil
}

func replaceCursor(cursorName, value string, ops cursorOps) (err error) {
	info, err := ops.lstat(cursorName)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect %s cursor: %w", cursorName, err)
	}
	if err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("%s cursor must be a regular file: %w", cursorName, fs.ErrInvalid)
	}
	mode := fs.FileMode(0o666)
	var oldMode *fs.FileMode
	if err == nil {
		mode = 0o600
		permissions := info.Mode().Perm()
		oldMode = &permissions
	}
	var temp string
	var file *os.File
	for range cursorTempAttempts {
		temp = ops.tempName()
		file, err = ops.openFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if !errors.Is(err, fs.ErrExist) {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("create temporary cursor %q: %w", temp, err)
	}
	isRenamed := false
	defer func() {
		if !isRenamed {
			if cleanupErr := ops.remove(temp); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("remove temporary cursor %q: %w", temp, cleanupErr))
			}
		}
	}()
	if err := writeCursorFile(file, value+"\n", oldMode, ops); err != nil {
		return err
	}
	if err := ops.rename(temp, cursorName); err != nil {
		return fmt.Errorf("replace %s cursor: %w", cursorName, err)
	}
	isRenamed = true
	return nil
}

func writeCursorFile(file *os.File, content string, oldMode *fs.FileMode, ops cursorOps) (err error) {
	defer func() {
		if closeErr := ops.close(file); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close temporary cursor: %w", closeErr))
		}
	}()
	n, err := ops.write(file, content)
	if err == nil && n != len(content) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return fmt.Errorf("write temporary cursor: %w", err)
	}
	if oldMode != nil {
		if err := ops.chmod(file, *oldMode); err != nil {
			return fmt.Errorf("restore cursor permissions: %w", err)
		}
	}
	return nil
}
