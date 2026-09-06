// Package sensor contains the advisory checks used by the intent-capture
// gate. Sensor output is presentation and audit evidence only; it is never
// an authority input to a gate decision.
package sensor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

const (
	ClaimSourcesID     = "claim-sources"
	RequiredSectionsID = "required-sections"
	UpstreamCoverageID = "upstream-coverage"

	StatusFired       Status = "FIRED"
	StatusCompleted   Status = "COMPLETED"
	StatusFailed      Status = "FAILED"
	StatusUnavailable Status = "UNAVAILABLE"
)

// Status is the non-authoritative result state of one sensor invocation.
type Status string

// Input is the bounded artifact context supplied to an advisory check.
// Callers do not supply fire ids, timestamps, or receipt positions.
type Input struct {
	Stage        string
	ArtifactPath string
	Content      []byte
	// ProjectDescription and Scope are optional authority context resolved by
	// the conductor. When present, source-register entries must match them
	// exactly; the sensor never derives authority from a caller-provided
	// digest or receipt.
	ProjectDescription    string
	Scope                 string
	PastedDocumentPresent bool
	ActiveSpace           string
	// MemorySources optionally supplies the authoritative visible rules keyed
	// by their canonical `path#heading` reference. An omitted map keeps the
	// source-register syntax check while a supplied map also verifies the
	// referenced rule exactly.
	MemorySources map[string][]string
	// OutputFiles is the path-preserving stage deliverable set. Scaffolding
	// files such as questions and timestamps are intentionally excluded by the
	// conductor before sensors inspect this set.
	OutputFiles []Deliverable
	// AuthorityFindings are advisory diagnostics from fresh context reads. A
	// missing authority context must remain observable without becoming gate
	// authority.
	AuthorityFindings []Finding
	// TeamTemplates and FrameworkTemplates are the already safely-read
	// template candidates. A team template wins over the framework template
	// for the same eligible artifact.
	TeamTemplates      map[string][]byte
	FrameworkTemplates map[string][]byte
	// Deliverables and Questions are optional stage-level context. The runner
	// uses them to evaluate the fixed Intent Capture checks as a set; callers
	// cannot use them to mint sensor identity or authority.
	Deliverables  [][]byte
	QuestionsPath string
	Questions     []byte
	Consumes      []string
}

// Deliverable is one path-preserving stage output supplied to a sensor.
type Deliverable struct {
	Path    string
	Content []byte
}

// Finding is a presentation-only diagnostic returned by a sensor.
type Finding struct {
	Path    string
	Message string
}

// CheckResult is the output a check implementation may provide. Identity and
// authority fields are intentionally absent; RunAll supplies those itself.
type CheckResult struct {
	Status   Status
	Detail   string
	Note     string
	Findings []Finding
}

// CheckFunc permits tests and future local implementations to replace one
// check. The runner still controls the fixed sensor set and all fire ids.
type CheckFunc func(Input) (CheckResult, error)

// Result is one fire or terminal sensor record. FireID is private so callers
// cannot mint a receipt identity; the runner creates it once per invocation.
type Result struct {
	Stage         string
	Sensor        string
	Status        Status
	Detail        string
	Note          string
	OutputPath    string
	DetailPath    string
	DurationMS    int64
	FindingsCount int
	Findings      []Finding
	Terminal      bool
	fireID        string
}

// FireID returns the runner-generated eight-hex invocation identity.
func (r Result) FireID() string { return r.fireID }

// Invocation pairs a non-authoritative FIRED record with an optional
// terminal result. A missing terminal result is represented by a zero Result,
// never by a synthetic pass.
type Invocation struct {
	fired    Result
	terminal *Result
	err      error
}

// FireResult returns the invocation's generated FIRED record.
func (i Invocation) FireResult() Result { return cloneResult(i.fired) }

// TerminalResult returns the terminal result, or a zero result when the
// sensor did not produce one.
func (i Invocation) TerminalResult() Result {
	if i.terminal == nil {
		return Result{}
	}
	return cloneResult(*i.terminal)
}

// HasTerminal reports whether this invocation yielded a terminal result.
func (i Invocation) HasTerminal() bool { return i.terminal != nil }

// Error returns the local execution error, if any.
func (i Invocation) Error() error { return i.err }

var fallbackFireID atomic.Uint32

