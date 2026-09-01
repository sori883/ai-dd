//go:build integration && (darwin || linux)

package workspace

import (
	"context"
	"errors"
	"io/fs"
	"maps"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCreateIntentIntegrationRejectsRegistrySpecialFileWithoutMutation(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	writeSpaceFixture(t, project, []string{"aidlc/spaces/team/intents"}, map[string]string{
		"keep": "unchanged",
	})
	registryPath := filepath.Join(project, "aidlc", "spaces", "team", "intents", "intents.json")
	if err := syscall.Mkfifo(registryPath, 0o600); err != nil {
		t.Fatalf("create registry FIFO fixture: %v", err)
	}
	before := snapshotSpaceTree(t, project)
	created, err := CreateIntent(
		context.Background(),
		RootInput{ExplicitDir: project},
		IntentCreateInput{SpaceName: "team", Label: "Build Auth"},
	)
	if created != (CreatedIntent{}) || !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("CreateIntent() = (%+v, %v), want zero and fs.ErrInvalid", created, err)
	}
	if !maps.Equal(before, snapshotSpaceTree(t, project)) {
		t.Error("rejected registry special file changed the project")
	}
	assertWorkspaceLockAbsent(t, project)
}
