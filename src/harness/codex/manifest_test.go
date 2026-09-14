package codex_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/core"
	"github.com/sori883/ai-dd/src/harness/codex"
)

func TestDistribution(t *testing.T) {
	t.Parallel()
	assets, err := codex.Distribution("/fixed/project", "/opt/aidlc's binary")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, asset := range assets {
		if _, ok := files[asset.Path]; ok {
			t.Fatalf("duplicate asset %s", asset.Path)
		}
		files[asset.Path] = asset.Data
		if strings.Contains(asset.Path, "shared/") || strings.HasSuffix(asset.Path, ".tmpl") || bytes.Contains(asset.Data, []byte("{{include")) || bytes.Contains(asset.Data, []byte("@@")) {
			t.Fatalf("unexpanded source: %s", asset.Path)
		}
	}
	required := []string{".agents/skills/aidlc/SKILL.md", ".agents/skills/aidlc-cli/SKILL.md", ".codex/hooks.json", "aidlc/templates/adr.md", "aidlc/workflow/stage-graph.json"}
	for _, stage := range []string{"initialization", "discovery", "planning", "architecture-analysis", "tdd", "integration"} {
		required = append(required, "aidlc/workflow/stages/"+stage+".md")
	}
	for _, name := range []string{"index.md", "adr/index.md", "codekb/index.md", "design/index.md", "rules/entry.md", "rules/rule.md"} {
		required = append(required, "aidlc/spaces/default/knowledge/"+name)
	}
	upstream := []struct{ name, repository string }{
		{"architecture", "owainlewis/blueprint"}, {"code-review", "mattpocock/skills"},
		{"domain-modeling", "mattpocock/skills"}, {"grill-with-docs", "mattpocock/skills"},
		{"grilling", "mattpocock/skills"}, {"natural-japanese-go", "coji/natural-japanese"},
		{"okf-agent-memory", "okf-memory/okf-agent-memory"}, {"planning", "mblode/agent-skills"},
		{"research", "mattpocock/skills"}, {"systematic-debugging", "obra/superpowers"},
		{"tdd", "mattpocock/skills"}, {"to-spec", "mattpocock/skills"},
		{"verification-before-completion", "obra/superpowers"},
	}
	for _, skill := range upstream {
		base := ".agents/skills/" + skill.name + "/"
		required = append(required, base+"SKILL.md", base+"LICENSE", base+"references/source.md")
		body := string(files[base+"SKILL.md"])
		if !strings.Contains(body, "name: "+skill.name+"\n") {
			t.Errorf("wrong frontmatter: %s", skill.name)
		}
		source := string(files[base+"references/source.md"])
		if !strings.Contains(source, "https://github.com/"+skill.repository+"/") || !strings.Contains(source, "原典") || (!strings.Contains(source, "調整") && !strings.Contains(source, "翻案")) {
			t.Errorf("missing attribution/adaptation: %s", skill.name)
		}
		for _, resource := range []string{"LICENSE", "references/source.md"} {
			want, err := fs.ReadFile(core.Files, "skills/"+skill.name+"/"+resource)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(files[base+resource], want) {
				t.Errorf("changed original: %s%s", base, resource)
			}
		}
		old := "aidlc-" + skill.name
		if skill.name == "okf-agent-memory" {
			old = "aidlc-okf"
		}
		stale := regexp.MustCompile(regexp.QuoteMeta(old) + `([^a-zA-Z0-9-]|$)`)
		for name, raw := range files {
			if strings.HasPrefix(name, ".agents/skills/"+old+"/") || ((strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".toml")) && stale.Match(raw)) {
				t.Errorf("old skill reference: %s: %s", name, old)
			}
		}
	}
	for _, resource := range []string{"code-review/references/review.md", "planning/references/handoff.md", "tdd/references/testing.md", "natural-japanese-go/references/cli.md", "natural-japanese-go/references/writing.md", "natural-japanese-go/licenses/kagome.txt", "natural-japanese-go/licenses/kagome-dict.txt", "natural-japanese-go/licenses/uni.txt", "natural-japanese-go/licenses/UniDic-NOTICE.txt", "natural-japanese-go/licenses/natural-japanese.txt"} {
		name := ".agents/skills/" + resource
		required = append(required, name)
		if strings.Contains(resource, "/licenses/") {
			want, err := fs.ReadFile(core.Files, "skills/"+resource)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(files[name], want) {
				t.Errorf("changed license: %s", name)
			}
		}
	}
	for _, name := range []string{"aidlc-requirements", "aidlc-researcher", "aidlc-reviewer", "aidlc-stage-planner", "aidlc-worker"} {
		file := ".codex/agents/" + name + ".toml"
		required = append(required, file)
		settings, encoded, ok := strings.Cut(string(files[file]), "developer_instructions = ")
		var instructions string
		if !ok || json.Unmarshal([]byte(strings.TrimSpace(encoded)), &instructions) != nil || instructions == "" {
			t.Errorf("invalid instructions: %s", file)
		}
		sandbox := "read-only"
		if name == "aidlc-worker" {
			sandbox = "workspace-write"
		}
		if !strings.Contains(settings, `sandbox_mode = "`+sandbox+`"`) {
			t.Errorf("wrong permissions: %s", file)
		}
	}
	for _, name := range required {
		if len(files[name]) == 0 {
			t.Errorf("missing/empty asset %s", name)
		}
	}
	if !json.Valid(files[".codex/hooks.json"]) {
		t.Error("invalid hook JSON")
	}
	if _, ok := files["aidlc/templates/kdr.md"]; ok {
		t.Error("obsolete KDR template")
	}
	links := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	for name, raw := range files {
		if !strings.HasPrefix(name, ".agents/skills/") || !strings.HasSuffix(name, ".md") {
			continue
		}
		for _, match := range links.FindAllStringSubmatch(string(raw), -1) {
			link := match[1]
			if strings.Contains(link, "://") || strings.HasPrefix(link, "#") {
				continue
			}
			target := path.Clean(path.Join(path.Dir(name), strings.Split(link, "#")[0]))
			if !strings.HasPrefix(target, ".agents/skills/") || len(files[target]) == 0 {
				t.Errorf("unresolved link: %s -> %s", name, link)
			}
		}
	}
}
