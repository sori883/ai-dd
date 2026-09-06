package sensor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/sori883/ai-dd/src/internal/artifact"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/state"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

const maxSensorContextBytes = 8 << 20

// BuildIntentCaptureInput is the single conductor-owned input builder for the
// fixed intent-capture sensor set. It deliberately derives authority context
// from the opened project and record roots rather than accepting caller
// supplied digests, timestamps, or receipt metadata.
func BuildIntentCaptureInput(projectRoot, recordRoot *os.Root, stage graph.Stage) (Input, error) {
	if projectRoot == nil || recordRoot == nil {
		return Input{}, fmt.Errorf("build sensor input: roots are required")
	}
	input := Input{
		Stage:        stage.Slug,
		ArtifactPath: path.Join(stage.Phase, stage.Slug, artifact.Filename(stage.ReviewArtifact)),
		Consumes:     make([]string, 0, len(stage.Consumes)),
	}
	for _, consume := range stage.Consumes {
		if value := strings.TrimSpace(consume.Artifact); value != "" {
			input.Consumes = append(input.Consumes, value)
		}
	}

	// The state and project-description sidecar are authority inputs. A
	// damaged or unreadable authority file is retained as a finding so an
	// advisory sensor cannot turn an unavailable context into a pass.
	if document, err := state.ReadDocument(recordRoot); err != nil {
		input.AuthorityFindings = append(input.AuthorityFindings, Finding{Path: "aidlc-state.md", Message: "state context is unavailable: " + err.Error()})
	} else {
		input.Scope = document.State.Scope()
	}
	if content, err := readSensorLeaf(recordRoot, "project-description.json"); err != nil {
		input.AuthorityFindings = append(input.AuthorityFindings, Finding{Path: "project-description.json", Message: "project description context is unavailable: " + err.Error()})
	} else {
		var description string
		if err := json.Unmarshal(content, &description); err != nil {
			input.AuthorityFindings = append(input.AuthorityFindings, Finding{Path: "project-description.json", Message: "project description context is not a JSON string: " + err.Error()})
		} else {
			resolved, pasted, resolveErr := authoritativeDescription(description)
			input.PastedDocumentPresent = pasted
			if resolveErr != nil {
				input.AuthorityFindings = append(input.AuthorityFindings, Finding{Path: "project-description.json", Message: "project description context is ambiguous: " + resolveErr.Error()})
			} else {
				input.ProjectDescription = resolved
			}
		}
	}
	if input.ProjectDescription == "" {
		input.AuthorityFindings = append(input.AuthorityFindings, Finding{Path: "project-description.json", Message: "authoritative project directions are missing"})
	}
	if input.Scope == "" {
		input.AuthorityFindings = append(input.AuthorityFindings, Finding{Path: "aidlc-state.md", Message: "Scope authority is missing"})
	}

	activeSpace := workspace.ActiveSpace(projectRoot.FS())
	if content, err := readSensorLeaf(projectRoot, "aidlc/active-space"); err != nil {
		input.AuthorityFindings = append(input.AuthorityFindings, Finding{Path: "aidlc/active-space", Message: "active space context is unavailable: " + err.Error()})
	} else if strings.TrimSpace(string(content)) != "" {
		activeSpace = strings.TrimSpace(string(content))
	}
	if !validSensorComponent(activeSpace) {
		input.AuthorityFindings = append(input.AuthorityFindings, Finding{Path: "aidlc/active-space", Message: "active space is not a single safe path component"})
		activeSpace = ""
	}
	if activeSpace == "" {
		input.AuthorityFindings = append(input.AuthorityFindings, Finding{Path: "aidlc/active-space", Message: "active space is missing"})
	}
	input.ActiveSpace = activeSpace
	input.MemorySources = make(map[string][]string)
	for _, name := range []string{"org.md", "team.md", "project.md"} {
		if activeSpace == "" {
			break
		}
		memoryPath := path.Join("aidlc", "spaces", activeSpace, "memory", name)
		content, err := readSensorLeaf(projectRoot, memoryPath)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				input.AuthorityFindings = append(input.AuthorityFindings, Finding{Path: memoryPath, Message: "memory context is unavailable: " + err.Error()})
			}
			continue
		}
		if sources := parseMemorySources(memoryPath, content); len(sources) != 0 {
			if input.MemorySources == nil {
				input.MemorySources = make(map[string][]string)
			}
			for key, rules := range sources {
				input.MemorySources[key] = rules
			}
		}
	}

	outputNames := append(append([]string(nil), stage.Produces...), stage.OptionalProduces...)
	for _, name := range outputNames {
		relative := path.Join(stage.Phase, stage.Slug, artifact.Filename(name))
		if sensorScaffoldingPath(relative) {
			continue
		}
		output := Deliverable{Path: relative}
		content, err := readSensorLeaf(recordRoot, relative)
		if err != nil {
			input.AuthorityFindings = append(input.AuthorityFindings, Finding{Path: relative, Message: "deliverable is unavailable: " + err.Error()})
		} else {
			output.Content = content
			input.Deliverables = append(input.Deliverables, append([]byte(nil), content...))
			if relative == input.ArtifactPath {
				input.Content = append([]byte(nil), content...)
			}
		}
		input.OutputFiles = append(input.OutputFiles, output)
	}
	for _, output := range input.OutputFiles {
		base := path.Base(output.Path)
		if !strings.HasSuffix(base, ".md") || sensorScaffoldingPath(output.Path) {
			continue
		}
		key := strings.TrimSuffix(base, ".md")
		teamPath := path.Join("aidlc", "spaces", activeSpace, "memory", "templates", base)
		if activeSpace != "" {
			if content, err := readSensorLeaf(projectRoot, teamPath); err == nil {
				if input.TeamTemplates == nil {
					input.TeamTemplates = make(map[string][]byte)
				}
				input.TeamTemplates[key] = content
				continue
			} else if !errors.Is(err, fs.ErrNotExist) {
				input.AuthorityFindings = append(input.AuthorityFindings, Finding{Path: teamPath, Message: "team template is unavailable: " + err.Error()})
				continue
			}
		}
		frameworkPath := path.Join(".codex", "tools", "data", "templates", base)
		if content, err := readSensorLeaf(projectRoot, frameworkPath); err == nil {
			if input.FrameworkTemplates == nil {
				input.FrameworkTemplates = make(map[string][]byte)
			}
			input.FrameworkTemplates[key] = content
		} else if !errors.Is(err, fs.ErrNotExist) {
			input.AuthorityFindings = append(input.AuthorityFindings, Finding{Path: frameworkPath, Message: "framework template is unavailable: " + err.Error()})
		}
	}
	questionsPath := path.Join(stage.Phase, stage.Slug, artifact.Filename("intent-capture-questions"))
	input.QuestionsPath = questionsPath
	if content, err := readSensorLeaf(recordRoot, questionsPath); err != nil {
		input.AuthorityFindings = append(input.AuthorityFindings, Finding{Path: questionsPath, Message: "questions context is unavailable: " + err.Error()})
	} else {
		input.Questions = content
	}
	return input, nil
}