// RunAll attempts all three fixed intent-capture advisory sensors in graph
// order. A failed or unavailable check is collected and does not change gate
// authorization. The optional map is an internal/test seam keyed by sensor
// id; omitted entries use the standard-library checks below.
func RunAll(ctx context.Context, input Input, checks map[string]CheckFunc) []Invocation {
	if ctx == nil {
		ctx = context.Background()
	}
	ids := []string{ClaimSourcesID, RequiredSectionsID, UpstreamCoverageID}
	results := make([]Invocation, 0, len(ids))
	for _, id := range ids {
		fireID := newFireID()
		fired := Result{Stage: input.Stage, Sensor: id, Status: StatusFired, OutputPath: input.ArtifactPath, Terminal: false, fireID: fireID}
		check := defaultCheck(id)
		if checks != nil && checks[id] != nil {
			check = checks[id]
		}
		var terminal *Result
		var checkErr error
		startedAt := time.Now()
		if err := context.Cause(ctx); err != nil {
			checkErr = err
			value := Result{Stage: input.Stage, Sensor: id, Status: StatusUnavailable, Detail: err.Error(), OutputPath: input.ArtifactPath, DurationMS: sensorDurationMillis(startedAt), Terminal: true, fireID: fireID}
			terminal = &value
		} else {
			value, err := check(input)
			if err != nil {
				checkErr = err
				value = CheckResult{Status: StatusCompleted, Detail: value.Detail, Note: "script-error: " + err.Error(), Findings: value.Findings}
			}
			if value.Status != "" {
				result := Result{Stage: input.Stage, Sensor: id, Status: value.Status, Detail: value.Detail, Note: value.Note, OutputPath: input.ArtifactPath, DurationMS: sensorDurationMillis(startedAt), FindingsCount: len(value.Findings), Findings: cloneFindings(value.Findings), Terminal: true, fireID: fireID}
				if value.Status == StatusFailed {
					result.DetailPath = path.Join(".aidlc-sensors", input.Stage, id+"-"+fireID+".md")
				}
				terminal = &result
			}
		}
		results = append(results, Invocation{fired: fired, terminal: terminal, err: checkErr})
	}
	return results
}

func sensorDurationMillis(start time.Time) int64 {
	duration := time.Since(start).Milliseconds()
	if duration < 1 {
		return 1
	}
	return duration
}

// AdvisoryOutcomesAuthorizeGate always returns false: sensor output cannot
// mint the required authority evidence or make a gate ready.
func AdvisoryOutcomesAuthorizeGate(_ []Invocation) bool { return false }

// AdvisoryOutcomesBlockGate always returns false: advisory sensor failures,
// missing terminals, and findings are non-blocking observations.
func AdvisoryOutcomesBlockGate(_ []Invocation) bool { return false }

func defaultCheck(id string) CheckFunc {
	switch id {
	case ClaimSourcesID:
		return runClaimSources
	case RequiredSectionsID:
		return runRequiredSections
	case UpstreamCoverageID:
		return runUpstreamCoverage
	default:
		return func(Input) (CheckResult, error) { return CheckResult{}, errors.New("unknown sensor") }
	}
}

func runClaimSources(input Input) (CheckResult, error) {
	universe := parseClaimSourceUniverse(input.Questions, input)
	deliverables := sensorDeliverables(input)
	findings := append([]Finding(nil), input.AuthorityFindings...)
	findings = append(findings, universe.findings...)
	hasAssumptions := false
	for _, deliverable := range deliverables {
		content := deliverable.Content
		claimPath := deliverable.Path
		if claimPath == "" {
			claimPath = input.ArtifactPath
		}
		if sensorScaffoldingPath(claimPath) {
			continue
		}
		if !utf8.Valid(content) {
			findings = append(findings, Finding{Path: claimPath, Message: "deliverable is not valid UTF-8"})
			continue
		}
		claims, hasAssumptionsSection := visibleClaimBlocks(string(content))
		for _, claim := range claims {
			line := claim.Text
			section := claim.Section
			if section == assumptionsHeading && isNoneBlock(line) {
				continue
			}
			if section == assumptionsHeading {
				hasAssumptions = true
			}
			tags := visibleSourceTags(line, claim.ReferenceLabels)
			if len(tags) == 0 {
				findings = append(findings, Finding{Path: claimPath, Message: "claim block has no source tag: " + line})
				continue
			}
			if section == assumptionsHeading && !strings.Contains(line, "[assumption]") {
				findings = append(findings, Finding{Path: claimPath, Message: "assumption/open-question block has no [assumption] tag"})
			}
			for _, tag := range tags {
				switch {
				case strings.HasPrefix(tag, "Q"):
					if !universe.answeredQuestions[tag] {
						findings = append(findings, Finding{Path: claimPath, Message: "[" + tag + "] has no filled answer"})
					}
				case tag == "assumption":
					if section != assumptionsHeading {
						findings = append(findings, Finding{Path: claimPath, Message: "[assumption] is outside ## " + assumptionsHeading})
					} else if universe.assumptionsAccepted && !assumptionAccepted(universe, line) {
						findings = append(findings, Finding{Path: claimPath, Message: "retained assumption is not listed in ## Assumption Confirmation"})
					}
				case tag == "scope":
					if universe.registered[tag] == "" {
						findings = append(findings, Finding{Path: claimPath, Message: "[scope] is not registered in ## Sources"})
					}
					if section != "Initial Scope Signal" || !strings.Contains(strings.ToLower(line), "workflow-selected") {
						findings = append(findings, Finding{Path: claimPath, Message: "[scope] is valid only for a workflow-selected Initial Scope Signal"})
					}
				case tag == "desc" && input.PastedDocumentPresent:
					findings = append(findings, Finding{Path: claimPath, Message: "[desc] cannot ground a pasted-document request; use a confirmed [Q<n>]"})
				default:
					if universe.registered[tag] == "" {
						findings = append(findings, Finding{Path: claimPath, Message: "[" + tag + "] is not registered in ## Sources"})
					}
				}
			}
		}
		if !hasAssumptionsSection {
			findings = append(findings, Finding{Path: claimPath, Message: "missing ## " + assumptionsHeading})
		}
	}
	if hasAssumptions && !universe.assumptionsAccepted {
		findings = append(findings, Finding{Path: input.ArtifactPath, Message: "retained assumptions require an answered ## Assumption Confirmation with Accept assumptions"})
	}
	if len(findings) != 0 {
		return CheckResult{Status: StatusFailed, Detail: fmt.Sprintf("claim source validation found %d finding(s)", len(findings)), Findings: findings}, nil
	}
	return CheckResult{Status: StatusCompleted, Detail: "claim sources verified"}, nil
}

