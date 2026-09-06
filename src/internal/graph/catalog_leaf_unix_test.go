//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package graph

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestOpenCatalogLeafFIFOWithoutWriterReturns(t *testing.T) {
	if os.Getenv("AIDLC_GRAPH_FIFO_HELPER") == "1" {
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run", "^TestOpenCatalogLeafFIFOHelper$", "-test.v")
	cmd.Env = append(os.Environ(), "AIDLC_GRAPH_FIFO_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start FIFO helper: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("FIFO helper: %v", err)
		}
	case <-time.After(2 * time.Second):
		if err := cmd.Process.Kill(); err != nil {
			t.Fatalf("kill blocked FIFO helper: %v", err)
		}
		<-done
		t.Fatal("openCatalogLeaf blocked on writerless FIFO")
	}
}

func TestOpenCatalogLeafFIFOHelper(t *testing.T) {
	if os.Getenv("AIDLC_GRAPH_FIFO_HELPER") != "1" {
		return
	}
	projectPath := t.TempDir()
	dataPath := filepath.Join(projectPath, ".codex", "tools", "data")
	if err := os.MkdirAll(dataPath, 0o700); err != nil {
		t.Fatalf("MkdirAll(data): %v", err)
	}
	fifoPath := filepath.Join(dataPath, "stage-graph.json")
	if err := syscall.Mkfifo(fifoPath, 0o600); err != nil {
		t.Fatalf("Mkfifo(catalog): %v", err)
	}
	projectRoot, err := os.OpenRoot(projectPath)
	if err != nil {
		t.Fatalf("OpenRoot(project): %v", err)
	}
	defer func() { _ = projectRoot.Close() }()

	file, err := openCatalogLeaf(projectRoot, filepath.ToSlash(filepath.Join(".codex", "tools", "data", "stage-graph.json")))
	if err != nil {
		t.Fatalf("openCatalogLeaf(FIFO): %v", err)
	}
	if file == nil {
		t.Fatal("openCatalogLeaf(FIFO) returned nil file")
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(FIFO): %v", err)
	}
}
