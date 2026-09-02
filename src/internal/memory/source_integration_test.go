//go:build integration

package memory_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/sori883/ai-dd/src/internal/memory"
)

func TestReadSourcesIntegrationReadsMemoryRoot(t *testing.T) {
	t.Parallel()

	root, memoryDir := openMemoryRoot(t)
	writeMemoryFile(t, memoryDir, "org.md", "org\r\n")
	writeMemoryFile(t, memoryDir, "phases/implementation.md", "phase")

	got, err := memory.ReadSources(root.FS(), "implementation")
	if err != nil {
		t.Fatalf("ReadSources() error = %v, want nil", err)
	}
	want := []memory.Source{
		{Layer: memory.LayerOrg, Path: "org.md", Content: "org\r\n"},
		{Layer: memory.LayerPhase, Path: "phases/implementation.md", Content: "phase"},
	}
	if len(got) != len(want) {
		t.Fatalf("ReadSources() returned %d sources, want %d: %#v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("source %d = %#v, want %#v", index, got[index], want[index])
		}
	}

	file, err := root.Open("org.md")
	if err != nil {
		t.Fatalf("root.Open() after ReadSources() error = %v, want nil", err)
	}
	if err := file.Close(); err != nil {
		t.Errorf("root file Close() error = %v, want nil", err)
	}
}

func TestReadSourcesIntegrationAllowsInRootSymlink(t *testing.T) {
	t.Parallel()

	root, memoryDir := openMemoryRoot(t)
	writeMemoryFile(t, memoryDir, "org-target.md", "in root")
	if err := os.Symlink("org-target.md", filepath.Join(memoryDir, "org.md")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}

	got, err := memory.ReadSources(root.FS(), "implementation")
	if err != nil {
		t.Fatalf("ReadSources() error = %v, want nil", err)
	}
	if len(got) != 1 || got[0] != (memory.Source{
		Layer: memory.LayerOrg, Path: "org.md", Content: "in root",
	}) {
		t.Errorf("ReadSources() = %#v, want in-root symlink content", got)
	}
}

func TestReadSourcesIntegrationRejectsOutwardSymlink(t *testing.T) {
	t.Parallel()

	root, memoryDir := openMemoryRoot(t)
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "secret.md")
	if err := os.WriteFile(outsidePath, []byte("outside secret"), 0o600); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	if err := os.Symlink(outsidePath, filepath.Join(memoryDir, "org.md")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}

	got, err := memory.ReadSources(root.FS(), "implementation")
	if err == nil {
		t.Fatal("ReadSources() error = nil, want outward symlink rejection")
	}
	if got != nil {
		t.Errorf("ReadSources() result = %#v, want nil without external bytes", got)
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadSources() error = %v, want boundary error rather than missing source", err)
	}
	info, err := os.Lstat(filepath.Join(memoryDir, "org.md"))
	if err != nil {
		t.Fatalf("Lstat() outward symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("outward source mode = %v, want symlink retained", info.Mode())
	}
}

func openMemoryRoot(t *testing.T) (*os.Root, string) {
	t.Helper()

	project := t.TempDir()
	memoryDir := filepath.Join(project, "memory")
	if err := os.Mkdir(memoryDir, 0o700); err != nil {
		t.Fatalf("create memory directory: %v", err)
	}
	root, err := os.OpenRoot(memoryDir)
	if err != nil {
		t.Fatalf("open memory root: %v", err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("memory root Close() error = %v", err)
		}
	})
	return root, memoryDir
}

func writeMemoryFile(t *testing.T, memoryDir, name, content string) {
	t.Helper()

	path := filepath.Join(memoryDir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create parent for %q: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %q: %v", name, err)
	}
}
