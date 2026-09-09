// Package workflow reads the deployed, versioned four-stage definition.
package workflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

type Stage struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Procedure string `json:"procedure"`
}
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}
type Reopen struct {
	Current   bool `json:"allow_current"`
	Ancestors bool `json:"allow_ancestors"`
}
type Graph struct {
	SchemaVersion int     `json:"schema_version"`
	Start         string  `json:"start_stage"`
	Completion    string  `json:"completion_stage"`
	Stages        []Stage `json:"stages"`
	Advance       []Edge  `json:"advance"`
	Reopen        Reopen  `json:"reopen"`
}
type Agent struct {
	Role  string `yaml:"role" json:"role"`
	Agent string `yaml:"agent" json:"agent"`
}
type Reference struct {
	Path         string `yaml:"path" json:"path,omitempty"`
	Refs         string `yaml:"refs" json:"refs,omitempty"`
	Role         string `yaml:"role" json:"role,omitempty"`
	Version      string `yaml:"version" json:"version,omitempty"`
	AcceptedAt   string `yaml:"accepted_at" json:"accepted_at,omitempty"`
	RequiredWhen string `yaml:"required_when" json:"required_when,omitempty"`
}
type Sensors struct {
	Start string `yaml:"start" json:"start"`
	End   string `yaml:"end" json:"end"`
}
type Procedure struct {
	StageID string      `yaml:"stage_id" json:"stage_id"`
	Agents  []Agent     `yaml:"agents" json:"agents"`
	Inputs  []Reference `yaml:"inputs" json:"inputs"`
	Outputs []Reference `yaml:"outputs" json:"outputs"`
	Sensors Sensors     `yaml:"sensors" json:"sensors"`
	Text    string      `yaml:"-" json:"text"`
	Path    string      `yaml:"-" json:"path"`
}
type Definition struct {
	Graph      Graph
	Procedures map[string]Procedure
	Hash       string
}

const GraphPath = "aidlc/workflow/stage-graph.json"