type visibleClaimBlock struct {
	Text            string
	Section         string
	ReferenceLabels map[string]struct{}
}

func visibleClaimBlocks(body string) ([]visibleClaimBlock, bool) {
	lines := visibleMarkdownLines(body)
	references := make(map[string]struct{})
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if label, ok := referenceDefinitionLabel(line); ok {
			references[strings.ToLower(label)] = struct{}{}
		}
	}
	blocks := make([]visibleClaimBlock, 0)
	section := ""
	review := false
	hasAssumptionsSection := false
	pending := make([]string, 0, 2)
	flush := func() {
		text := strings.TrimSpace(strings.Join(pending, "\n"))
		if text != "" {
			blocks = append(blocks, visibleClaimBlock{Text: text, Section: section, ReferenceLabels: references})
		}
		pending = pending[:0]
	}
	for index, raw := range lines {
		line := strings.TrimSpace(raw)
		if heading, ok := sensorH2Heading(raw); ok {
			flush()
			section = heading
			review = heading == "Review"
			if heading == assumptionsHeading {
				hasAssumptionsSection = true
			}
			continue
		}
		if review || line == "" || strings.HasPrefix(line, "#") || isMarkdownTableSeparator(line) || isMarkdownTableHeader(lines, index) || isReferenceDefinition(line) {
			flush()
			continue
		}
		if strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") {
			flush()
			blocks = append(blocks, visibleClaimBlock{Text: line, Section: section, ReferenceLabels: references})
			continue
		}
		if strings.HasPrefix(line, ">") {
			line = strings.TrimSpace(strings.TrimPrefix(line, ">"))
		}
		if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "+") || numberedListPattern.MatchString(line) {
			flush()
		}
		pending = append(pending, line)
	}
	flush()
	return blocks, hasAssumptionsSection
}

func visibleMarkdownLines(body string) []string {
	return VisibleMarkdownLines(body)
}

// VisibleMarkdownLines returns the line-preserving visible Markdown view used
// by the deterministic sensors. Fenced code, indented code, and HTML
// comments are blanked without changing line indexes, so callers can safely
// reason about rendered headings and list items without treating examples as
// authority.
func VisibleMarkdownLines(body string) []string {
	lines := strings.Split(strings.TrimPrefix(strings.ReplaceAll(body, "\r\n", "\n"), "\ufeff"), "\n")
	visible := make([]string, len(lines))
	var fence markdownFence
	comment := false
	for index, raw := range lines {
		line := stripHTMLComments(raw, &comment)
		if comment && strings.TrimSpace(line) == "" {
			continue
		}
		if fence.length != 0 {
			if markdownFenceClosed(line, fence) {
				fence = markdownFence{}
			}
			continue
		}
		if marker, ok := markdownFenceOpening(line); ok {
			fence = marker
			continue
		}
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ") {
			continue
		}
		if strings.TrimSpace(line) != "" {
			visible[index] = line
		}
	}
	return visible
}

