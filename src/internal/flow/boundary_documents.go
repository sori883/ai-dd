package flow

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"os"
	"strings"
)

type boundaryCollector struct {
	proof    []FileVersion
	contents map[string][]byte
	store    Store
	files    []FileVersion
	sources  []FileVersion
	failures []string
}

func (c *boundaryCollector) require(ok bool, message string) {
	if !ok {
		c.failures = append(c.failures, message)
	}
}
func (s Store) documentPath(st State, kind string) string {
	prefix := "aidlc/spaces/" + s.Space + "/knowledge/"
	return prefix + map[string]string{"Rule": "rules/rule.md", "Requirements": "design/" + st.ID + "/requirements.md", "ImplementationPlan": "design/" + st.ID + "/implementation-plan.md", "CurrentAnalysis": "knowledge/current-analysis.md", "Architecture": "knowledge/architecture.md"}[kind]
}
func (c *boundaryCollector) file(name string) ([]byte, bool) {
	if !safeEvidencePath(name) {
		c.require(false, "unsafe input path: "+name)
		return nil, false
	}
	if raw, ok := c.contents[name]; ok {
		c.recordFile(name, raw)
		return raw, true
	}
	root, err := os.OpenRoot(c.store.Root)
	if err != nil {
		c.require(false, "unreadable input: "+name)
		return nil, false
	}
	info, statErr := root.Lstat(name)
	closeErr := root.Close()
	if statErr != nil || closeErr != nil || !info.Mode().IsRegular() {
		c.require(false, "input must be regular: "+name)
		return nil, false
	}
	raw, err := filestore.ReadFile(c.store.Root, name)
	if err != nil {
		c.require(false, "unreadable input: "+name)
		return nil, false
	}
	if c.contents == nil {
		c.contents = map[string][]byte{}
	}
	c.contents[name] = raw
	c.recordFile(name, raw)
	return raw, true
}
func (c *boundaryCollector) recordFile(name string, raw []byte) {
	for _, f := range c.files {
		if f.Path == name {
			return
		}
	}
	c.files = append(c.files, FileVersion{name, fmt.Sprintf("%x", sha256.Sum256(raw))})
}

func (c *boundaryCollector) document(st State, name, kind string, optional bool) {
	if optional {
		root, err := os.OpenRoot(c.store.Root)
		if err == nil {
			_, statErr := root.Lstat(name)
			closeErr := root.Close()
			if closeErr != nil {
				c.require(false, closeErr.Error())
				return
			}
			if os.IsNotExist(statErr) {
				return
			}
		}
	}
	raw, ok := c.file(name)
	if !ok {
		return
	}
	doc, err := okfmemory.Parse(raw)
	if err != nil {
		c.require(false, "invalid OKF: "+name)
		return
	}
	c.require(doc.String("type") == kind && strings.TrimSpace(doc.String("title")) != "" && strings.TrimSpace(doc.String("description")) != "" && strings.TrimSpace(doc.Body) != "", "document metadata/body mismatch: "+name)
	if kind == "Requirements" || kind == "ImplementationPlan" {
		c.require(doc.String("intent_id") == st.ID, "document Intent mismatch: "+name)
	}
	sections := map[string][]string{"Requirements": {"目的", "範囲", "要件", "受入条件", "未確定事項"}, "ImplementationPlan": {"変更箇所", "実装手順", "検証方法"}, "CurrentAnalysis": {"現状", "構成・動作", "根拠", "未確認事項"}, "Architecture": {"構成図", "構成要素", "データフロー"}, "Knowledge": {"機能", "利用手順", "制約"}}[kind]
	bodies := documentSections(doc.Body)
	for _, heading := range sections {
		c.require(strings.TrimSpace(bodies[heading]) != "", "missing or empty section "+heading+": "+name)
	}
	if kind == "Architecture" {
		body := bodies["構成図"]
		parts := strings.Split(body, "```mermaid\n")
		valid := false
		if len(parts) > 1 {
			end := strings.Index(parts[1], "```")
			valid = end >= 0 && strings.TrimSpace(parts[1][:end]) != ""
		}
		c.require(valid, "nonempty Mermaid diagram required: "+name)
	}
}
func documentSections(body string) map[string]string {
	out := map[string]string{}
	heading := ""
	fence := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fence = !fence
		}
		if !fence && strings.HasPrefix(line, "## ") {
			heading = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			continue
		}
		if heading != "" {
			out[heading] += line + "\n"
		}
	}
	return out
}
func (c *boundaryCollector) gate(stage string) Gate {
	raw, _ := json.Marshal(struct {
		Stage string
		Files []FileVersion
	}{stage, c.files})
	g := Gate{Target: fmt.Sprintf("%x", sha256.Sum256(raw)), Status: "pass", Summary: "boundary requirements satisfied"}
	if len(c.failures) > 0 {
		g.Status = "fail"
		g.Summary = strings.Join(c.failures, "; ")
	}
	return g
}
func (c *boundaryCollector) accepted(st State, stage, name string) {
	a, ok := st.Accepted[stage]
	c.require(ok, "accepted "+stage+" required")
	raw, good := c.file(name)
	if !good {
		return
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(raw))
	match := false
	for _, f := range a.Outputs {
		if f.Path == name && f.SHA256 == hash {
			match = true
		}
	}
	c.require(match, "accepted input changed; reopen "+stage+": "+name)
}
func (s Store) startState(st State) (Gate, []FileVersion, []FileVersion) {
	c := boundaryCollector{store: s}
	c.require(st.Status == "active", "Intent is not active")
	c.document(st, s.documentPath(st, "Rule"), "Rule", false)
	for _, kind := range []string{"CurrentAnalysis", "Architecture"} {
		c.document(st, s.documentPath(st, kind), kind, true)
	}
	if stageOrder[st.Stage] >= 1 {
		name := s.documentPath(st, "Requirements")
		c.document(st, name, "Requirements", false)
		c.accepted(st, "discovery", name)
	}
	if stageOrder[st.Stage] >= 2 {
		name := s.documentPath(st, "ImplementationPlan")
		c.document(st, name, "ImplementationPlan", false)
		c.accepted(st, "planning", name)
		for _, name := range st.Config.ADR.Refs {
			c.document(st, name, "ADR", false)
		}
	}
	if st.Stage == "integration" {
		a, ok := st.Accepted["tdd"]
		c.require(ok, "accepted tdd required")
		for _, f := range a.Outputs {
			c.accepted(st, "tdd", f.Path)
		}
	}
	sources := boundaryCollector{store: s}
	for _, name := range st.Config.MaterialSources {
		sources.material(name)
	}
	c.failures = append(c.failures, sources.failures...)
	inputs := append([]FileVersion(nil), c.files...)
	c.files = append(c.files, sources.files...)
	return c.gate(st.Stage), inputs, sources.files
}
