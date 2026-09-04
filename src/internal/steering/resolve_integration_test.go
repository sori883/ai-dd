//go:build integration

package steering_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/steering"
)

func TestReadResolvedRulesIntegrationPreservesPOSIXBackslashSpace(t *testing.T) {
	t.Parallel()

	if filepath.Separator == '\\' {
		t.Skip("POSIX literal backslash Space case")
	}
	projectDir := t.TempDir()
	memoryDir := filepath.Join(projectDir, "rules")
	writeRuleFile(t, memoryDir, "org.md", "POSIX backslash Space rule")
	resolved, err := steering.ResolveRulePaths(
		projectDir,
		`team\active`,
		"rules",
		[]graph.Rule{{Path: "aidlc/spaces/default/memory/org.md", Scope: "org"}},
	)
	if err != nil {
		t.Fatalf("ResolveRulePaths() error = %v", err)
	}
	if len(resolved.Entries) != 1 || resolved.Entries[0].Path != `aidlc/spaces/team\active/memory/org.md` {
		t.Fatalf("ResolveRulePaths() entries = %#v, want literal backslash display path", resolved.Entries)
	}

	memoryRoot, err := os.OpenRoot(resolved.MemoryDir)
	if err != nil {
		t.Fatalf("open resolved Memory root: %v", err)
	}
	t.Cleanup(func() {
		if err := memoryRoot.Close(); err != nil {
			t.Errorf("memory root Close() error = %v", err)
		}
	})
	got, err := steering.ReadResolvedRules(nil, memoryRoot.FS(), resolved.Entries)
	if err != nil {
		t.Fatalf("ReadResolvedRules() error = %v", err)
	}
	want := []steering.RuleContent{{
		Path: `aidlc/spaces/team\active/memory/org.md`,
		Text: "POSIX backslash Space rule",
	}}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("ReadResolvedRules() = %#v, want %#v", got, want)
	}
	if file, err := memoryRoot.Open("org.md"); err != nil {
		t.Fatalf("memory root Open() after read error = %v", err)
	} else if err := file.Close(); err != nil {
		t.Errorf("memory root file Close() error = %v", err)
	}
}

func TestReadResolvedRulesIntegrationConnectsResolverAndOverrideRoot(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	refs := []graph.Rule{{Path: "aidlc/spaces/default/memory/rule.md", Scope: "phase"}}
	defaultMemoryDir := filepath.Join(projectDir, "aidlc", "spaces", "default", "memory")
	writeRuleFile(t, defaultMemoryDir, "rule.md", "default Space rule")
	writeRuleFile(t, filepath.Join(projectDir, "aidlc", "spaces", "active", "memory"), "rule.md", "default fallback rule")
	overrideDir := filepath.Join(projectDir, "configured", "rules")
	writeRuleFile(t, overrideDir, "rule.md", "relative override first")

	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatalf("open project root: %v", err)
	}
	t.Cleanup(func() {
		if err := projectRoot.Close(); err != nil {
			t.Errorf("project root Close() error = %v", err)
		}
	})
	resolved, err := steering.ResolveRulePaths(projectDir, "active", "configured/rules", refs)
	if err != nil {
		t.Fatalf("ResolveRulePaths() error = %v", err)
	}
	if resolved.MemoryDir != overrideDir {
		t.Fatalf("MemoryDir = %q, want relative override %q", resolved.MemoryDir, overrideDir)
	}
	if len(resolved.Entries) != 1 || resolved.Entries[0].Path != "aidlc/spaces/active/memory/rule.md" {
		t.Fatalf("ResolveRulePaths() entries = %#v, want active Space display path", resolved.Entries)
	}

	memoryRoot, err := os.OpenRoot(resolved.MemoryDir)
	if err != nil {
		t.Fatalf("open resolved Memory root: %v", err)
	}
	t.Cleanup(func() {
		if err := memoryRoot.Close(); err != nil {
			t.Errorf("memory root Close() error = %v", err)
		}
	})
	first, err := steering.ReadResolvedRules(projectRoot.FS(), memoryRoot.FS(), resolved.Entries)
	if err != nil {
		t.Fatalf("first ReadResolvedRules() error = %v", err)
	}
	wantFirst := []steering.RuleContent{{
		Path: "aidlc/spaces/active/memory/rule.md",
		Text: "relative override first",
	}}
	if len(first) != 1 || first[0] != wantFirst[0] {
		t.Fatalf("first ReadResolvedRules() = %#v, want %#v", first, wantFirst)
	}

	if file, err := memoryRoot.Open("rule.md"); err != nil {
		t.Fatalf("memory root Open() after first read error = %v", err)
	} else if err := file.Close(); err != nil {
		t.Errorf("memory root file Close() error = %v", err)
	}
	writeRuleFile(t, overrideDir, "rule.md", "relative override second")
	second, err := steering.ReadResolvedRules(projectRoot.FS(), memoryRoot.FS(), resolved.Entries)
	if err != nil {
		t.Fatalf("second ReadResolvedRules() error = %v", err)
	}
	wantSecond := []steering.RuleContent{{
		Path: "aidlc/spaces/active/memory/rule.md",
		Text: "relative override second",
	}}
	if len(second) != 1 || second[0] != wantSecond[0] {
		t.Fatalf("second ReadResolvedRules() = %#v, want %#v", second, wantSecond)
	}

	missingDir := filepath.Join(projectDir, "missing", "rules")
	if err := os.MkdirAll(missingDir, 0o700); err != nil {
		t.Fatalf("create missing override directory: %v", err)
	}
	missingResolved, err := steering.ResolveRulePaths(projectDir, "active", "missing/rules", refs)
	if err != nil {
		t.Fatalf("ResolveRulePaths() for missing override error = %v", err)
	}
	missingRoot, err := os.OpenRoot(missingResolved.MemoryDir)
	if err != nil {
		t.Fatalf("open missing override root: %v", err)
	}
	defer func() {
		if err := missingRoot.Close(); err != nil {
			t.Errorf("missing root Close() error = %v", err)
		}
	}()
	missing, err := steering.ReadResolvedRules(projectRoot.FS(), missingRoot.FS(), missingResolved.Entries)
	if err == nil {
		t.Fatal("ReadResolvedRules() for missing override error = nil, want no fallback")
	}
	if missing != nil {
		t.Fatalf("ReadResolvedRules() for missing override = %#v, want nil without default fallback", missing)
	}
}

