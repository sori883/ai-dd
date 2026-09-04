package steering_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/steering"
)

func TestResolveRulePathsUsesActiveSpaceAndRulesDirectory(t *testing.T) {
	t.Parallel()

	projectDir := filepath.Join(t.TempDir(), "project")
	refs := []graph.Rule{
		{Path: "aidlc/spaces/default/memory/org.md", Scope: "org"},
		{Path: "docs/project-rule.md", Scope: "project"},
		{Path: "prefix/memory/team.md", Scope: "team"},
	}
	externalRulesDir := filepath.Join(t.TempDir(), "external-rules")

	tests := []struct {
		name     string
		rulesDir string
		wantDir  string
	}{
		{
			name:     "default directory",
			wantDir:  filepath.Join(projectDir, "aidlc", "spaces", "active", "memory"),
			rulesDir: "",
		},
		{
			name:     "relative override",
			wantDir:  filepath.Join(projectDir, "configured", "rules"),
			rulesDir: "configured/rules",
		},
		{
			name:     "absolute override",
			wantDir:  externalRulesDir,
			rulesDir: externalRulesDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := steering.ResolveRulePaths(projectDir, "active", tt.rulesDir, refs)
			if err != nil {
				t.Fatalf("ResolveRulePaths() error = %v", err)
			}
			if got.MemoryDir != tt.wantDir {
				t.Fatalf("MemoryDir = %q, want %q", got.MemoryDir, tt.wantDir)
			}
			wantEntries := []steering.RulePath{
				{
					Path:     "aidlc/spaces/active/memory/org.md",
					ReadPath: "org.md",
					Source:   steering.RuleSourceMemory,
				},
				{
					Path:     "docs/project-rule.md",
					ReadPath: "docs/project-rule.md",
					Source:   steering.RuleSourceProject,
				},
				{
					Path:     "aidlc/spaces/active/memory/team.md",
					ReadPath: "team.md",
					Source:   steering.RuleSourceMemory,
				},
			}
			if !reflect.DeepEqual(got.Entries, wantEntries) {
				t.Fatalf("Entries = %#v, want %#v", got.Entries, wantEntries)
			}
		})
	}
}

func TestResolveRulePathsUsesFirstMemoryMarker(t *testing.T) {
	t.Parallel()

	got, err := steering.ResolveRulePaths(
		filepath.Join(t.TempDir(), "project"),
		"active",
		"",
		[]graph.Rule{
			{Path: "prefix/memory/first/memory/second.md", Scope: "phase"},
			{Path: "memory/project.md", Scope: "project"},
		},
	)
	if err != nil {
		t.Fatalf("ResolveRulePaths() error = %v", err)
	}
	want := []steering.RulePath{
		{
			Path:     "aidlc/spaces/active/memory/first/memory/second.md",
			ReadPath: "first/memory/second.md",
			Source:   steering.RuleSourceMemory,
		},
		{
			Path:     "memory/project.md",
			ReadPath: "memory/project.md",
			Source:   steering.RuleSourceProject,
		},
	}
	if !reflect.DeepEqual(got.Entries, want) {
		t.Fatalf("Entries = %#v, want %#v", got.Entries, want)
	}
}

