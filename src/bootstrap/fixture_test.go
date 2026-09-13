package bootstrap

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

func unsafeInstaller(t *testing.T, binary []byte, mode string) []byte {
	t.Helper()
	var b bytes.Buffer
	g := gzip.NewWriter(&b)
	w := tar.NewWriter(g)
	for i := 0; i < 2; i++ {
		h := &tar.Header{Name: "aidlc-install", Typeflag: tar.TypeReg, Mode: 0755, Size: int64(len(binary))}
		if mode == "symlink" {
			h.Typeflag = tar.TypeSymlink
			h.Linkname = "outside"
			h.Size = 0
		}
		if err := w.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Size > 0 {
			w.Write(binary)
		}
		if mode == "symlink" {
			break
		}
	}
	w.Close()
	g.Close()
	return b.Bytes()
}
