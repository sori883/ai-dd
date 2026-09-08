// Package filestore provides bounded, rooted file replacement and local locks.
package filestore

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const MaxBytes = 256 * 1024

func invalid(s string) error { return fmt.Errorf("%s: %w", s, fs.ErrInvalid) }

// ReadFile rejects symlinks and non-regular files within the supplied root.
func ReadFile(root, name string) ([]byte, error) {
	directory, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	if err := checkPath(directory, name, false); err != nil {
		return nil, err
	}
	file, err := directory.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not regular: %w", fs.ErrInvalid)
	}
	raw, err := io.ReadAll(io.LimitReader(file, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxBytes {
		return nil, fmt.Errorf("file exceeds 256 KiB: %w", fs.ErrInvalid)
	}
	return raw, nil
}
func checkPath(root *os.Root, name string, missing bool) error {
	if !fs.ValidPath(filepath.ToSlash(name)) || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid relative path: %w", fs.ErrInvalid)
	}
	parts := strings.Split(filepath.ToSlash(name), "/")
	for i := range parts {
		info, err := root.Lstat(filepath.Join(parts[:i+1]...))
		if missing && os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink path: %w", fs.ErrInvalid)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return fmt.Errorf("non-directory parent: %w", fs.ErrInvalid)
		}
	}
	return nil
}

// WriteFile replaces one file via a sibling temporary file. Callers hold their
// bundle lock; it does not promise a multi-file transaction.
func WriteFile(root, name string, raw []byte) error {
	directory, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer directory.Close()
	if err := checkPath(directory, name, true); err != nil {
		return err
	}
	if err := directory.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return err
	}
	temp := filepath.Join(filepath.Dir(name), fmt.Sprintf(".aidlc-%x", randomBytes()))
	file, err := directory.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer directory.Remove(temp)
	_, writeErr := file.Write(raw)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	return directory.Rename(temp, name)
}
func randomBytes() []byte { raw := make([]byte, 16); _, _ = rand.Read(raw); return raw }

// Lock acquires a local exclusive lock without stealing an existing owner.
func Lock(project, key string) (func() error, error) {
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(key) {
		return nil, invalid("invalid lock key")
	}
	root, err := os.OpenRoot(project)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"aidlc", "aidlc/.runtime", "aidlc/.runtime/locks"} {
		info, err := root.Lstat(name)
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			root.Close()
			return nil, invalid("runtime path is not a directory")
		}
		if err != nil && !os.IsNotExist(err) {
			root.Close()
			return nil, err
		}
		if os.IsNotExist(err) {
			if err := root.Mkdir(name, 0700); err != nil && !os.IsExist(err) {
				root.Close()
				return nil, err
			}
		}
	}
	ignore, err := root.OpenFile("aidlc/.runtime/.gitignore", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		_, writeErr := ignore.WriteString("*\n")
		closeErr := ignore.Close()
		if writeErr != nil {
			root.Close()
			return nil, writeErr
		}
		if closeErr != nil {
			root.Close()
			return nil, closeErr
		}
	} else if !os.IsExist(err) {
		root.Close()
		return nil, err
	}
	name := "aidlc/.runtime/locks/" + key
	if err := root.Mkdir(name, 0700); err != nil {
		root.Close()
		return nil, fmt.Errorf("lock %s unavailable; inspect active process before recovery: %w", key, err)
	}
	return func() error {
		err := root.Remove(name)
		closeErr := root.Close()
		if err != nil {
			return err
		}
		return closeErr
	}, nil
}

// Hash identifies exact persisted bytes.
func Hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
