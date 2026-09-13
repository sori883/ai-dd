package codex

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	"github.com/sori883/ai-dd/src/core"
	coreworkflow "github.com/sori883/ai-dd/src/core/workflow"
	"github.com/sori883/ai-dd/src/harness"
)

func contentAssets(common, host fs.FS, binary string) ([]harness.Asset, error) {
	return splitContentAssets(common, host, SiblingBinaries(binary))
}
func splitContentAssets(common, host fs.FS, b Binaries) ([]harness.Asset, error) {
	fragments := make(map[string]string)
	err := fs.WalkDir(host, "skills", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(host, name)
		if err != nil {
			return err
		}
		fragments[strings.TrimSuffix(strings.TrimPrefix(name, "skills/"), ".md")] = string(data)
		return nil
	})
	if err != nil {
		return nil, err
	}
	var assets []harness.Asset
	err = fs.WalkDir(common, "skills", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "skills/shared" {
			return fs.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		data, err := core.RenderContent(common, name, fragments)
		if err != nil {
			return err
		}
		data, err = b.replace(data)
		if err != nil {
			return err
		}
		destination := ".agents/" + strings.TrimSuffix(name, ".tmpl")
		assets = append(assets, harness.Asset{Path: destination, Data: data})
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = fs.WalkDir(common, "agents", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(name, ".md.tmpl") {
			return fmt.Errorf("unexpected agent source %q", name)
		}
		body, err := core.RenderContent(common, name, fragments)
		if err != nil {
			return err
		}
		body, err = b.replace(body)
		if err != nil {
			return err
		}
		agent := strings.TrimSuffix(strings.TrimPrefix(name, "agents/"), ".md.tmpl")
		settings, err := fs.ReadFile(host, "agents/"+agent+".toml")
		if err != nil {
			return err
		}
		title, instructions, ok := strings.Cut(string(body), "\n\n")
		if !ok || !strings.HasPrefix(title, "# ") || strings.TrimSpace(strings.TrimPrefix(title, "# ")) == "" {
			return fmt.Errorf("missing common role description in %q", name)
		}
		encodedName, _ := json.Marshal(agent)
		encodedDescription, _ := json.Marshal(strings.TrimPrefix(title, "# "))
		// JSON strings are a subset of TOML basic strings, including control escapes.
		quoted, err := json.Marshal(instructions)
		if err != nil {
			return err
		}
		data := append([]byte("name = "+string(encodedName)+"\ndescription = "+string(encodedDescription)+"\n"+string(settings)+"developer_instructions = "), quoted...)
		data = append(data, '\n')
		assets = append(assets, harness.Asset{Path: ".codex/agents/" + agent + ".toml", Data: data})
		return nil
	})
	if err != nil {
		return nil, err
	}
	workflow, err := coreworkflow.Render(common, fragments)
	if err != nil {
		return nil, err
	}
	for name, data := range workflow {
		data, err = b.replace(data)
		if err != nil {
			return nil, err
		}
		assets = append(assets, harness.Asset{Path: "aidlc/workflow/" + name, Data: data})
	}
	return assets, nil
}