func TestResolveRulePathsEntriesPassReadResolvedRulesValidation(t *testing.T) {
	t.Parallel()

	projectDir := filepath.Join(t.TempDir(), "project")
	refs := []graph.Rule{
		{Path: "prefix/memory/first/memory/second.md", Scope: "phase"},
		{Path: "prefix/memory/first/memory/second.md", Scope: "phase"},
		{Path: "project.md", Scope: "project"},
	}
	resolved, err := steering.ResolveRulePaths(projectDir, "active", "configured/rules", refs)
	if err != nil {
		t.Fatalf("ResolveRulePaths() error = %v", err)
	}

	projectFS := fstest.MapFS{
		"project.md": &fstest.MapFile{Data: []byte("project rule")},
	}
	memoryFS := fstest.MapFS{
		"first/memory/second.md": &fstest.MapFile{Data: []byte("memory rule")},
	}
	got, err := steering.ReadResolvedRules(projectFS, memoryFS, resolved.Entries)
	if err != nil {
		t.Fatalf("ReadResolvedRules() on ResolveRulePaths entries error = %v", err)
	}
	want := []steering.RuleContent{
		{Path: "aidlc/spaces/active/memory/first/memory/second.md", Text: "memory rule"},
		{Path: "project.md", Text: "project rule"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadResolvedRules() = %#v, want %#v", got, want)
	}
}

func TestResolveRulePathsAllowsExplicitRulesDirectory(t *testing.T) {
	t.Parallel()

	projectDir := filepath.Join(t.TempDir(), "project")
	externalBase := t.TempDir()
	absoluteRulesDir := externalBase + string(filepath.Separator) + "nested" + string(filepath.Separator) + ".." + string(filepath.Separator) + "rules"
	refs := []graph.Rule{{Path: "rule.md", Scope: "project"}}
	tests := []struct {
		name     string
		rulesDir string
		wantDir  string
	}{
		{name: "current directory", rulesDir: ".", wantDir: filepath.Join(projectDir, ".")},
		{name: "relative current directory", rulesDir: "./rules", wantDir: filepath.Join(projectDir, "./rules")},
		{name: "relative parent directory", rulesDir: "../rules", wantDir: filepath.Join(projectDir, "../rules")},
		{name: "absolute dot segment", rulesDir: absoluteRulesDir, wantDir: absoluteRulesDir},
		{name: "whitespace is not trimmed", rulesDir: " ./rules ", wantDir: filepath.Join(projectDir, " ./rules ")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := steering.ResolveRulePaths(projectDir, "active", tt.rulesDir, refs)
			if err != nil {
				t.Fatalf("ResolveRulePaths() error = %v", err)
			}
			if got.MemoryDir != tt.wantDir {
				t.Fatalf("MemoryDir = %q, want %q", got.MemoryDir, tt.wantDir)
			}
		})
	}
}

func TestResolveRulePathsAllowsDotSegmentProjectDirectory(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	projectDir := baseDir + string(filepath.Separator) + "project" + string(filepath.Separator) + ".." + string(filepath.Separator) + "actual"
	got, err := steering.ResolveRulePaths(projectDir, "active", "", nil)
	if err != nil {
		t.Fatalf("ResolveRulePaths() error = %v", err)
	}
	wantDir := filepath.Join(projectDir, "aidlc", "spaces", "active", "memory")
	if got.MemoryDir != wantDir {
		t.Fatalf("MemoryDir = %q, want %q", got.MemoryDir, wantDir)
	}
}

func TestResolveRulePathsNativeColonNames(t *testing.T) {
	t.Parallel()

	projectDir := filepath.Join(t.TempDir(), "project")
	got, err := steering.ResolveRulePaths(
		projectDir,
		"C:record",
		"",
		[]graph.Rule{{Path: "C:rule.md", Scope: "project"}},
	)
	if filepath.Separator == '\\' {
		if err == nil {
			t.Fatalf("ResolveRulePaths() error = nil, want native Windows colon path rejection: %#v", got)
		}
		return
	}
	if err != nil {
		t.Fatalf("ResolveRulePaths() error = %v, want POSIX colon names accepted", err)
	}
	if len(got.Entries) != 1 || got.Entries[0].Path != "C:rule.md" || got.Entries[0].ReadPath != "C:rule.md" {
		t.Fatalf("ResolveRulePaths() entries = %#v, want POSIX colon rule path", got.Entries)
	}
}

