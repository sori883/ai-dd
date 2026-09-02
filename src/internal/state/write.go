package state

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

const (
	stateFile                = "aidlc-state.md"
	initialWriteTempAttempts = 10
)

// initialWriteOps is a private failure-injection seam for the two-file
// initial-state writer. All filesystem operations remain bound to one root.
type initialWriteOps struct {
	tempName func(string) string
	lstat    func(string) (fs.FileInfo, error)
	openFile func(string, int, fs.FileMode) (*os.File, error)
	write    func(*os.File, []byte) (int, error)
	close    func(*os.File) error
	rename   func(string, string) error
	remove   func(string) error
}

// WriteInitial persists the initial state files beneath recordRoot.
func WriteInitial(recordRoot *os.Root, initial Initial) error {
	if recordRoot == nil {
		return errors.New("write initial: record root is nil")
	}
	return writeInitialWithOps(initial, initialWriteOperations(recordRoot))
}

func initialWriteOperations(root *os.Root) initialWriteOps {
	return initialWriteOps{
		tempName: func(target string) string { return "." + target + "-" + rand.Text() + ".tmp" },
		lstat:    root.Lstat,
		openFile: root.OpenFile,
		write:    (*os.File).Write,
		close:    (*os.File).Close,
		rename:   root.Rename,
		remove:   root.Remove,
	}
}

func writeInitialWithOps(initial Initial, ops initialWriteOps) error {
	if err := writeInitialFile(projectDescriptionFile, []byte(initial.ProjectDescriptionJSON), ops); err != nil {
		return fmt.Errorf("write initial project description: %w", err)
	}
	if err := checkStateWriteBarrier(ops); err != nil {
		return err
	}
	if err := writeInitialFile(stateFile, []byte(initial.StateContent), ops); err != nil {
		return fmt.Errorf("write initial state: %w", err)
	}
	return nil
}

func checkStateWriteBarrier(ops initialWriteOps) error {
	info, err := ops.lstat(stateFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect initial state: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("initial state must be a regular file: %w", fs.ErrInvalid)
	}
	file, err := ops.openFile(stateFile, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open initial state write barrier: %w", err)
	}
	if err := ops.close(file); err != nil {
		return fmt.Errorf("close initial state write barrier: %w", err)
	}
	return nil
}

func writeInitialFile(target string, data []byte, ops initialWriteOps) (err error) {
	var temp string
	var file *os.File
	for range initialWriteTempAttempts {
		temp = ops.tempName(target)
		file, err = ops.openFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if !errors.Is(err, fs.ErrExist) {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("create temporary %s %q: %w", target, temp, err)
	}
	renamed := false
	defer func() {
		if renamed {
			return
		}
		if cleanupErr := ops.remove(temp); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove temporary %s %q: %w", target, temp, cleanupErr))
		}
	}()

	n, writeErr := ops.write(file, data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr != nil {
		err = fmt.Errorf("write temporary %s %q: %w", target, temp, writeErr)
	}
	if closeErr := ops.close(file); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("close temporary %s %q: %w", target, temp, closeErr))
	}
	if err != nil {
		return err
	}
	if err := ops.rename(temp, target); err != nil {
		return fmt.Errorf("replace %s: %w", target, err)
	}
	renamed = true
	return nil
}