type markdownFence struct {
	marker byte
	length int
}

func markdownFenceOpening(line string) (markdownFence, bool) {
	indent := len(line) - len(strings.TrimLeft(line, " "))
	if indent > 3 {
		return markdownFence{}, false
	}
	line = line[indent:]
	if len(line) < 3 || (line[0] != '`' && line[0] != '~') {
		return markdownFence{}, false
	}
	length := 0
	for length < len(line) && line[length] == line[0] {
		length++
	}
	if length < 3 {
		return markdownFence{}, false
	}
	return markdownFence{marker: line[0], length: length}, true
}

func markdownFenceClosed(line string, fence markdownFence) bool {
	indent := len(line) - len(strings.TrimLeft(line, " "))
	if indent > 3 {
		return false
	}
	line = line[indent:]
	if len(line) < fence.length || line[0] != fence.marker {
		return false
	}
	length := 0
	for length < len(line) && line[length] == fence.marker {
		length++
	}
	return length >= fence.length && strings.TrimSpace(line[length:]) == ""
}

func stripHTMLComments(line string, inComment *bool) string {
	if inComment == nil {
		return line
	}
	var visible strings.Builder
	for offset := 0; offset < len(line); {
		if *inComment {
			end := strings.Index(line[offset:], "-->")
			if end < 0 {
				return visible.String()
			}
			offset += end + len("-->")
			*inComment = false
			continue
		}
		start := strings.Index(line[offset:], "<!--")
		if start < 0 {
			visible.WriteString(line[offset:])
			break
		}
		visible.WriteString(line[offset : offset+start])
		offset += start + len("<!--")
		*inComment = true
	}
	return visible.String()
}

func isMarkdownTableHeader(lines []string, index int) bool {
	if index+1 >= len(lines) || !strings.Contains(lines[index], "|") {
		return false
	}
	next := strings.TrimSpace(lines[index+1])
	return isMarkdownTableSeparator(next)
}

func visibleSourceTags(line string, references map[string]struct{}) []string {
	line = stripInlineCode(line)
	line = inlineLinkedSourcePattern.ReplaceAllString(line, " ")
	line = linkedReferencePattern.ReplaceAllString(line, " ")
	line = referenceShortcutPattern.ReplaceAllStringFunc(line, func(value string) string {
		label := strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
		if _, ok := references[strings.ToLower(strings.TrimSpace(label))]; ok {
			return " "
		}
		return value
	})
	matches := sourceTagPattern.FindAllStringSubmatch(line, -1)
	tags := make([]string, 0, len(matches))
	for _, match := range matches {
		tags = append(tags, match[1])
	}
	return tags
}

func stripInlineCode(line string) string {
	for start := 0; start < len(line); {
		relative := strings.IndexByte(line[start:], '`')
		if relative < 0 {
			return line
		}
		start += relative
		run := 1
		for start+run < len(line) && line[start+run] == '`' {
			run++
		}
		end := -1
		for search := start + run; search < len(line); {
			candidate := strings.IndexByte(line[search:], '`')
			if candidate < 0 {
				break
			}
			search += candidate
			closeRun := 1
			for search+closeRun < len(line) && line[search+closeRun] == '`' {
				closeRun++
			}
			if closeRun == run {
				end = search + closeRun
				break
			}
			search += closeRun
		}
		if end < 0 {
			return line[:start]
		}
		line = line[:start] + strings.Repeat(" ", end-start) + line[end:]
		start += end - start
	}
	return line
}

var (
	referenceDefinitionPattern   = regexp.MustCompile(`^ {0,3}\[([^]]+)\]:`)
	markdownListContainerPattern = regexp.MustCompile(`^(?:[-*+]|[0-9]{1,9}[.)])(?:[ \t]+|$)`)
	linkedReferencePattern       = regexp.MustCompile(`\[(?:desc|scope|assumption|Q[0-9]+|memory:[A-Za-z0-9][A-Za-z0-9._-]*)\]\s*\[[^]]*\]`)
	inlineLinkedSourcePattern    = regexp.MustCompile(`\[(?:desc|scope|assumption|Q[0-9]+|memory:[A-Za-z0-9][A-Za-z0-9._-]*)\]\s*\([^)]*\)`)
	referenceShortcutPattern     = regexp.MustCompile(`\[(?:desc|scope|assumption|Q[0-9]+|memory:[A-Za-z0-9][A-Za-z0-9._-]*)\]`)
	numberedListPattern          = regexp.MustCompile(`^[0-9]{1,9}[.)]\s+`)
)