func TestReadResolvedRulesIntegrationReadsBorrowedRootsAndFreshContent(t *testing.T) {
	t.Parallel()

	projectRoot, projectDir := openRulesRoot(t)
	memoryRoot, memoryDir := openRulesRoot(t)
	writeRuleFile(t, projectDir, "project.md", "first project rule")
	writeRuleFile(t, memoryDir, "memory.md", "first memory rule")
	entries := []steering.RulePath{
		{Path: "project.md", ReadPath: "project.md", Source: steering.RuleSourceProject},
		{Path: "aidlc/spaces/active/memory/memory.md", ReadPath: "memory.md", Source: steering.RuleSourceMemory},
	}

	first, err := steering.ReadResolvedRules(projectRoot.FS(), memoryRoot.FS(), entries)
	if err != nil {
		t.Fatalf("first ReadResolvedRules() error = %v", err)
	}
	wantFirst := []steering.RuleContent{
		{Path: "project.md", Text: "first project rule"},
		{Path: "aidlc/spaces/active/memory/memory.md", Text: "first memory rule"},
	}
	if len(first) != len(wantFirst) || first[0] != wantFirst[0] || first[1] != wantFirst[1] {
		t.Fatalf("first ReadResolvedRules() = %#v, want %#v", first, wantFirst)
	}

	projectFile, err := projectRoot.Open("project.md")
	if err != nil {
		t.Fatalf("projectRoot.Open() after ReadResolvedRules() error = %v, want nil", err)
	}
	if err := projectFile.Close(); err != nil {
		t.Errorf("project root file Close() error = %v", err)
	}
	memoryFile, err := memoryRoot.Open("memory.md")
	if err != nil {
		t.Fatalf("memoryRoot.Open() after ReadResolvedRules() error = %v, want nil", err)
	}
	if err := memoryFile.Close(); err != nil {
		t.Errorf("memory root file Close() error = %v", err)
	}

	writeRuleFile(t, projectDir, "project.md", "second project rule")
	writeRuleFile(t, memoryDir, "memory.md", "second memory rule")
	second, err := steering.ReadResolvedRules(projectRoot.FS(), memoryRoot.FS(), entries)
	if err != nil {
		t.Fatalf("second ReadResolvedRules() error = %v", err)
	}
	wantSecond := []steering.RuleContent{
		{Path: "project.md", Text: "second project rule"},
		{Path: "aidlc/spaces/active/memory/memory.md", Text: "second memory rule"},
	}
	if len(second) != len(wantSecond) || second[0] != wantSecond[0] || second[1] != wantSecond[1] {
		t.Fatalf("second ReadResolvedRules() = %#v, want %#v", second, wantSecond)
	}
}

func TestReadResolvedRulesIntegrationAllowsInRootMemorySymlink(t *testing.T) {
	t.Parallel()

	memoryRoot, memoryDir := openRulesRoot(t)
	writeRuleFile(t, memoryDir, "memory-target.md", "in-root memory rule")
	createRulesSymlink(t, "memory-target.md", filepath.Join(memoryDir, "memory.md"))

	got, err := steering.ReadResolvedRules(nil, memoryRoot.FS(), []steering.RulePath{
		{Path: "aidlc/spaces/active/memory/memory.md", ReadPath: "memory.md", Source: steering.RuleSourceMemory},
	})
	if err != nil {
		t.Fatalf("ReadResolvedRules() error = %v, want nil", err)
	}
	want := []steering.RuleContent{{
		Path: "aidlc/spaces/active/memory/memory.md",
		Text: "in-root memory rule",
	}}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("ReadResolvedRules() = %#v, want %#v", got, want)
	}
}

func TestReadResolvedRulesIntegrationRejectsOutwardMemorySymlink(t *testing.T) {
	t.Parallel()

	memoryRoot, memoryDir := openRulesRoot(t)
	outsidePath := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(outsidePath, []byte("outside secret"), 0o600); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	createRulesSymlink(t, outsidePath, filepath.Join(memoryDir, "memory.md"))

	got, err := steering.ReadResolvedRules(nil, memoryRoot.FS(), []steering.RulePath{
		{Path: "aidlc/spaces/active/memory/memory.md", ReadPath: "memory.md", Source: steering.RuleSourceMemory},
	})
	if err == nil {
		t.Fatal("ReadResolvedRules() error = nil, want outward symlink rejection")
	}
	if got != nil {
		t.Errorf("ReadResolvedRules() result = %#v, want nil without external bytes", got)
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadResolvedRules() error = %v, want boundary error rather than missing rule", err)
	}
}
