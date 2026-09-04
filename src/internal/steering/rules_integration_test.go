//go:build integration

package steering_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/sori883/ai-dd/src/internal/steering"
)

func TestReadRulesIntegrationReadsRootAndFreshContent(t *testing.T) {
	t.Parallel()

	root, rulesDir := openRulesRoot(t)
	writeRuleFile(t, rulesDir, "required.md", "first rule")

	first, err := steering.ReadRules(root.FS(), []string{"required.md"})
	if err != nil {
		t.Fatalf("first ReadRules() error = %v, want nil", err)
	}
	if len(first) != 1 || first[0] != (steering.RuleContent{
		Path: "required.md",
		Text: "first rule",
	}) {
		t.Fatalf("first ReadRules() = %#v, want first rule", first)
	}

	file, err := root.Open("required.md")
	if err != nil {
		t.Fatalf("root.Open() after ReadRules() error = %v, want nil", err)
	}
	if err := file.Close(); err != nil {
		t.Errorf("root file Close() error = %v, want nil", err)
	}

	writeRuleFile(t, rulesDir, "required.md", "second rule")
	second, err := steering.ReadRules(root.FS(), []string{"required.md"})
	if err != nil {
		t.Fatalf("second ReadRules() error = %v, want nil", err)
	}
	if len(second) != 1 || second[0] != (steering.RuleContent{
		Path: "required.md",
		Text: "second rule",
	}) {
		t.Fatalf("second ReadRules() = %#v, want second rule", second)
	}
}

func TestReadRulesIntegrationAllowsInRootSymlink(t *testing.T) {
	t.Parallel()

	root, rulesDir := openRulesRoot(t)
	writeRuleFile(t, rulesDir, "target.md", "in-root rule")
	createRulesSymlink(t, "target.md", filepath.Join(rulesDir, "required.md"))

	got, err := steering.ReadRules(root.FS(), []string{"required.md"})
	if err != nil {
		t.Fatalf("ReadRules() error = %v, want nil", err)
	}
	want := []steering.RuleContent{{Path: "required.md", Text: "in-root rule"}}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("ReadRules() = %#v, want %#v", got, want)
	}
}

func TestReadRulesIntegrationRejectsOutwardSymlink(t *testing.T) {
	t.Parallel()

	root, rulesDir := openRulesRoot(t)
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "secret.md")
	if err := os.WriteFile(outsidePath, []byte("outside secret"), 0o600); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	createRulesSymlink(t, outsidePath, filepath.Join(rulesDir, "required.md"))

	got, err := steering.ReadRules(root.FS(), []string{"required.md"})
	if err == nil {
		t.Fatal("ReadRules() error = nil, want outward symlink rejection")
	}
	if got != nil {
		t.Errorf("ReadRules() result = %#v, want nil without external bytes", got)
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadRules() error = %v, want boundary error rather than missing rule", err)
	}
}

func TestSteeringSymlinkErrorClassification(t *testing.T) {
	tests := []struct {
		name string
		goos string
		err  error
		want bool
	}{
		{name: "windows permission", goos: "windows", err: fs.ErrPermission, want: true},
		{name: "windows privilege", goos: "windows", err: syscall.Errno(1314), want: true},
		{name: "windows unsupported", goos: "windows", err: errors.ErrUnsupported, want: true},
		{name: "wrapped windows permission", goos: "windows", err: fmt.Errorf("create link: %w", fs.ErrPermission), want: true},
		{name: "windows disk full", goos: "windows", err: syscall.Errno(112), want: false},
		{name: "windows path failure", goos: "windows", err: errors.New("path is invalid"), want: false},
		{name: "unix permission", goos: "darwin", err: fs.ErrPermission, want: false},
		{name: "unix privilege", goos: "linux", err: syscall.Errno(1314), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isExpectedSymlinkUnavailable(tt.goos, tt.err); got != tt.want {
				t.Errorf("isExpectedSymlinkUnavailable(%q, %v) = %t, want %t", tt.goos, tt.err, got, tt.want)
			}
		})
	}
}

func openRulesRoot(t *testing.T) (*os.Root, string) {
	t.Helper()

	project := t.TempDir()
	rulesDir := filepath.Join(project, "rules")
	if err := os.Mkdir(rulesDir, 0o700); err != nil {
		t.Fatalf("create rules directory: %v", err)
	}
	root, err := os.OpenRoot(rulesDir)
	if err != nil {
		t.Fatalf("open rules root: %v", err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("rules root Close() error = %v", err)
		}
	})
	return root, rulesDir
}

func writeRuleFile(t *testing.T, rulesDir, name, content string) {
	t.Helper()

	path := filepath.Join(rulesDir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create parent for %q: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %q: %v", name, err)
	}
}

func createRulesSymlink(t *testing.T, target, link string) {
	t.Helper()

	if err := os.Symlink(target, link); err != nil {
		if isExpectedSymlinkUnavailable(runtime.GOOS, err) {
			t.Skipf("Windows symlink unavailable (Developer Mode, privilege, or filesystem support required): %v", err)
		}
		t.Fatal(err)
	}
}

func isExpectedSymlinkUnavailable(goos string, err error) bool {
	return goos == "windows" &&
		(errors.Is(err, fs.ErrPermission) || errors.Is(err, errors.ErrUnsupported) ||
			errors.Is(err, syscall.Errno(1314)))
}