func stripMarkdownContainers(line string) string {
	line = strings.TrimSpace(line)
	for line != "" {
		if strings.HasPrefix(line, ">") {
			line = strings.TrimSpace(strings.TrimPrefix(line, ">"))
			continue
		}
		if marker := markdownListContainerPattern.FindString(line); marker != "" {
			line = strings.TrimSpace(strings.TrimPrefix(line, marker))
			continue
		}
		break
	}
	return line
}

func referenceDefinitionLabel(line string) (string, bool) {
	line = stripMarkdownContainers(line)
	match := referenceDefinitionPattern.FindStringSubmatch(line)
	if len(match) != 2 {
		return "", false
	}
	return strings.TrimSpace(match[1]), true
}

func isReferenceDefinition(line string) bool {
	_, ok := referenceDefinitionLabel(line)
	return ok
}

func sensorDeliverables(input Input) []Deliverable {
	if len(input.OutputFiles) != 0 {
		return append([]Deliverable(nil), input.OutputFiles...)
	}
	if len(input.Deliverables) != 0 {
		result := make([]Deliverable, 0, len(input.Deliverables))
		for _, content := range input.Deliverables {
			result = append(result, Deliverable{Path: input.ArtifactPath, Content: content})
		}
		return result
	}
	return []Deliverable{{Path: input.ArtifactPath, Content: input.Content}}
}

func runRequiredSections(input Input) (CheckResult, error) {
	deliverables := sensorDeliverables(input)
	findings := append([]Finding(nil), input.AuthorityFindings...)
	checked := 0
	totalSections := 0
	for _, deliverable := range deliverables {
		relative := deliverable.Path
		if relative == "" {
			relative = input.ArtifactPath
		}
		if sensorScaffoldingPath(relative) {
			continue
		}
		checked++
		if !utf8.Valid(deliverable.Content) {
			findings = append(findings, Finding{Path: relative, Message: "artifact is not valid UTF-8"})
			continue
		}
		if strings.TrimSpace(string(deliverable.Content)) == "" {
			findings = append(findings, Finding{Path: relative, Message: "artifact has no sections"})
			continue
		}
		sections := markdownH2Set(string(deliverable.Content))
		totalSections += len(sections)
		template, hasTemplate := requiredSectionsTemplate(input, relative)
		if !hasTemplate && len(sections) < 2 {
			findings = append(findings, Finding{Path: relative, Message: fmt.Sprintf("artifact has %d distinct H2 section(s); add at least two H2 headings", len(sections))})
		}
		if hasTemplate {
			if !utf8.Valid(template) {
				findings = append(findings, Finding{Path: relative, Message: "required-sections template is not valid UTF-8"})
				continue
			}
			for heading := range markdownH2Set(string(template)) {
				if _, present := sections[heading]; !present {
					findings = append(findings, Finding{Path: relative, Message: "template-required H2 section is missing: " + heading})
				}
			}
		}
	}
	if checked == 0 {
		findings = append(findings, Finding{Path: input.ArtifactPath, Message: "artifact has no template-eligible deliverable"})
	}
	if len(findings) != 0 {
		return CheckResult{Status: StatusFailed, Detail: fmt.Sprintf("required section validation found %d finding(s)", len(findings)), Findings: findings}, nil
	}
	return CheckResult{Status: StatusCompleted, Detail: fmt.Sprintf("detected %d H2 section(s)", totalSections)}, nil
}

func markdownH2Set(content string) map[string]struct{} {
	sections := make(map[string]struct{})
	for _, line := range visibleMarkdownLines(content) {
		if heading, ok := sensorH2Heading(line); ok && heading != "" {
			sections[heading] = struct{}{}
		}
	}
	return sections
}

func requiredSectionsTemplate(input Input, relative string) ([]byte, bool) {
	keys := []string{relative, path.Base(relative), strings.TrimSuffix(path.Base(relative), path.Ext(relative))}
	for _, key := range keys {
		if content, ok := input.TeamTemplates[key]; ok {
			return content, true
		}
	}
	for _, key := range keys {
		if content, ok := input.FrameworkTemplates[key]; ok {
			return content, true
		}
	}
	return nil, false
}