func authoritativeDescription(raw string) (description string, pasted bool, err error) {
	const open, close = "<document>", "</document>"
	start := strings.Index(raw, open)
	strayClose := strings.Index(raw, close)
	if start < 0 {
		if strayClose >= 0 {
			return "", false, fmt.Errorf("%s has no matching %s", close, open)
		}
		return strings.TrimSpace(raw), false, nil
	}
	if strayClose >= 0 && strayClose < start {
		return "", false, fmt.Errorf("%s appears before the next %s", close, open)
	}
	end := strings.Index(raw[start+len(open):], close)
	if end < 0 {
		return "", true, fmt.Errorf("%s has no matching %s", open, close)
	}
	end += start + len(open)
	nested := strings.Index(raw[start+len(open):end], open)
	if nested >= 0 {
		return "", true, fmt.Errorf("nested %s blocks are not allowed", open)
	}
	trailing := raw[end+len(close):]
	if strings.Contains(trailing, open) || strings.Contains(trailing, close) || strings.TrimSpace(trailing) != "" {
		return "", true, fmt.Errorf("content follows terminal %s", close)
	}
	return strings.TrimSpace(raw[:start]), true, nil
}

func validSensorComponent(value string) bool {
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, "/\\") {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' && char != '_' && char != '.' {
			return false
		}
	}
	return true
}

func sensorScaffoldingPath(relative string) bool {
	base := path.Base(relative)
	return base == "memory.md" || strings.HasSuffix(base, "-questions.md") || strings.HasSuffix(base, "-timestamp.md")
}

func readSensorLeaf(root *os.Root, name string) (content []byte, err error) {
	if root == nil || name == "" || !fs.ValidPath(name) || path.IsAbs(name) || strings.Contains(name, "\\") {
		return nil, fmt.Errorf("unsafe sensor context path %q: %w", name, fs.ErrInvalid)
	}
	pathInfo, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if pathInfo == nil || pathInfo.Mode()&fs.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("sensor context %q is not a regular file: %w", name, fs.ErrInvalid)
	}
	file, err := openSensorContextLeaf(root, name)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, fmt.Errorf("sensor context %q opened nil: %w", name, fs.ErrInvalid)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close sensor context %q: %w", name, closeErr))
		}
	}()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if opened == nil || opened.Mode()&fs.ModeSymlink != 0 || !opened.Mode().IsRegular() || !os.SameFile(pathInfo, opened) {
		return nil, fmt.Errorf("sensor context %q changed identity before read: %w", name, fs.ErrInvalid)
	}
	content, err = io.ReadAll(io.LimitReader(file, maxSensorContextBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxSensorContextBytes || !utf8.Valid(content) {
		return nil, fmt.Errorf("sensor context %q is oversized or invalid UTF-8: %w", name, fs.ErrInvalid)
	}
	final, err := file.Stat()
	if err != nil {
		return nil, err
	}
	current, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if final == nil || current == nil || final.Mode()&fs.ModeSymlink != 0 || current.Mode()&fs.ModeSymlink != 0 || !final.Mode().IsRegular() || !current.Mode().IsRegular() || !os.SameFile(pathInfo, final) || !os.SameFile(pathInfo, current) {
		return nil, fmt.Errorf("sensor context %q changed identity during read: %w", name, fs.ErrInvalid)
	}
	return content, nil
}

func parseMemorySources(memoryPath string, content []byte) map[string][]string {
	if !utf8.Valid(content) {
		return nil
	}
	result := make(map[string][]string)
	heading := ""
	headingCounts := make(map[string]int)
	for _, raw := range visibleMarkdownLines(string(content)) {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "## ") {
			heading = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			headingCounts[heading]++
			continue
		}
		if heading == "" || line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ">") {
			continue
		}
		match := memoryListItemPattern.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, match[0]))
		if line == "" {
			continue
		}
		key := memoryPath + "#" + heading
		result[key] = append(result[key], line)
	}
	for key := range result {
		heading := strings.TrimPrefix(key, memoryPath+"#")
		if headingCounts[heading] != 1 {
			delete(result, key)
		}
	}
	return result
}

var memoryListItemPattern = regexp.MustCompile(`^(?:[-*+]|[0-9]{1,9}[.)])\s+`)
