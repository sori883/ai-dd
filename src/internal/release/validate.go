package release

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"strings"
)

const MaxArchiveBytes int64 = 512 << 20
const MaxSourceBytes int64 = 32 << 20

func StrictJSON(raw []byte, value any) error {
	// Reject duplicate object keys as well as unknown fields and trailing values.
	d := json.NewDecoder(bytes.NewReader(raw))
	var scan func(int) error
	scan = func(depth int) error {
		if depth > 64 {
			return fmt.Errorf("JSON nesting too deep")
		}
		t, e := d.Token()
		if e != nil {
			return e
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					key, e := d.Token()
					if e != nil {
						return e
					}
					k, ok := key.(string)
					if !ok || seen[k] {
						return fmt.Errorf("duplicate/invalid JSON key")
					}
					seen[k] = true
					if e := scan(depth + 1); e != nil {
						return e
					}
				}
			case '[':
				for d.More() {
					if e := scan(depth + 1); e != nil {
						return e
					}
				}
			default:
				return fmt.Errorf("unexpected delimiter")
			}
			_, e = d.Token()
			return e
		}
		return nil
	}
	if err := scan(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(value)
}
func ValidHash(s string) bool { return regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(s) }
func Unpack(raw []byte, windows bool, maxBytes int64) (map[string][]byte, error) {
	entries := map[string][]byte{}
	var total int64
	add := func(name string, size int64, mode fs.FileMode, r io.Reader) error {
		if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") || !mode.IsRegular() || size < 0 || size > maxBytes-total {
			return fmt.Errorf("unsafe or oversized archive member %s", name)
		}
		if _, ok := entries[name]; ok {
			return fmt.Errorf("duplicate archive member")
		}
		data, err := io.ReadAll(io.LimitReader(r, size+1))
		if err != nil {
			return err
		}
		if int64(len(data)) != size {
			return fmt.Errorf("archive member size mismatch")
		}
		total += size
		entries[name] = data
		return nil
	}
	if windows {
		z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			return nil, err
		}
		for _, file := range z.File {
			if file.UncompressedSize64 > uint64(maxBytes) {
				return nil, fmt.Errorf("oversized zip entry")
			}
			r, err := file.Open()
			if err != nil {
				return nil, err
			}
			err = add(file.Name, int64(file.UncompressedSize64), file.Mode(), r)
			closeErr := r.Close()
			if err != nil {
				return nil, err
			}
			if closeErr != nil {
				return nil, closeErr
			}
		}
	} else {
		g, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer g.Close()
		t := tar.NewReader(g)
		for {
			h, err := t.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
				return nil, fmt.Errorf("nonregular tar entry")
			}
			if err := add(h.Name, h.Size, h.FileInfo().Mode(), t); err != nil {
				return nil, err
			}
		}
		if _, err := io.Copy(io.Discard, io.LimitReader(g, 1)); err != nil {
			return nil, err
		}
	}
	return entries, nil
}
