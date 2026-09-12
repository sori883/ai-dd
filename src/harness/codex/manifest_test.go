package codex_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/sori883/ai-dd/src/harness/codex"
)

func TestDistribution(t *testing.T) {
	t.Parallel()
	// This is the old installer's independently captured baseline, not renderer output.
	raw, err := os.ReadFile("../../internal/install/testdata/codex-assets-sha256.json")
	if err != nil {
		t.Fatal(err)
	}
	var want []struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	assets, err := codex.Distribution("/fixed/project", "/opt/aidlc's binary")
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != len(want) {
		t.Fatalf("distribution has %d files, want %d", len(assets), len(want))
	}
	for i, asset := range assets {
		sum := sha256.Sum256(asset.Data)
		if asset.Path != want[i].Path || hex.EncodeToString(sum[:]) != want[i].SHA256 {
			t.Errorf("asset %d (%s) differs from old installer", i, asset.Path)
		}
	}
}
