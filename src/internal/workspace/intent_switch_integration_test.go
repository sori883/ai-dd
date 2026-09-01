//go:build integration

package workspace

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"testing"
)

func TestSwitchIntentPreservesProtectedDataAndCursorMode(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	dirName := "240901-build-auth"
	intentsRoot := filepath.Join(project, "aidlc", "spaces", "team", "intents")
	writeSpaceFixture(
		t,
		project,
		[]string{filepath.ToSlash(filepath.Join("aidlc", "spaces", "team", "knowledge", "audit"))},
		map[string]string{
			"aidlc/active-space":                                       " \tteam \n",
			"aidlc/.aidlc-sessions/session.binding.json":               "binding",
			"aidlc/.aidlc-sessions/.current-session":                   "session",
			"aidlc/.aidlc-sessions/session.rebind-offer":               "offer",
			"aidlc/.aidlc-sessions/session":                            "intent id",
			"aidlc/spaces/team/intents/intents.json":                   `[]`,
			"aidlc/spaces/team/intents/active-intent":                  "old bytes",
			"aidlc/spaces/team/intents/" + dirName + "/aidlc-state.md": "state",
			"aidlc/spaces/team/knowledge/audit/keep.jsonl":             "audit",
			".codex/config.toml":                                       "rules",
		},
	)
	cursor := filepath.Join(intentsRoot, "active-intent")
	if err := os.Chmod(cursor, 0o640); err != nil {
		t.Fatal(err)
	}
	before := snapshotSpaceTree(t, project)
	selection, err := SwitchIntent(RootInput{ExplicitDir: project}, dirName)
	if err != nil || selection != (IntentSelection{SpaceName: "team", DirName: dirName}) {
		t.Fatalf("SwitchIntent() = (%+v, %v), want selected exact directory", selection, err)
	}
	data, err := os.ReadFile(cursor)
	if err != nil || string(data) != dirName+"\n" {
		t.Errorf("active-intent = (%q, %v), want %q", data, err, dirName+"\n")
	}
	info, err := os.Stat(cursor)
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Errorf("active-intent mode = (%v, %v), want 0640", info, err)
	}
	after := snapshotSpaceTree(t, project)
	for _, mutable := range []string{intentsRoot, cursor} {
		delete(before, mutable)
		delete(after, mutable)
	}
	if !maps.Equal(before, after) {
		t.Error("intent switch changed registry, state, session, audit, config, or shared active-space")
	}
}

func TestSwitchIntentRejectsNonRegularActiveIntent(t *testing.T) {
	t.Parallel()

	t.Run("directory", func(t *testing.T) {
		t.Parallel()

		project, intentsRoot, dirName := intentSwitchIntegrationFixture(t)
		if err := os.Mkdir(filepath.Join(intentsRoot, "active-intent"), 0o700); err != nil {
			t.Fatal(err)
		}
		before := snapshotSpaceTree(t, project)
		selection, err := SwitchIntent(RootInput{ExplicitDir: project}, dirName)
		if selection != (IntentSelection{}) || !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("SwitchIntent() = (%+v, %v), want zero and fs.ErrInvalid", selection, err)
		}
		if !maps.Equal(before, snapshotSpaceTree(t, project)) {
			t.Error("rejected active-intent directory changed the project")
		}
	})

	for _, kind := range []string{"inside relative", "outside relative", "absolute inside", "broken"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			project, intentsRoot, dirName := intentSwitchIntegrationFixture(t)
			outside := t.TempDir()
			writeSpaceFixture(t, intentsRoot, nil, map[string]string{"old": "old"})
			writeSpaceFixture(t, outside, nil, map[string]string{"keep": "outside"})
			target := "old"
			switch kind {
			case "outside relative":
				var err error
				target, err = filepath.Rel(intentsRoot, filepath.Join(outside, "keep"))
				if err != nil {
					t.Fatal(err)
				}
			case "absolute inside":
				target = filepath.Join(intentsRoot, "old")
			case "broken":
				target = "missing"
			}
			createSpaceSymlink(t, target, filepath.Join(intentsRoot, "active-intent"))
			before := snapshotSpaceTree(t, project)
			outsideBefore := snapshotSpaceTree(t, outside)
			selection, err := SwitchIntent(RootInput{ExplicitDir: project}, dirName)
			if selection != (IntentSelection{}) || !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("SwitchIntent() = (%+v, %v), want zero and fs.ErrInvalid", selection, err)
			}
			if !maps.Equal(before, snapshotSpaceTree(t, project)) {
				t.Error("rejected active-intent symlink changed the project")
			}
			if !maps.Equal(outsideBefore, snapshotSpaceTree(t, outside)) {
				t.Error("rejected active-intent symlink changed the outside tree")
			}
		})
	}
}