func TestResolveRulePathsNativeDriveRelativeRulesDirectory(t *testing.T) {
	t.Parallel()

	projectDir := filepath.Join(t.TempDir(), "project")
	got, err := steering.ResolveRulePaths(projectDir, "active", "C:rules", nil)
	if filepath.Separator == '\\' {
		if err == nil {
			t.Fatalf("ResolveRulePaths() error = nil, result = %#v; want native drive-relative rejection", got)
		}
		if !errors.Is(err, fs.ErrInvalid) {
			t.Fatalf("ResolveRulePaths() error = %v, want fs.ErrInvalid", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("ResolveRulePaths() error = %v, want POSIX colon directory accepted", err)
	}
	wantDir := filepath.Join(projectDir, "C:rules")
	if got.MemoryDir != wantDir {
		t.Fatalf("MemoryDir = %q, want %q", got.MemoryDir, wantDir)
	}
}

func TestResolveRulePathsRejectsWindowsDriveLessRootedRulesDirectory(t *testing.T) {
	t.Parallel()

	if filepath.Separator != '\\' {
		t.Skip("Windows native rooted path case")
	}
	projectDir := filepath.Join(t.TempDir(), "project")
	for _, rulesDir := range []string{`\rules`, `/rules`} {
		t.Run(rulesDir, func(t *testing.T) {
			t.Parallel()

			got, err := steering.ResolveRulePaths(projectDir, "active", rulesDir, nil)
			if err == nil {
				t.Fatalf("ResolveRulePaths() error = nil, result = %#v; want drive-less rooted rejection", got)
			}
			if !errors.Is(err, fs.ErrInvalid) {
				t.Fatalf("ResolveRulePaths() error = %v, want fs.ErrInvalid", err)
			}
		})
	}
}

func TestResolveRulePathsRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	projectDir := filepath.Join(t.TempDir(), "project")
	validRefs := []graph.Rule{{Path: "rule.md", Scope: "project"}}
	tests := []struct {
		name        string
		projectDir  string
		activeSpace string
		rulesDir    string
		refs        []graph.Rule
	}{
		{name: "relative project directory", projectDir: "project", activeSpace: "active", refs: validRefs},
		{name: "empty project directory", projectDir: "", activeSpace: "active", refs: validRefs},
		{name: "empty active space", projectDir: projectDir, activeSpace: "", refs: validRefs},
		{name: "nested active space", projectDir: projectDir, activeSpace: "team/active", refs: validRefs},
		{name: "backslash active space", projectDir: projectDir, activeSpace: `team\active`, refs: validRefs},
		{name: "dot active space", projectDir: projectDir, activeSpace: ".", refs: validRefs},
		{name: "empty rule path", projectDir: projectDir, activeSpace: "active", refs: []graph.Rule{{Path: "", Scope: "project"}}},
		{name: "dot rule path", projectDir: projectDir, activeSpace: "active", refs: []graph.Rule{{Path: ".", Scope: "project"}}},
		{name: "parent rule path", projectDir: projectDir, activeSpace: "active", refs: []graph.Rule{{Path: "../rule.md", Scope: "project"}}},
		{name: "absolute rule path", projectDir: projectDir, activeSpace: "active", refs: []graph.Rule{{Path: "/rule.md", Scope: "project"}}},
		{name: "backslash rule path", projectDir: projectDir, activeSpace: "active", refs: []graph.Rule{{Path: `rules\rule.md`, Scope: "project"}}},
		{name: "parent memory path", projectDir: projectDir, activeSpace: "active", refs: []graph.Rule{{Path: "prefix/memory/../rule.md", Scope: "project"}}},
		{name: "empty memory component", projectDir: projectDir, activeSpace: "active", refs: []graph.Rule{{Path: "prefix//memory/rule.md", Scope: "project"}}},
		{name: "empty memory path", projectDir: projectDir, activeSpace: "active", refs: []graph.Rule{{Path: "prefix/memory/", Scope: "project"}}},
		{name: "absolute memory reference", projectDir: projectDir, activeSpace: "active", refs: []graph.Rule{{Path: "/memory/rule.md", Scope: "project"}}},
		{name: "invalid utf-8 rule path", projectDir: projectDir, activeSpace: "active", refs: []graph.Rule{{Path: string([]byte{0xff}), Scope: "project"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := steering.ResolveRulePaths(tt.projectDir, tt.activeSpace, tt.rulesDir, tt.refs)
			if err == nil {
				t.Fatalf("ResolveRulePaths() error = nil, result = %#v", got)
			}
			if !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("ResolveRulePaths() error = %v, want fs.ErrInvalid", err)
			}
			if !reflect.DeepEqual(got, steering.RulePaths{}) {
				t.Errorf("ResolveRulePaths() result on error = %#v, want zero value", got)
			}
		})
	}
}

func TestReadResolvedRules(t *testing.T) {
	projectFS := &readFileFS{files: map[string]readFileResult{
		"project.md":  {data: []byte("project rule")},
		"template.md": {data: []byte("# template")},
	}}
	memoryFS := &readFileFS{files: map[string]readFileResult{
		"memory.md": {data: []byte("memory rule")},
	}}
	entries := []steering.RulePath{
		{Path: "aidlc/spaces/active/memory/memory.md", ReadPath: "memory.md", Source: steering.RuleSourceMemory},
		{Path: "project.md", ReadPath: "project.md", Source: steering.RuleSourceProject},
		{Path: "aidlc/spaces/active/memory/memory.md", ReadPath: "memory.md", Source: steering.RuleSourceMemory},
		{Path: "aidlc/spaces/active/memory/template.md", ReadPath: "template.md", Source: steering.RuleSourceProject},
		{Path: "project.md", ReadPath: "project.md", Source: steering.RuleSourceProject},
	}

	got, err := steering.ReadResolvedRules(projectFS, memoryFS, entries)
	if err != nil {
		t.Fatalf("ReadResolvedRules() error = %v", err)
	}
	want := []steering.RuleContent{
		{Path: "aidlc/spaces/active/memory/memory.md", Text: "memory rule"},
		{Path: "project.md", Text: "project rule"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadResolvedRules() = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(memoryFS.calls, []string{"memory.md"}) {
		t.Fatalf("memory read paths = %q, want one first read", memoryFS.calls)
	}
	if !reflect.DeepEqual(projectFS.calls, []string{"project.md", "template.md"}) {
		t.Fatalf("project read paths = %q, want unique reads in display order", projectFS.calls)
	}

	got[0].Text = "caller mutation"
	memoryFS.files["memory.md"] = readFileResult{data: []byte("fresh memory rule")}
	got, err = steering.ReadResolvedRules(projectFS, memoryFS, entries)
	if err != nil {
		t.Fatalf("ReadResolvedRules() second error = %v", err)
	}
	want[0].Text = "fresh memory rule"
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("second ReadResolvedRules() = %#v, want %#v", got, want)
	}
}

func TestReadResolvedRulesAllowsUnusedFilesystems(t *testing.T) {
	projectFS := &readFileFS{files: map[string]readFileResult{
		"project.md": {data: []byte("project rule")},
	}}
	memoryFS := &readFileFS{files: map[string]readFileResult{
		"memory.md": {data: []byte("memory rule")},
	}}

	projectRules, err := steering.ReadResolvedRules(projectFS, nil, []steering.RulePath{
		{Path: "project.md", ReadPath: "project.md", Source: steering.RuleSourceProject},
	})
	if err != nil {
		t.Fatalf("project-only ReadResolvedRules() error = %v", err)
	}
	if !reflect.DeepEqual(projectRules, []steering.RuleContent{{Path: "project.md", Text: "project rule"}}) {
		t.Fatalf("project-only rules = %#v", projectRules)
	}

	memoryRules, err := steering.ReadResolvedRules(nil, memoryFS, []steering.RulePath{
		{Path: "memory.md", ReadPath: "memory.md", Source: steering.RuleSourceMemory},
	})
	if err != nil {
		t.Fatalf("memory-only ReadResolvedRules() error = %v", err)
	}
	if !reflect.DeepEqual(memoryRules, []steering.RuleContent{{Path: "memory.md", Text: "memory rule"}}) {
		t.Fatalf("memory-only rules = %#v", memoryRules)
	}

	projectFS = &readFileFS{files: map[string]readFileResult{
		"project.md": {data: []byte("project rule")},
	}}
	duplicateSourceRules, err := steering.ReadResolvedRules(projectFS, nil, []steering.RulePath{
		{Path: "same.md", ReadPath: "project.md", Source: steering.RuleSourceProject},
		{Path: "same.md", ReadPath: "memory.md", Source: steering.RuleSourceMemory},
	})
	if err != nil {
		t.Fatalf("duplicate-source ReadResolvedRules() error = %v", err)
	}
	if !reflect.DeepEqual(duplicateSourceRules, []steering.RuleContent{{Path: "same.md", Text: "project rule"}}) {
		t.Fatalf("duplicate-source rules = %#v", duplicateSourceRules)
	}
	if !reflect.DeepEqual(projectFS.calls, []string{"project.md"}) {
		t.Fatalf("duplicate-source project read paths = %q, want first source only", projectFS.calls)
	}

	projectFS = &readFileFS{files: map[string]readFileResult{
		"project.md": {data: []byte("project rule")},
	}}
	_, err = steering.ReadResolvedRules(projectFS, nil, []steering.RulePath{
		{Path: "project.md", ReadPath: "project.md", Source: steering.RuleSourceProject},
		{Path: "memory.md", ReadPath: "memory.md", Source: steering.RuleSourceMemory},
	})
	if !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("mixed-source ReadResolvedRules() error = %v, want fs.ErrInvalid", err)
	}
	if len(projectFS.calls) != 0 {
		t.Fatalf("mixed-source ReadResolvedRules() performed %d project reads before nil check", len(projectFS.calls))
	}

	empty, err := steering.ReadResolvedRules(nil, nil, []steering.RulePath{})
	if err != nil {
		t.Fatalf("empty ReadResolvedRules() error = %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty ReadResolvedRules() = %#v, want non-nil empty result", empty)
	}
}

func TestReadResolvedRulesRejectsInvalidEntriesBeforeIO(t *testing.T) {
	projectFS := &readFileFS{files: map[string]readFileResult{
		"valid.md": {data: []byte("valid rule")},
	}}
	entries := []steering.RulePath{
		{Path: "display.md", ReadPath: "valid.md", Source: steering.RuleSourceProject},
		{Path: "display.md", ReadPath: `invalid\\path.md`, Source: steering.RuleSourceProject},
	}

	got, err := steering.ReadResolvedRules(projectFS, nil, entries)
	if !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("ReadResolvedRules() error = %v, want fs.ErrInvalid", err)
	}
	if got != nil {
		t.Fatalf("ReadResolvedRules() result = %#v, want nil", got)
	}
	if len(projectFS.calls) != 0 {
		t.Fatalf("ReadResolvedRules() performed %d reads before validating duplicate entry", len(projectFS.calls))
	}
}