func runUpstreamCoverage(input Input) (CheckResult, error) {
	if len(input.Consumes) == 0 {
		if len(input.AuthorityFindings) != 0 {
			return CheckResult{Status: StatusFailed, Detail: "sensor authority context is incomplete", Findings: append([]Finding(nil), input.AuthorityFindings...)}, nil
		}
		return CheckResult{Status: StatusCompleted, Detail: "no upstream consumes declared"}, nil
	}
	deliverables := sensorDeliverables(input)
	body := make([]string, 0, len(deliverables))
	for _, deliverable := range deliverables {
		if sensorScaffoldingPath(deliverable.Path) {
			continue
		}
		body = append(body, string(deliverable.Content))
	}
	joined := strings.Join(body, "\n")
	missing := append([]Finding(nil), input.AuthorityFindings...)
	for _, rawConsume := range input.Consumes {
		consume, producer := parseConsume(rawConsume)
		if consume == "" {
			continue
		}
		covered := standaloneSlugPattern(consume).MatchString(joined) || strings.Contains(strings.ToLower(joined), "[["+strings.ToLower(consume)+"]]") || strings.Contains(strings.ToLower(joined), "`"+strings.ToLower(consume)+".md`")
		if !covered && producer != "" {
			covered = producerDirectoryReferenced(joined, producer)
		}
		if !covered {
			missing = append(missing, Finding{Path: input.ArtifactPath, Message: "upstream artifact is not referenced: " + consume})
		}
	}
	if len(missing) != 0 {
		return CheckResult{Status: StatusFailed, Detail: fmt.Sprintf("%d upstream artifact(s) are unreferenced", len(missing)), Findings: missing}, nil
	}
	return CheckResult{Status: StatusCompleted, Detail: "upstream coverage verified"}, nil
}

func parseConsume(value string) (slug, producer string) {
	value = strings.TrimSpace(value)
	if index := strings.IndexByte(value, ':'); index >= 0 {
		return strings.TrimSpace(value[:index]), strings.TrimSpace(value[index+1:])
	}
	return value, ""
}

func producerDirectoryReferenced(body, producer string) bool {
	producer = strings.TrimSpace(producer)
	if producer == "" {
		return false
	}
	for start := 0; start < len(body); {
		index := strings.Index(strings.ToLower(body[start:]), strings.ToLower(producer))
		if index < 0 {
			return false
		}
		index += start
		beforeOK := index == 0 || !isSlugCharacter(body[index-1])
		after := index + len(producer)
		afterOK := after < len(body) && body[after] == '/'
		trailingOK := index > 0 && body[index-1] == '/' && (after == len(body) || !isSlugCharacter(body[after]))
		if (beforeOK && afterOK) || trailingOK {
			return true
		}
		start = index + 1
	}
	return false
}

func isSlugCharacter(value byte) bool {
	return value == '-' || value == '_' || (value >= '0' && value <= '9') || (value >= 'A' && value <= 'Z') || (value >= 'a' && value <= 'z')
}

var sourceTagPattern = regexp.MustCompile(`\[(desc|scope|assumption|Q[0-9]+|memory:[A-Za-z0-9][A-Za-z0-9._-]*)\]`)

var (
	h2LinePattern        = regexp.MustCompile(`^ {0,3}##(?:[ \t]+|$)(.*)$`)
	sourceEntryPattern   = regexp.MustCompile(`^ {0,3}[-*+]\s+\[(desc|scope|memory:[A-Za-z0-9][A-Za-z0-9._-]*)\]\s+(.+?)\s*$`)
	answerLinePattern    = regexp.MustCompile(`^\[Answer\]:\s*(.*)$`)
	unknownTagPattern    = regexp.MustCompile(`\[([A-Za-z][A-Za-z0-9:_-]*)\]`)
	memorySourcePattern  = regexp.MustCompile("^`([^`]+)#([^`]+)`: \\\"((?:\\\\.|[^\\\"\\\\])*)\\\"$")
	assumptionsHeading   = "Assumptions & Open Questions"
	assumptionConfirming = "A. Accept assumptions"
)

type claimSourceUniverse struct {
	registered          map[string]string
	answeredQuestions   map[string]bool
	assumptionsAccepted bool
	acceptedAssumptions map[string]struct{}
	findings            []Finding
}