func TestSwitchIntentChildLinkBoundaries(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"inside relative", "outside relative", "absolute inside", "broken"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			project := t.TempDir()
			outside := t.TempDir()
			dirName := "240901-build-auth"
			writeSpaceFixture(
				t,
				project,
				[]string{"aidlc/spaces", "storage/intents/" + dirName},
				map[string]string{
					"aidlc/active-space":                             "team\n",
					"storage/intents/" + dirName + "/aidlc-state.md": "state",
				},
			)
			writeSpaceFixture(
				t,
				outside,
				[]string{"storage/intents/" + dirName},
				map[string]string{
					"storage/intents/" + dirName + "/aidlc-state.md": "outside state",
				},
			)
			link := filepath.Join(project, "aidlc", "spaces", "team")
			targetPath := filepath.Join(project, "storage")
			switch kind {
			case "outside relative":
				targetPath = filepath.Join(outside, "storage")
			case "broken":
				targetPath = filepath.Join(project, "missing")
			}
			target := targetPath
			if kind != "absolute inside" {
				var err error
				target, err = filepath.Rel(filepath.Dir(link), targetPath)
				if err != nil {
					t.Fatal(err)
				}
			}
			if kind == "broken" {
				if err := os.MkdirAll(targetPath, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			createSpaceSymlink(t, target, link)
			if kind == "broken" {
				if err := os.Remove(targetPath); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshotSpaceTree(t, project)
			outsideBefore := snapshotSpaceTree(t, outside)
			selection, err := SwitchIntent(RootInput{ExplicitDir: project}, dirName)
			if kind == "inside relative" {
				if err != nil || selection != (IntentSelection{SpaceName: "team", DirName: dirName}) {
					t.Errorf("SwitchIntent() = (%+v, %v), want internal relative-link success", selection, err)
				}
				data, readErr := os.ReadFile(filepath.Join(project, "storage", "intents", "active-intent"))
				if readErr != nil || string(data) != dirName+"\n" {
					t.Errorf("linked active-intent = (%q, %v), want selected directory", data, readErr)
				}
			} else {
				if selection != (IntentSelection{}) || err == nil {
					t.Errorf("SwitchIntent() = (%+v, %v), want boundary failure", selection, err)
				}
				if !maps.Equal(before, snapshotSpaceTree(t, project)) {
					t.Error("rejected child link changed the project")
				}
			}
			if !maps.Equal(outsideBefore, snapshotSpaceTree(t, outside)) {
				t.Error("intent switch changed the outside tree")
			}
		})
	}
}

func TestSwitchIntentInitialProjectLink(t *testing.T) {
	t.Parallel()

	project, _, dirName := intentSwitchIntegrationFixture(t)
	base := t.TempDir()
	link := filepath.Join(base, "project-link")
	createSpaceSymlink(t, project, link)
	selection, err := SwitchIntent(RootInput{ExplicitDir: link}, dirName)
	if err != nil || selection != (IntentSelection{SpaceName: "default", DirName: dirName}) {
		t.Errorf("SwitchIntent() = (%+v, %v), want initial project-link success", selection, err)
	}
}

func intentSwitchIntegrationFixture(t *testing.T) (project, intentsRoot, dirName string) {
	t.Helper()

	project = t.TempDir()
	dirName = "240901-build-auth"
	intentsRoot = filepath.Join(project, "aidlc", "spaces", "default", "intents")
	writeSpaceFixture(
		t,
		project,
		nil,
		map[string]string{
			"aidlc/active-space": "default\n",
			"aidlc/spaces/default/intents/" + dirName + "/aidlc-state.md": "state",
		},
	)
	return project, intentsRoot, dirName
}
