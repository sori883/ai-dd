package workspace

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

const registryTempAttempts = 10

func decodeIntentRegistryForMutation(data []byte) ([]json.RawMessage, error) {
	rows, err := parseIntentRegistryDocument(data, true)
	if err != nil {
		return nil, err
	}
	for index, row := range rows {
		if _, err := decodeRegistryIntentRow(row, index); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

func parseIntentRegistryDocument(data []byte, strict bool) ([]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		if strict {
			return nil, fmt.Errorf("intent registry must be a JSON array: %w", fs.ErrInvalid)
		}
		return []json.RawMessage{}, nil
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(data, &rows); err != nil || rows == nil {
		if strict {
			return nil, fmt.Errorf("decode intent registry: %w", fs.ErrInvalid)
		}
		return []json.RawMessage{}, nil
	}
	return rows, nil
}

type intentRegistryEntry struct {
	UUID    string   `json:"uuid"`
	Slug    string   `json:"slug"`
	DirName string   `json:"dirName"`
	Scope   *string  `json:"scope,omitempty"`
	Repos   []string `json:"repos,omitempty"`
	Status  string   `json:"status"`
}

type registryReadOps struct {
	lstat    func(string) (fs.FileInfo, error)
	readFile func(string) ([]byte, error)
}

func registryReadOperations(root *os.Root) registryReadOps {
	return registryReadOps{
		lstat: root.Lstat,
		readFile: func(name string) ([]byte, error) {
			return fs.ReadFile(root.FS(), name)
		},
	}
}

func readIntentRegistryForMutation(ops registryReadOps) ([]json.RawMessage, error) {
	const name = "intents.json"
	info, err := ops.lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return []json.RawMessage{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect intent registry: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("intent registry must be a regular file: %w", fs.ErrInvalid)
	}
	data, err := ops.readFile(name)
	if err != nil {
		return nil, fmt.Errorf("read intent registry: %w", err)
	}
	return decodeIntentRegistryForMutation(data)
}

type registryWriteOps struct {
	tempName func() string
	openFile func(string, int, fs.FileMode) (*os.File, error)
	write    func(*os.File, []byte) (int, error)
	close    func(*os.File) error
	rename   func(string, string) error
	remove   func(string) error
}

func registryWriteOperations(root *os.Root) registryWriteOps {
	return registryWriteOps{
		tempName: func() string { return ".intents-" + rand.Text() + ".tmp" },
		openFile: root.OpenFile,
		write:    (*os.File).Write,
		close:    (*os.File).Close,
		rename:   root.Rename,
		remove:   root.Remove,
	}
}

func writeIntentRegistry(
	rows []json.RawMessage,
	entry intentRegistryEntry,
	ops registryWriteOps,
) (err error) {
	updated := make([]any, 0, len(rows)+1)
	for _, row := range rows {
		updated = append(updated, row)
	}
	updated = append(updated, entry)
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(updated); err != nil {
		return fmt.Errorf("encode intent registry: %w", err)
	}
	data := encoded.Bytes()

	var temp string
	var file *os.File
	for range registryTempAttempts {
		temp = ops.tempName()
		file, err = ops.openFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if !errors.Is(err, fs.ErrExist) {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("create temporary intent registry %q: %w", temp, err)
	}
	renamed := false
	defer func() {
		if renamed {
			return
		}
		if cleanupErr := ops.remove(temp); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove temporary intent registry %q: %w", temp, cleanupErr))
		}
	}()

	n, writeErr := ops.write(file, data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr != nil {
		err = fmt.Errorf("write temporary intent registry %q: %w", temp, writeErr)
	}
	if closeErr := ops.close(file); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("close temporary intent registry %q: %w", temp, closeErr))
	}
	if err != nil {
		return err
	}
	if err := ops.rename(temp, "intents.json"); err != nil {
		return fmt.Errorf("replace intent registry: %w", err)
	}
	renamed = true
	return nil
}