func parseClaimSourceUniverse(questions []byte, input Input) claimSourceUniverse {
	universe := claimSourceUniverse{
		registered:          make(map[string]string),
		answeredQuestions:   make(map[string]bool),
		acceptedAssumptions: make(map[string]struct{}),
	}
	if len(questions) == 0 {
		universe.findings = append(universe.findings, Finding{Path: input.ArtifactPath, Message: "questions file is missing the source register"})
		return universe
	}
	if !utf8.Valid(questions) {
		universe.findings = append(universe.findings, Finding{Path: input.ArtifactPath, Message: "questions file is not valid UTF-8"})
		return universe
	}
	lines := visibleMarkdownLines(string(questions))
	sections := make(map[string][][]string)
	current := ""
	var body []string
	flush := func() {
		if current != "" {
			sections[current] = append(sections[current], append([]string(nil), body...))
		}
		body = nil
	}
	for _, line := range lines {
		if heading, ok := sensorH2Heading(line); ok {
			flush()
			current = heading
			continue
		}
		body = append(body, line)
	}
	flush()

	sources := sections["Sources"]
	if len(sources) == 0 {
		universe.findings = append(universe.findings, Finding{Path: input.ArtifactPath, Message: "questions file is missing ## Sources"})
	} else {
		if len(sources) > 1 {
			universe.findings = append(universe.findings, Finding{Path: input.ArtifactPath, Message: "questions file has duplicate ## Sources sections"})
		}
		seen := make(map[string]bool)
		for _, line := range sources[0] {
			match := sourceEntryPattern.FindStringSubmatch(strings.TrimSuffix(line, "\r"))
			if match == nil {
				continue
			}
			id, value := match[1], strings.TrimSpace(match[2])
			if seen[id] {
				universe.findings = append(universe.findings, Finding{Path: input.ArtifactPath, Message: "duplicate source id [" + id + "] in ## Sources"})
			}
			seen[id] = true
			valid := true
			switch {
			case id == "desc":
				matchDescription := regexp.MustCompile(`^Initial description:\s*("(?:\\.|[^"\\])*")$`).FindStringSubmatch(value)
				parsed := ""
				if len(matchDescription) == 2 {
					parsed, _ = strconv.Unquote(matchDescription[1])
				}
				if parsed == "" && matchDescription == nil {
					valid = false
					universe.findings = append(universe.findings, Finding{Path: input.ArtifactPath, Message: "[desc] source entry is not an authoritative Initial description"})
				} else if input.ProjectDescription != "" && parsed != input.ProjectDescription {
					valid = false
					universe.findings = append(universe.findings, Finding{Path: input.ArtifactPath, Message: "[desc] does not match the authoritative project description"})
				}
			case id == "scope":
				matchScope := regexp.MustCompile("^Workflow-selected scope:\\s*`([^`]+)`\\.?$").FindStringSubmatch(value)
				scope := ""
				if len(matchScope) == 2 {
					scope = matchScope[1]
				}
				if scope == "" {
					valid = false
					universe.findings = append(universe.findings, Finding{Path: input.ArtifactPath, Message: "[scope] source entry is not a workflow-selected scope"})
				} else if input.Scope != "" && scope != input.Scope {
					valid = false
					universe.findings = append(universe.findings, Finding{Path: input.ArtifactPath, Message: "[scope] does not match the authoritative workflow scope"})
				}
			case strings.HasPrefix(id, "memory:"):
				memoryMatch := memorySourcePattern.FindStringSubmatch(value)
				if memoryMatch == nil || !validMemorySourcePath(memoryMatch[1], input.ActiveSpace) {
					valid = false
					universe.findings = append(universe.findings, Finding{Path: input.ArtifactPath, Message: "[" + id + "] memory source has invalid path or rule format"})
				} else if input.MemorySources != nil {
					rule, err := strconv.Unquote("\"" + memoryMatch[3] + "\"")
					key := memoryMatch[1] + "#" + memoryMatch[2]
					if err != nil || !memoryRulePresent(input.MemorySources[key], rule) {
						valid = false
						universe.findings = append(universe.findings, Finding{Path: input.ArtifactPath, Message: "[" + id + "] quoted rule does not match the authoritative memory source"})
					}
				}
			}
			if valid {
				universe.registered[id] = value
			}
		}
		for _, required := range []string{"desc", "scope"} {
			if !seen[required] {
				universe.findings = append(universe.findings, Finding{Path: input.ArtifactPath, Message: "## Sources is missing [" + required + "]"})
			}
		}
	}

	for id, section := range questionSections(lines) {
		answers := 0
		filled := false
		for _, line := range section {
			match := answerLinePattern.FindStringSubmatch(strings.TrimSpace(line))
			if match == nil {
				continue
			}
			answers++
			value := strings.TrimSpace(match[1])
			if value != "" && !strings.Contains(value, "TBD") && !strings.Contains(value, "Unanswered") && !strings.HasPrefix(value, "_") {
				filled = true
			}
		}
		if answers > 1 {
			universe.findings = append(universe.findings, Finding{Path: input.ArtifactPath, Message: "duplicate [Answer] entries for " + id})
		}
		if filled {
			universe.answeredQuestions[id] = true
		}
	}

	confirmation := sections["Assumption Confirmation"]
	if len(confirmation) > 1 {
		universe.findings = append(universe.findings, Finding{Path: input.ArtifactPath, Message: "questions file has duplicate ## Assumption Confirmation sections"})
	}
	if len(confirmation) > 0 {
		answer := ""
		answers := 0
		for _, line := range confirmation[0] {
			if match := answerLinePattern.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
				answers++
				answer = strings.TrimSpace(match[1])
			}
			if strings.Contains(line, "[assumption]") {
				universe.acceptedAssumptions[normalizeAssumption(line)] = struct{}{}
			}
		}
		if answers > 1 {
			universe.findings = append(universe.findings, Finding{Path: input.ArtifactPath, Message: "duplicate [Answer] entries for Assumption Confirmation"})
		}
		universe.assumptionsAccepted = answer == assumptionConfirming
	}
	return universe
}