func Load(root string) (Definition, error) {
	d := Definition{Procedures: map[string]Procedure{}}
	raw, err := read(root, GraphPath)
	if err != nil {
		return d, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if uniqueJSON(json.NewDecoder(bytes.NewReader(raw))) != nil || decoder.Decode(&d.Graph) != nil || decoder.Decode(new(any)) != io.EOF {
		return d, fmt.Errorf("invalid workflow graph")
	}
	if err = d.validateGraph(); err != nil {
		return d, err
	}
	h := sha256.New()
	fmt.Fprintf(h, "%d:%s%d:", len(GraphPath), GraphPath, len(raw))
	h.Write(raw)
	for _, stage := range d.Graph.Stages {
		name := "aidlc/workflow/" + stage.Procedure
		raw, err = read(root, name)
		if err != nil {
			return d, err
		}
		p, err := parseProcedure(stage, raw)
		if err != nil {
			return d, fmt.Errorf("%s: %w", name, err)
		}
		for _, ref := range p.Inputs {
			if ref.Version == "accepted" && !d.Before(ref.AcceptedAt, stage.ID) {
				return d, fmt.Errorf("accepted input must precede current stage")
			}
		}
		d.Procedures[stage.ID] = p
		fmt.Fprintf(h, "%d:%s%d:", len(name), name, len(raw))
		h.Write(raw)
	}
	d.Hash = fmt.Sprintf("%x", h.Sum(nil))
	return d, nil
}
func read(root, name string) ([]byte, error) {
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	parts := strings.Split(name, "/")
	for i := range parts {
		info, err := r.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink workflow: %s", name)
		}
		if i == len(parts)-1 && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("nonregular workflow: %s", name)
		}
	}
	f, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 256*1024+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 256*1024 || !utf8.Valid(raw) {
		return nil, fmt.Errorf("invalid workflow size or encoding: %s", name)
	}
	return raw, nil
}
func (d Definition) Next(stage string) string {
	for _, edge := range d.Graph.Advance {
		if edge.From == stage {
			return edge.To
		}
	}
	return ""
}
func (d Definition) Before(a, b string) bool {
	for next := d.Next(a); next != ""; next = d.Next(next) {
		if next == b {
			return true
		}
	}
	return false
}
func (d Definition) CanReopen(from, to string) bool {
	return from == to && d.Graph.Reopen.Current || d.Graph.Reopen.Ancestors && d.Before(to, from)
}
func (d Definition) validateGraph() error {
	g := d.Graph
	if g.SchemaVersion != 1 || len(g.Stages) != 4 || len(g.Advance) != 3 {
		return fmt.Errorf("unsupported workflow graph")
	}
	ids := map[string]bool{}
	paths := map[string]bool{}
	for _, s := range g.Stages {
		supported := s.ID == "discovery" || s.ID == "planning" || s.ID == "tdd" || s.ID == "integration"
		if !supported || ids[s.ID] || strings.TrimSpace(s.Name) == "" || !fs.ValidPath(s.Procedure) || !strings.HasPrefix(s.Procedure, "stages/") || path.Ext(s.Procedure) != ".md" || paths[s.Procedure] || strings.Contains(s.Procedure, "\\") {
			return fmt.Errorf("invalid stage or procedure path")
		}
		ids[s.ID] = true
		paths[s.Procedure] = true
	}
	outgoing := map[string]bool{}
	incoming := map[string]bool{}
	for _, e := range g.Advance {
		if !ids[e.From] || !ids[e.To] || outgoing[e.From] || incoming[e.To] {
			return fmt.Errorf("invalid graph edge")
		}
		outgoing[e.From] = true
		incoming[e.To] = true
	}
	seen := map[string]bool{}
	for at := g.Start; at != ""; at = d.Next(at) {
		if seen[at] || !ids[at] {
			return fmt.Errorf("cyclic or missing stage")
		}
		seen[at] = true
	}
	if len(seen) != 4 || g.Start != "discovery" || g.Completion != "integration" || d.Next(g.Completion) != "" {
		return fmt.Errorf("unreachable completion")
	}
	if !d.Before("discovery", "planning") || !d.Before("planning", "tdd") || !d.Before("tdd", "integration") {
		return fmt.Errorf("reversed required artifact prerequisites")
	}
	return nil
}
func parseProcedure(stage Stage, raw []byte) (Procedure, error) {
	var p Procedure
	if !bytes.HasPrefix(raw, []byte("---\n")) {
		return p, fmt.Errorf("frontmatter required")
	}
	end := bytes.Index(raw[4:], []byte("\n---\n"))
	if end < 0 || end > 64*1024 {
		return p, fmt.Errorf("invalid frontmatter boundary")
	}
	metadata := raw[4 : 4+end]
	body := raw[4+end+5:]
	var node yaml.Node
	if yaml.Unmarshal(metadata, &node) != nil || safeYAML(&node) != nil {
		return p, fmt.Errorf("invalid YAML")
	}
	dec := yaml.NewDecoder(bytes.NewReader(metadata))
	dec.KnownFields(true)
	if dec.Decode(&p) != nil || dec.Decode(new(any)) != io.EOF {
		return p, fmt.Errorf("unknown or invalid frontmatter field")
	}
	if p.StageID != stage.ID || strings.TrimSpace(string(body)) == "" || p.Inputs == nil || p.Outputs == nil || p.Agents == nil || p.Sensors.Start != stage.ID+"-start" || p.Sensors.End != stage.ID+"-end" {
		return p, fmt.Errorf("incomplete procedure")
	}
	agents := map[string]string{"research": "aidlc-researcher", "requirements": "aidlc-requirements", "implementation": "aidlc-worker", "independent_review": "aidlc-reviewer"}
	seen := map[string]bool{}
	for _, a := range p.Agents {
		expected, known := agents[a.Role]
		if !known || expected != a.Agent || seen[a.Role] {
			return p, fmt.Errorf("invalid agent role")
		}
		seen[a.Role] = true
	}
	for _, r := range p.Inputs {
		if err := validateReference(r, false); err != nil {
			return p, err
		}
	}
	for _, r := range p.Outputs {
		if err := validateReference(r, true); err != nil {
			return p, err
		}
	}
	p.Text = string(raw)
	p.Path = "aidlc/workflow/" + stage.Procedure
	return p, nil
}
func validateReference(r Reference, output bool) error {
	if (r.Path == "") == (r.Refs == "") {
		return fmt.Errorf("one document path or refs required")
	}
	if r.Refs != "" && r.Refs != "config.adr.refs" && r.Refs != "config.feature_knowledge" {
		return fmt.Errorf("unsupported reference")
	}
	if r.Path != "" {
		name := strings.ReplaceAll(strings.ReplaceAll(r.Path, "${knowledge_root}", "knowledge"), "${intent_id}", "intent")
		if !strings.HasPrefix(r.Path, "${knowledge_root}/") || !fs.ValidPath(name) || strings.ContainsAny(name, "$\\") || path.Ext(name) != ".md" {
			return fmt.Errorf("document path outside Knowledge")
		}
	}
	switch r.RequiredWhen {
	case "", "always", "exists", "adr_required", "materials_present":
	default:
		return fmt.Errorf("unsupported required_when")
	}
	if r.RequiredWhen == "adr_required" && r.Refs != "config.adr.refs" {
		return fmt.Errorf("ADR condition requires ADR refs")
	}
	if output {
		if r.Version != "" || r.AcceptedAt != "" || strings.TrimSpace(r.Role) == "" {
			return fmt.Errorf("invalid output version or role")
		}
		return nil
	}
	if r.Version != "current" && r.Version != "accepted" {
		return fmt.Errorf("input version required")
	}
	if r.Version == "accepted" {
		switch r.AcceptedAt {
		case "discovery", "planning", "tdd", "integration":
		default:
			return fmt.Errorf("accepted stage required")
		}
	} else if r.AcceptedAt != "" {
		return fmt.Errorf("current input has accepted stage")
	}
	return nil
}
func safeYAML(n *yaml.Node) error {
	if n.Kind == yaml.AliasNode || n.Anchor != "" || n.Style&yaml.TaggedStyle != 0 || n.Tag == "!!merge" {
		return fmt.Errorf("YAML alias, merge or tag forbidden")
	}
	keys := map[string]bool{}
	for i, c := range n.Content {
		if n.Kind == yaml.MappingNode && i%2 == 0 {
			if c.Kind != yaml.ScalarNode || keys[c.Value] {
				return fmt.Errorf("duplicate YAML key")
			}
			keys[c.Value] = true
		}
		if err := safeYAML(c); err != nil {
			return err
		}
	}
	return nil
}
func uniqueJSON(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	keys := map[string]bool{}
	for d.More() {
		if delim == '{' {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || keys[name] {
				return fmt.Errorf("duplicate JSON key")
			}
			keys[name] = true
		}
		if err := uniqueJSON(d); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}
