//go:build integration

package knowledge_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/knowledge"
)

func TestBuildRosterIntegrationReadsBorrowedRootsAndFreshEdits(t *testing.T) {
	project := t.TempDir()
	frameworkDir := filepath.Join(project, ".codex")
	if err := os.MkdirAll(filepath.Join(frameworkDir, "agents"), 0o700); err != nil {
		t.Fatalf("create agents directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(frameworkDir, "knowledge", "aidlc-shared"), 0o700); err != nil {
		t.Fatalf("create shared knowledge directory: %v", err)
	}
	writeKnowledgeFile(t, filepath.Join(frameworkDir, "agents", "lead.md"), "persona")
	writeKnowledgeFile(t, filepath.Join(frameworkDir, "knowledge", "aidlc-shared", "old.md"), "old")

	root, err := os.OpenRoot(frameworkDir)
	if err != nil {
		t.Fatalf("OpenRoot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("Root.Close() error = %v", err)
		}
	})
	input := knowledge.RosterInput{
		Stage:        graph.Stage{Mode: "inline", LeadAgent: "lead"},
		Framework:    knowledge.Source{FS: root.FS(), DisplayPrefix: ".codex"},
		FrameworkDir: frameworkDir,
	}

	first, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("first BuildRoster() error = %v, want nil", err)
	}
	if !contains(first.Paths, ".codex/knowledge/aidlc-shared/old.md") {
		t.Fatalf("first BuildRoster() paths = %#v, want old.md", first.Paths)
	}
	if err := root.Remove("knowledge/aidlc-shared/old.md"); err != nil {
		t.Fatalf("remove old knowledge: %v", err)
	}
	writeRootFile(t, root, "knowledge/aidlc-shared/new.md", "new")

	second, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("second BuildRoster() error = %v, want nil", err)
	}
	if contains(second.Paths, ".codex/knowledge/aidlc-shared/old.md") ||
		!contains(second.Paths, ".codex/knowledge/aidlc-shared/new.md") {
		t.Errorf("second BuildRoster() paths = %#v, want fresh edit", second.Paths)
	}

	file, err := root.Open("agents/lead.md")
	if err != nil {
		t.Fatalf("Root.Open() after BuildRoster() error = %v, want borrowed root still open", err)
	}
	if err := file.Close(); err != nil {
		t.Errorf("close post-read file: %v", err)
	}
}

func TestBuildRosterIntegrationClassifiesSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink availability varies on Windows")
	}
	project := t.TempDir()
	frameworkDir := filepath.Join(project, ".codex")
	sharedDir := filepath.Join(frameworkDir, "knowledge", "aidlc-shared")
	if err := os.MkdirAll(filepath.Join(frameworkDir, "agents"), 0o700); err != nil {
		t.Fatalf("create agents directory: %v", err)
	}
	if err := os.MkdirAll(sharedDir, 0o700); err != nil {
		t.Fatalf("create shared knowledge directory: %v", err)
	}
	writeKnowledgeFile(t, filepath.Join(frameworkDir, "agents", "lead.md"), "persona")
	writeKnowledgeFile(t, filepath.Join(sharedDir, "inside-target.md"), "inside")
	if err := os.Symlink("inside-target.md", filepath.Join(sharedDir, "inside.md")); err != nil {
		t.Fatalf("create in-root symlink: %v", err)
	}
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "outside.md")
	writeKnowledgeFile(t, outsideFile, "outside")
	if err := os.Symlink(outsideFile, filepath.Join(sharedDir, "outside.md")); err != nil {
		t.Fatalf("create outward symlink: %v", err)
	}
	linkedDirTarget := filepath.Join(outsideDir, "linked-dir")
	if err := os.Mkdir(linkedDirTarget, 0o700); err != nil {
		t.Fatalf("create linked directory target: %v", err)
	}
	writeKnowledgeFile(t, filepath.Join(linkedDirTarget, "hidden.md"), "hidden")
	if err := os.Symlink(linkedDirTarget, filepath.Join(sharedDir, "linked-dir")); err != nil {
		t.Fatalf("create directory symlink: %v", err)
	}

	root, err := os.OpenRoot(frameworkDir)
	if err != nil {
		t.Fatalf("OpenRoot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("Root.Close() error = %v", err)
		}
	})
	got, err := knowledge.BuildRoster(knowledge.RosterInput{
		Stage:        graph.Stage{Mode: "inline", LeadAgent: "lead"},
		Framework:    knowledge.Source{FS: root.FS(), DisplayPrefix: ".codex"},
		FrameworkDir: frameworkDir,
	})
	if err != nil {
		t.Fatalf("BuildRoster() error = %v, want nil", err)
	}
	if !contains(got.Paths, ".codex/knowledge/aidlc-shared/inside.md") ||
		!contains(got.Paths, ".codex/knowledge/aidlc-shared/inside-target.md") {
		t.Errorf("BuildRoster() paths = %#v, want in-root symlink and target", got.Paths)
	}
	if contains(got.Paths, ".codex/knowledge/aidlc-shared/outside.md") ||
		contains(got.Paths, ".codex/knowledge/aidlc-shared/linked-dir/hidden.md") {
		t.Errorf("BuildRoster() paths = %#v, outward or directory symlink escaped", got.Paths)
	}
	warnings := strings.Join(got.Warnings, "\n")
	if !strings.Contains(warnings, "outside.md") {
		t.Errorf("BuildRoster() warnings = %#v, want outward symlink warning", got.Warnings)
	}
}

func writeKnowledgeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

func writeRootFile(t *testing.T, root *os.Root, name, content string) {
	t.Helper()
	file, err := root.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatalf("open root file %q: %v", name, err)
	}
	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("write root file %q: %v", name, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close root file %q: %v", name, err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