func sensorH2Heading(line string) (string, bool) {
	match := h2LinePattern.FindStringSubmatch(strings.TrimSuffix(line, "\r"))
	if match == nil {
		return "", false
	}
	heading := strings.TrimSpace(strings.TrimRight(match[1], "# \t"))
	return heading, true
}

func questionSections(lines []string) map[string][]string {
	sections := make(map[string][]string)
	for index := 0; index < len(lines); index++ {
		heading, ok := sensorH2Heading(lines[index])
		if !ok {
			continue
		}
		match := regexp.MustCompile(`^(Q[0-9]+)\b`).FindStringSubmatch(heading)
		if match == nil {
			continue
		}
		end := index + 1
		for end < len(lines) {
			if _, nextHeading := sensorH2Heading(lines[end]); nextHeading {
				break
			}
			end++
		}
		sections[match[1]] = append(sections[match[1]], lines[index+1:end]...)
		index = end - 1
	}
	return sections
}

func normalizeAssumption(line string) string {
	line = regexp.MustCompile(`^\s*(?:[-*+]|[0-9]{1,9}[.)])\s+`).ReplaceAllString(line, "")
	line = sourceTagPattern.ReplaceAllString(line, "")
	return strings.ToLower(strings.Join(strings.Fields(line), " "))
}

func isNoneBlock(line string) bool {
	line = strings.TrimSpace(line)
	line = regexp.MustCompile(`^\s*(?:[-*+]|[0-9]{1,9}[.)])\s+`).ReplaceAllString(line, "")
	return strings.EqualFold(strings.TrimSpace(line), "None.") || strings.EqualFold(strings.TrimSpace(line), "None")
}

func assumptionAccepted(universe claimSourceUniverse, line string) bool {
	_, ok := universe.acceptedAssumptions[normalizeAssumption(line)]
	return ok
}

func validMemorySourcePath(value, activeSpace string) bool {
	pathValue := value
	if !strings.HasPrefix(pathValue, "aidlc/spaces/") || strings.Contains(pathValue, "\\") || strings.Contains(pathValue, "/../") || strings.HasSuffix(pathValue, "/..") {
		return false
	}
	pathParts := strings.Split(pathValue, "/")
	if len(pathParts) != 5 || pathParts[0] != "aidlc" || pathParts[1] != "spaces" || pathParts[3] != "memory" {
		return false
	}
	if activeSpace != "" && pathParts[2] != activeSpace {
		return false
	}
	return pathParts[4] == "org.md" || pathParts[4] == "team.md" || pathParts[4] == "project.md"
}

func memoryRulePresent(rules []string, want string) bool {
	for _, rule := range rules {
		if rule == want {
			return true
		}
	}
	return false
}

func filledQuestion(questions []byte, tag string) bool {
	if len(questions) == 0 {
		return false
	}
	for _, line := range strings.Split(string(questions), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "["+tag+"]") && !strings.Contains(line, "TBD") && !strings.Contains(line, "Unanswered") {
			return true
		}
	}
	return false
}

func isMarkdownTableSeparator(line string) bool {
	line = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "|"), "|")
	if strings.TrimSpace(line) == "" {
		return false
	}
	for _, cell := range strings.Split(line, "|") {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			return false
		}
		for _, char := range cell {
			if char != '-' && char != ':' {
				return false
			}
		}
	}
	return true
}

func standaloneSlugPattern(slug string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(?:^|[^A-Za-z0-9-])` + regexp.QuoteMeta(slug) + `(?:$|[^A-Za-z0-9-])`)
}

func newFireID() string {
	var bytes [4]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	// Randomness failure must not make a prompt or gate hang. This fallback is
	// process-generated and cannot be selected by a caller.
	return fmt.Sprintf("%08x", fallbackFireID.Add(1))
}

func cloneResult(result Result) Result {
	result.Findings = cloneFindings(result.Findings)
	return result
}

func cloneFindings(findings []Finding) []Finding {
	if findings == nil {
		return nil
	}
	return append([]Finding(nil), findings...)
}
