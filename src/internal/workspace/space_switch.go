package workspace

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

const cursorTempAttempts = 10

// cursorOps is a private failure-injection seam for this one-file replacement.
// Filesystem operations stay bound to the same aidlc root.
type cursorOps struct {
	lstat    func(string) (fs.FileInfo, error)
	tempName func() string
	openFile func(string, int, fs.FileMode) (*os.File, error)
	write    func(*os.File, string) (int, error)
	chmod    func(*os.File, fs.FileMode) error
	close    func(*os.File) error
	rename   func(string, string) error
	remove   func(string) error
}

// SwitchSpace selects a listed space by updating the shared active-space cursor.
// The project must already exist; the synthetic default space need not exist.
// Raw empty/help/-h names are invalid. Other names use the creation slug rules,
// but listed names reserved for creation can be selected.
//
// The cursor is replaced within os.Root boundaries; existing symlink or
// nonregular cursors are rejected. Only existing permission bits are preserved.
// An error returns an empty name, but the cursor may already have changed;
// callers must not interpret failure as a rollback or an atomicity guarantee.
func SwitchSpace(input RootInput, rawName string) (string, error) {
	return switchSpace(
		input,
		rawName,
		os.OpenRoot,
		(*os.Root).Close,
		saveSpaceCursor,
	)
}

func switchSpace(
	input RootInput,
	rawName string,
	openProject func(string) (*os.Root, error),
	closeProject func(*os.Root) error,
	save func(*os.Root, string) error,
) (name string, err error) {
	switch rawName {
	case "", "help", "-h":
		return "", fmt.Errorf("invalid space name %q: %w", rawName, fs.ErrInvalid)
	}
	name = spaceSlug(rawName)
	projectPath := ResolveRoot(input)
	if !filepath.IsAbs(projectPath) {
		return "", fmt.Errorf("resolve project root %q: %w", projectPath, fs.ErrInvalid)
	}
	projectRoot, err := openProject(projectPath)
	if err != nil {
		return "", fmt.Errorf("open project root %q: %w", projectPath, err)
	}
	defer func() {
		if closeErr := closeProject(projectRoot); closeErr != nil {
			name = ""
			err = errors.Join(err, fmt.Errorf("close project root %q: %w", projectPath, closeErr))
		}
	}()
	spaces := ListSpaces(projectRoot.FS(), &name)
	if !slices.ContainsFunc(spaces, func(space Space) bool { return space.Name == name }) {
		return "", fmt.Errorf("unknown space %q: not in the space list", name)
	}
	if err := save(projectRoot, name); err != nil {
		return "", err
	}
	return name, nil
}

func saveSpaceCursor(root *os.Root, name string) error {
	// Match the reference's mkdir default; the process umask controls permissions.
	if err := root.MkdirAll("aidlc", 0o777); err != nil {
		return fmt.Errorf("create cursor parent %q: %w", "aidlc", err)
	}
	return saveCursorInRoot(
		name,
		func() (*os.Root, error) { return root.OpenRoot("aidlc") },
		(*os.Root).Close,
	)
}

func saveCursorInRoot(
	name string,
	open func() (*os.Root, error),
	closeRoot func(*os.Root) error,
) (err error) {
	root, err := open()
	if err != nil {
		return fmt.Errorf("open cursor parent %q: %w", "aidlc", err)
	}
	defer func() {
		if closeErr := closeRoot(root); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close cursor parent %q: %w", "aidlc", closeErr))
		}
	}()
	return replaceSpaceCursor(name, cursorOperations(root))
}

func cursorOperations(root *os.Root) cursorOps {
	return cursorOps{
		lstat:    root.Lstat,
		tempName: func() string { return ".active-space-" + rand.Text() },
		openFile: root.OpenFile,
		write:    (*os.File).WriteString,
		chmod:    (*os.File).Chmod,
		close:    (*os.File).Close,
		rename:   root.Rename,
		remove:   root.Remove,
	}
}

func replaceSpaceCursor(name string, ops cursorOps) (err error) {
	info, err := ops.lstat("active-space")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect active-space cursor: %w", err)
	}
	if err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("active-space cursor must be a regular file: %w", fs.ErrInvalid)
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
	if err := writeCursorFile(
		file,
		name+"\n",
		oldMode,
		ops,
	); err != nil {
		return err
	}
	if err := ops.rename(temp, "active-space"); err != nil {
		return fmt.Errorf("replace active-space cursor: %w", err)
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