func TestReadResolvedRulesRejectsNilFilesystemWithoutPanic(t *testing.T) {
	var typedNil *countingFS
	entries := []steering.RulePath{
		{Path: "project.md", ReadPath: "project.md", Source: steering.RuleSourceProject},
	}

	got, err, panicValue := readResolvedRulesSafely(typedNil, nil, entries)
	if panicValue != nil {
		t.Fatalf("ReadResolvedRules() panicked: %v", panicValue)
	}
	if !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("ReadResolvedRules() error = %v, want fs.ErrInvalid", err)
	}
	if got != nil {
		t.Fatalf("ReadResolvedRules() result = %#v, want nil", got)
	}
}

func TestReadResolvedRulesReportsDisplayPathOnReadFailure(t *testing.T) {
	readFailure := errors.New("injected rule read failure")
	projectFS := &readFileFS{files: map[string]readFileResult{
		"physical.md": {err: readFailure},
	}}
	entries := []steering.RulePath{
		{Path: "aidlc/spaces/active/memory/display.md", ReadPath: "physical.md", Source: steering.RuleSourceProject},
	}

	got, err := steering.ReadResolvedRules(projectFS, nil, entries)
	if !errors.Is(err, readFailure) {
		t.Fatalf("ReadResolvedRules() error = %v, want cause %v", err, readFailure)
	}
	if !strings.Contains(err.Error(), "aidlc/spaces/active/memory/display.md") {
		t.Fatalf("ReadResolvedRules() error = %v, want display path context", err)
	}
	if got != nil {
		t.Fatalf("ReadResolvedRules() result = %#v, want nil", got)
	}
}

func readResolvedRulesSafely(
	projectFS, memoryFS fs.FS,
	entries []steering.RulePath,
) (rules []steering.RuleContent, err error, panicValue any) {
	defer func() {
		panicValue = recover()
	}()
	rules, err = steering.ReadResolvedRules(projectFS, memoryFS, entries)
	return rules, err, nil
}
