package steering

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"unicode/utf8"
)

// ErrInvalidContinuationKeyFile indicates an unusable continuation key file.
var ErrInvalidContinuationKeyFile = errors.New("steering: invalid continuation key file")

const continuationKeyFileMaxSize = 4 << 10

type continuationKeyFile interface {
	io.Reader
	io.Writer
	Stat() (fs.FileInfo, error)
	Close() error
}

type continuationKeyOps struct {
	random   io.Reader
	lstat    func(*os.Root, string) (fs.FileInfo, error)
	mkdirAll func(*os.Root, string, fs.FileMode) error
	openFile func(*os.Root, string, int, fs.FileMode) (continuationKeyFile, error)
}

func continuationKeyOperations() continuationKeyOps {
	return continuationKeyOps{
		random: rand.Reader,
		lstat: func(root *os.Root, name string) (fs.FileInfo, error) {
			return root.Lstat(name)
		},
		mkdirAll: func(root *os.Root, name string, perm fs.FileMode) error {
			return root.MkdirAll(name, perm)
		},
		openFile: func(root *os.Root, name string, flag int, perm fs.FileMode) (continuationKeyFile, error) {
			return root.OpenFile(name, flag, perm)
		},
	}
}

// ReadOrCreateContinuationKey reads or creates the private continuation key
// from the active record when its state exists or from the project session area
// otherwise. It reads the selected location on every call and does not close
// caller-owned roots.
func ReadOrCreateContinuationKey(projectRoot *os.Root, recordRoot *os.Root) ([]byte, error) {
	return readOrCreateContinuationKey(projectRoot, recordRoot, continuationKeyOperations())
}

func readOrCreateContinuationKey(projectRoot *os.Root, recordRoot *os.Root, ops continuationKeyOps) ([]byte, error) {
	keyRoot, keyPath, err := continuationKeyLocation(projectRoot, recordRoot, ops, true)
	if err != nil {
		return nil, err
	}

	if key, found, err := readContinuationKey(keyRoot, keyPath, ops); err != nil {
		return nil, err
	} else if found {
		return key, nil
	}
	return createContinuationKey(keyRoot, keyPath, ops)
}

// ReadContinuationKey reads the existing private continuation key without
// creating a session directory or changing the key file. It uses the same
// bounded, canonical file contract as ReadOrCreateContinuationKey.
func ReadContinuationKey(projectRoot *os.Root, recordRoot *os.Root) ([]byte, error) {
	return readContinuationKeyOnly(projectRoot, recordRoot, continuationKeyOperations())
}

func readContinuationKeyOnly(projectRoot *os.Root, recordRoot *os.Root, ops continuationKeyOps) ([]byte, error) {
	keyRoot, keyPath, err := continuationKeyLocation(projectRoot, recordRoot, ops, false)
	if err != nil {
		return nil, err
	}
	key, found, err := readContinuationKey(keyRoot, keyPath, ops)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("continuation key file is absent: %w", ErrInvalidContinuationKeyFile)
	}
	return key, nil
}

func continuationKeyLocation(projectRoot *os.Root, recordRoot *os.Root, ops continuationKeyOps, createSessionDirectory bool) (*os.Root, string, error) {
	if projectRoot == nil {
		return nil, "", fmt.Errorf("project root is required: %w", ErrInvalidContinuationKeyFile)
	}

	keyRoot := projectRoot
	keyPath := "aidlc/.aidlc-sessions/.aidlc-steering-token-key"
	if recordRoot != nil {
		stateInfo, err := ops.lstat(recordRoot, "aidlc-state.md")
		switch {
		case err == nil:
			if stateInfo == nil || !stateInfo.Mode().IsRegular() {
				return nil, "", fmt.Errorf("record state is not regular: %w", ErrInvalidContinuationKeyFile)
			}
			keyRoot = recordRoot
			keyPath = ".aidlc-steering-token-key"
		case errors.Is(err, fs.ErrNotExist):
			if createSessionDirectory {
				if err := ops.mkdirAll(projectRoot, "aidlc/.aidlc-sessions", 0o777); err != nil {
					return nil, "", fmt.Errorf("create session directory: %w", err)
				}
			}
		default:
			return nil, "", fmt.Errorf("inspect record state: %w: %w", ErrInvalidContinuationKeyFile, err)
		}
	} else if createSessionDirectory {
		if err := ops.mkdirAll(projectRoot, "aidlc/.aidlc-sessions", 0o777); err != nil {
			return nil, "", fmt.Errorf("create session directory: %w", err)
		}
	}
	return keyRoot, keyPath, nil
}

