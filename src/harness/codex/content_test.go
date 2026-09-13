package codex

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/core"
)

func TestContentDistribution(t *testing.T) {
	assets, err := contentAssets(core.Files, Files, "/opt/aidlc")
	if err != nil {
		t.Fatal(err)
	}
	skills, agents := 0, 0
	for _, asset := range assets {
		if strings.HasSuffix(asset.Path, "/SKILL.md") {
			skills++
		}
		if strings.HasPrefix(asset.Path, ".codex/agents/") {
			agents++
		}
		if strings.Contains(asset.Path, "shared/") || strings.HasSuffix(asset.Path, ".tmpl") || strings.Contains(string(asset.Data), "{{include") {
			t.Fatalf("internal source deployed: %s", asset.Path)
		}
	}
	if skills != 15 || agents != 5 {
		t.Fatalf("got %d skills and %d agents, want 15 and 5", skills, agents)
	}
}

func TestContentSharedAgentContractAndTOML(t *testing.T) {
	sources := fstest.MapFS{}
	if err := fs.WalkDir(core.Files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(core.Files, name)
		if err == nil {
			sources[name] = &fstest.MapFile{Data: data}
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	// Independent literal: quote, backslash and a TOML triple delimiter must remain text.
	const contract = "共有契約を変更: \"\"\"quoted\"\"\" C:\\tmp\n次行"
	sources["skills/shared/agent-contract.md"] = &fstest.MapFile{Data: []byte(contract)}
	assets, err := contentAssets(sources, Files, "/opt/aidlc")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, asset := range assets {
		if !strings.HasPrefix(asset.Path, ".codex/agents/") {
			continue
		}
		count++
		// The encoder uses a TOML basic string: no raw newline or delimiter can escape it.
		if !strings.Contains(string(asset.Data), `共有契約を変更: \"\"\"quoted\"\"\" C:\\tmp\n次行`) {
			t.Fatalf("%s lost shared contract or TOML escaping: %s", asset.Path, asset.Data)
		}
		if !strings.Contains(string(asset.Data), "send_message") {
			t.Fatalf("missing Codex report connection: %s", asset.Path)
		}
	}
	if count != 5 {
		t.Fatalf("shared contract reached %d agents, want 5", count)
	}
}

func TestContentRoleDescriptionComesFromCommonSource(t *testing.T) {
	files := fstest.MapFS{}
	if err := fs.WalkDir(core.Files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(core.Files, name)
		if err == nil {
			files[name] = &fstest.MapFile{Data: data}
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	name := "agents/aidlc-worker.md.tmpl"
	files[name].Data = append([]byte("# 共通の役割説明\n\n"), files[name].Data...)
	assets, err := contentAssets(files, Files, "/opt/aidlc")
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range assets {
		if asset.Path == ".codex/agents/aidlc-worker.toml" {
			if !strings.Contains(string(asset.Data), "description = \"共通の役割説明\"") {
				t.Fatalf("role description did not come from common source: %s", asset.Data)
			}
			return
		}
	}
	t.Fatal("missing worker")
}
