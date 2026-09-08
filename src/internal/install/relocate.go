package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	codex "github.com/sori883/ai-dd/src/harness/codex/minimal"
	"github.com/sori883/ai-dd/src/internal/filestore"
)

// RelocationResult separates committed files from files still requiring retry.
type RelocationResult struct{ Paths, Pending []string }

// Relocate updates only known references. Source paths are never accessed.
func Relocate(root, binary, fromRoot, fromBinary string) (RelocationResult, error) {
	return relocate(root, binary, fromRoot, fromBinary, filestore.WriteFile)
}
func relocate(root, binary, fromRoot, fromBinary string, write func(string, string, []byte) error) (result RelocationResult, err error) {
	for _, p := range []string{root, binary, fromRoot, fromBinary} {
		if !filepath.IsAbs(p) || strings.ContainsRune(p, 0) {
			return result, fmt.Errorf("absolute paths required: %w", fs.ErrInvalid)
		}
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return result, err
	}
	paths := []string{".agents/skills/aidlc/SKILL.md", ".codex/hooks.json"}
	before := make([][]byte, 2)
	after := make([][]byte, 2)
	for i, p := range paths {
		before[i], err = filestore.ReadFile(root, p)
		if err != nil {
			return result, fmt.Errorf("%s: %w", p, err)
		}
	}
	template, err := fs.ReadFile(codex.Files, "SKILL.md")
	if err != nil {
		return result, err
	}
	oldSkill := []byte(strings.ReplaceAll(string(template), "@@BINARY@@", shellQuote(fromBinary)))
	newSkill := []byte(strings.ReplaceAll(string(template), "@@BINARY@@", shellQuote(binary)))
	if !bytes.Equal(before[0], oldSkill) && !bytes.Equal(before[0], newSkill) {
		return result, fmt.Errorf("%s: unknown asset bytes: %w", paths[0], fs.ErrInvalid)
	}
	after[0] = newSkill
	after[1], err = relocateHooks(before[1], shellQuote(fromBinary)+" __minimal-hook --project-dir "+shellQuote(fromRoot), shellQuote(binary)+" __minimal-hook --project-dir "+shellQuote(root))
	if err != nil {
		return result, fmt.Errorf("%s: %w", paths[1], err)
	}
	for i, p := range paths {
		if !bytes.Equal(before[i], after[i]) {
			result.Pending = append(result.Pending, p)
		}
	}
	release, err := filestore.Lock(root, "install-relocate")
	if err != nil {
		return result, err
	}
	defer release()
	for i, p := range paths {
		current, readErr := filestore.ReadFile(root, p)
		if readErr != nil {
			return result, fmt.Errorf("%s: %w", p, readErr)
		}
		if !bytes.Equal(current, before[i]) {
			return result, fmt.Errorf("%s: concurrent asset change: %w", p, fs.ErrInvalid)
		}
		if bytes.Equal(before[i], after[i]) {
			continue
		}
		if err := write(root, p, after[i]); err != nil {
			return result, fmt.Errorf("%s: %w", p, err)
		}
		result.Paths = append(result.Paths, p)
		result.Pending = result.Pending[1:]
	}
	return result, nil
}

type relocationNode struct {
	start, end int
	value      any
	object     map[string]*relocationNode
	array      []*relocationNode
}

func relocationJSON(raw []byte) (*relocationNode, error) {
	if !utf8.Valid(raw) || len(raw) > filestore.MaxBytes {
		return nil, fmt.Errorf("invalid UTF-8 or oversized JSON: %w", fs.ErrInvalid)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var read func(int) (*relocationNode, error)
	read = func(depth int) (*relocationNode, error) {
		if depth > 64 {
			return nil, fmt.Errorf("JSON nesting too deep: %w", fs.ErrInvalid)
		}
		start := int(d.InputOffset())
		for start < len(raw) && strings.ContainsRune(" \r\n\t,:", rune(raw[start])) {
			start++
		}
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		n := &relocationNode{start: start, value: token}
		if token == json.Delim('{') {
			n.object = map[string]*relocationNode{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return nil, err
				}
				s, ok := key.(string)
				if !ok {
					return nil, fmt.Errorf("invalid key")
				}
				if _, ok := n.object[s]; ok {
					return nil, fmt.Errorf("duplicate JSON key %q: %w", s, fs.ErrInvalid)
				}
				child, err := read(depth + 1)
				if err != nil {
					return nil, err
				}
				n.object[s] = child
			}
			if _, err := d.Token(); err != nil {
				return nil, err
			}
		}
		if token == json.Delim('[') {
			n.array = []*relocationNode{}
			for d.More() {
				child, err := read(depth + 1)
				if err != nil {
					return nil, err
				}
				n.array = append(n.array, child)
			}
			if _, err := d.Token(); err != nil {
				return nil, err
			}
		}
		n.end = int(d.InputOffset())
		return n, nil
	}
	n, err := read(0)
	if err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON data: %w", fs.ErrInvalid)
	}
	return n, nil
}
func nodeValue(n *relocationNode) any {
	if n == nil {
		return nil
	}
	return n.value
}
func relocateHooks(raw []byte, old, new string) ([]byte, error) {
	root, err := relocationJSON(raw)
	if err != nil {
		return nil, err
	}
	fail := func(message string) ([]byte, error) { return nil, fmt.Errorf("%s: %w", message, fs.ErrInvalid) }
	if root.object == nil || root.object["hooks"] == nil || root.object["hooks"].object == nil {
		return fail("hooks must be object")
	}
	events := map[string]bool{"SessionStart": true, "UserPromptSubmit": true, "PreToolUse": true, "PostToolUse": true, "Stop": true}
	counts := map[string]int{}
	var replacements []*relocationNode
	for event, groups := range root.object["hooks"].object {
		if groups.array == nil {
			return fail("event groups must be array")
		}
		for _, group := range groups.array {
			if group.object == nil || group.object["hooks"] == nil || group.object["hooks"].array == nil {
				return fail("handler group must contain hooks array")
			}
			for _, handler := range group.object["hooks"].array {
				if handler.object == nil {
					return fail("handler must be object")
				}
				command := handler.object["command"]
				s, _ := nodeValue(command).(string)
				if s != old && s != new {
					if strings.Contains(s, "__minimal-hook") {
						return fail("unknown product command")
					}
					continue
				}
				if !events[event] {
					return fail("product handler on unknown event")
				}
				counts[event]++
				if nodeValue(handler.object["type"]) != "command" || nodeValue(handler.object["timeout"]) != json.Number("10") {
					return fail("product handler type or timeout mismatch")
				}
				if event == "SessionStart" {
					if nodeValue(handler.object["additionalContextLimit"]) != json.Number("8192") {
						return fail("product context limit mismatch")
					}
				} else if handler.object["additionalContextLimit"] != nil {
					return fail("unexpected product context limit")
				}
				if event == "PreToolUse" || event == "PostToolUse" {
					if nodeValue(group.object["matcher"]) != "^(Bash|apply_patch)$" {
						return fail("product matcher mismatch")
					}
				} else if group.object["matcher"] != nil {
					return fail("unexpected product matcher")
				}
				if s != new {
					replacements = append(replacements, command)
				}
			}
		}
	}
	for event := range events {
		if counts[event] != 1 {
			return fail("missing or duplicate product event " + event)
		}
	}
	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
	result := bytes.Clone(raw)
	encoded, _ := json.Marshal(new)
	for _, n := range replacements {
		result = append(append(append([]byte{}, result[:n.start]...), encoded...), result[n.end:]...)
	}
	return result, nil
}