func readContinuationKey(root *os.Root, path string, ops continuationKeyOps) ([]byte, bool, error) {
	pathInfo, err := ops.lstat(root, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, true, fmt.Errorf("inspect continuation key file: %w: %w", ErrInvalidContinuationKeyFile, err)
	}
	if pathInfo == nil || pathInfo.Mode()&fs.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return nil, true, fmt.Errorf("continuation key file is not a regular non-symlink: %w", ErrInvalidContinuationKeyFile)
	}

	keyFile, err := ops.openFile(root, path, os.O_RDONLY, 0)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, true, fmt.Errorf("open continuation key file: %w: %w", ErrInvalidContinuationKeyFile, err)
	}

	fileInfo, err := keyFile.Stat()
	if err != nil {
		_ = keyFile.Close()
		return nil, true, fmt.Errorf("stat continuation key file: %w: %w", ErrInvalidContinuationKeyFile, err)
	}
	if fileInfo == nil || !fileInfo.Mode().IsRegular() {
		_ = keyFile.Close()
		return nil, true, fmt.Errorf("continuation key file is not regular: %w", ErrInvalidContinuationKeyFile)
	}

	data, readErr := io.ReadAll(io.LimitReader(keyFile, continuationKeyFileMaxSize+1))
	closeErr := keyFile.Close()
	if readErr != nil {
		return nil, true, fmt.Errorf("read continuation key file: %w: %w", ErrInvalidContinuationKeyFile, readErr)
	}
	if closeErr != nil {
		return nil, true, fmt.Errorf("close continuation key file: %w: %w", ErrInvalidContinuationKeyFile, closeErr)
	}
	if len(data) > continuationKeyFileMaxSize {
		return nil, true, fmt.Errorf("continuation key file is too large: %w", ErrInvalidContinuationKeyFile)
	}
	if !utf8.Valid(data) {
		return nil, true, fmt.Errorf("continuation key file is not valid UTF-8: %w", ErrInvalidContinuationKeyFile)
	}

	canonicalText := strings.TrimFunc(string(data), isECMAScriptWhitespace)
	key, err := base64.RawURLEncoding.DecodeString(canonicalText)
	if err != nil {
		return nil, true, fmt.Errorf("decode continuation key file: %w: %w", ErrInvalidContinuationKeyFile, err)
	}
	if len(key) != 32 || base64.RawURLEncoding.EncodeToString(key) != canonicalText {
		return nil, true, fmt.Errorf("continuation key file is not canonical: %w", ErrInvalidContinuationKeyFile)
	}
	return key, true, nil
}

func createContinuationKey(root *os.Root, path string, ops continuationKeyOps) ([]byte, error) {
	var key [32]byte
	if _, err := io.ReadFull(ops.random, key[:]); err != nil {
		return nil, fmt.Errorf("read continuation key randomness: %w", err)
	}
	keyFile, err := ops.openFile(root, path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			winner, found, readErr := readContinuationKey(root, path, ops)
			if readErr != nil {
				return nil, readErr
			}
			if found {
				return winner, nil
			}
			return nil, fmt.Errorf("concurrent continuation key file disappeared: %w", ErrInvalidContinuationKeyFile)
		}
		return nil, fmt.Errorf("create continuation key file: %w", err)
	}
	keyText := []byte(base64.RawURLEncoding.EncodeToString(key[:]) + "\n")
	if err := writeContinuationKeyFile(keyFile, keyText); err != nil {
		_ = keyFile.Close()
		return nil, fmt.Errorf("write continuation key file: %w", err)
	}
	if err := keyFile.Close(); err != nil {
		return nil, fmt.Errorf("close continuation key file: %w", err)
	}
	return key[:], nil
}

func writeContinuationKeyFile(file continuationKeyFile, data []byte) error {
	for len(data) > 0 {
		written, err := file.Write(data)
		if written > 0 {
			data = data[written:]
		}
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
