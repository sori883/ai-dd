package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Submodule describes one valid declaration from the workspace .gitmodules.
type Submodule struct {
	Name        string
	Path        string
	URL         string
	Initialized bool
}

// ScanResult is the read-only workspace classification returned by Detect.
type ScanResult struct {
	ProjectType string
	Languages   string
	Frameworks  string
	BuildSystem string
	NestedRoot  string
	Submodules  []Submodule
}

type workspaceScanOps struct {
	readDir  func(string) ([]fs.DirEntry, error)
	lstat    func(string) (fs.FileInfo, error)
	stat     func(string) (fs.FileInfo, error)
	readFile func(string) ([]byte, error)
}

// Detect analyzes an already-open project root without modifying or closing it.
func Detect(projectRoot *os.Root) ScanResult {
	return detectWithScanOps(workspaceScanOperations(projectRoot))
}

func workspaceScanOperations(root *os.Root) workspaceScanOps {
	return workspaceScanOps{
		readDir: func(name string) ([]fs.DirEntry, error) {
			directory, err := root.Open(name)
			if err != nil {
				return nil, err
			}
			entries, readErr := directory.ReadDir(-1)
			return entries, errors.Join(readErr, directory.Close())
		},
		lstat:    root.Lstat,
		stat:     root.Stat,
		readFile: root.ReadFile,
	}
}

func detectWithScanOps(ops workspaceScanOps) ScanResult {
	root := scanDirectorySignals(ops, ".", 0)
	languages := root.languages
	frameworkValues := slices.Clone(root.frameworks)
	buildSystem := root.buildSystem
	isBrownfield := root.isBrownfield
	nestedHits := []string{}
	if !isBrownfield {
		aggregate := scanAggregation{
			languages:   &languages,
			frameworks:  &frameworkValues,
			buildSystem: &buildSystem,
			nestedHits:  &nestedHits,
		}
		walkNestedWorkspaces(ops, ".", nil, 0, &aggregate)
		isBrownfield = len(nestedHits) > 0
	}
	submodules := scanSubmodules(ops)
	if len(submodules) > 0 {
		isBrownfield = true
	}
	projectType := "Greenfield"
	if isBrownfield {
		projectType = "Brownfield"
	}
	frameworks := "Unknown"
	if len(frameworkValues) > 0 {
		frameworks = strings.Join(frameworkValues, ", ")
	}
	return ScanResult{
		ProjectType: projectType,
		Languages:   languages.String(),
		Frameworks:  frameworks,
		BuildSystem: buildSystem,
		NestedRoot:  strings.Join(nestedHits, ", "),
		Submodules:  submodules,
	}
}

type scanAggregation struct {
	languages   *languageCounts
	frameworks  *[]string
	buildSystem *string
	nestedHits  *[]string
}

type directorySignals struct {
	isBrownfield bool
	languages    languageCounts
	frameworks   []string
	buildSystem  string
}

type languageCounts struct {
	counts map[string]int
	order  []string
}

func newLanguageCounts() languageCounts {
	return languageCounts{
		counts: make(map[string]int),
		order:  []string{},
	}
}

func (c *languageCounts) add(language string, count int) {
	if c.counts[language] == 0 {
		c.order = append(c.order, language)
	}
	c.counts[language] += count
}

func (c *languageCounts) merge(other languageCounts) {
	for _, language := range other.order {
		c.add(language, other.counts[language])
	}
}

func (c languageCounts) String() string {
	if len(c.order) == 0 {
		return "Unknown"
	}
	languages := slices.Clone(c.order)
	slices.SortStableFunc(languages, func(a, b string) int {
		return c.counts[b] - c.counts[a]
	})
	primaryCount := c.counts[languages[0]]
	threshold := max(1, primaryCount/5)
	selected := []string{languages[0]}
	for _, language := range languages[1:] {
		if c.counts[language] >= threshold {
			selected = append(selected, language)
		}
	}
	return strings.Join(selected, ", ")
}

func scanDirectorySignals(ops workspaceScanOps, directory string, fileScanDepth int) directorySignals {
	entries, err := ops.readDir(directory)
	if err != nil {
		return directorySignals{
			languages:   newLanguageCounts(),
			frameworks:  []string{},
			buildSystem: "Unknown",
		}
	}
	entryNames := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if !isScanExcluded(entry.Name()) {
			entryNames[entry.Name()] = struct{}{}
		}
	}

	languages := newLanguageCounts()
	countLanguages(ops, directory, &languages, fileScanDepth, true)
	for _, sourceDirectory := range scanSourceDirectories() {
		if _, ok := entryNames[sourceDirectory]; ok {
			countLanguages(
				ops,
				joinScanPath(directory, sourceDirectory),
				&languages,
				6,
				false,
			)
		}
	}
	hasSourceDirectory := false
	for name := range entryNames {
		if isScanSourceDirectory(name) {
			hasSourceDirectory = true
		}
	}
	hasManifest := false
	for name := range entryNames {
		if isSourceManifest(name) {
			hasManifest = true
		}
	}
	frameworks := detectFrameworks(ops, entryNames, directory)
	hasNonDevDependencies := false
	if _, ok := entryNames["package.json"]; ok {
		hasNonDevDependencies = packageHasNonDevDependencies(ops, directory)
	}
	return directorySignals{
		isBrownfield: len(languages.order) > 0 || len(frameworks) > 0 ||
			hasNonDevDependencies || hasSourceDirectory || hasManifest,
		languages:   languages,
		frameworks:  frameworks,
		buildSystem: detectBuildSystem(ops, entryNames, directory),
	}
}

func walkNestedWorkspaces(
	ops workspaceScanOps,
	parent string,
	parentParts []string,
	parentDepth int,
	aggregate *scanAggregation,
) {
	entries, err := ops.readDir(parent)
	if err != nil {
		return
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int {
		return slices.Compare(
			utf16.Encode([]rune(a.Name())),
			utf16.Encode([]rune(b.Name())),
		)
	})
	for _, entry := range entries {
		name := entry.Name()
		if skipNestedScanDirectory(name) {
			continue
		}
		fullName := joinScanPath(parent, name)
		info, err := ops.lstat(fullName)
		if err != nil || info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
			continue
		}
		parts := append(slices.Clone(parentParts), name)
		depth := parentDepth + 1
		signals := scanDirectorySignals(ops, fullName, 0)
		if signals.isBrownfield {
			*aggregate.nestedHits = append(*aggregate.nestedHits, strings.Join(parts, "/"))
			aggregate.languages.merge(signals.languages)
			for _, framework := range signals.frameworks {
				if !slices.Contains(*aggregate.frameworks, framework) {
					*aggregate.frameworks = append(*aggregate.frameworks, framework)
				}
			}
			if *aggregate.buildSystem == "Unknown" {
				*aggregate.buildSystem = signals.buildSystem
			}
			continue
		}
		if depth < 3 {
			walkNestedWorkspaces(ops, fullName, parts, depth, aggregate)
		}
	}
}

func skipNestedScanDirectory(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasPrefix(name, ".") || isScanExcluded(lower) || isNestedScanExcluded(lower) {
		return true
	}
	return isScanSourceDirectory(name)
}

func isNestedScanExcluded(lowerName string) bool {
	switch lowerName {
	case "aidlc", "docs", "doc", "examples", "example", "samples", "sample",
		"demos", "demo", "reference", "testdata", "fixtures", "templates", "scripts":
		return true
	default:
		return false
	}
}

func detectFrameworks(
	ops workspaceScanOps,
	entries map[string]struct{},
	directory string,
) []string {
	frameworks := []string{}
	if hasAnyEntry(entries, "next.config.js", "next.config.ts", "next.config.mjs", "next.config.cjs") {
		frameworks = append(frameworks, "Next.js")
	}
	if hasAnyEntry(entries, "vite.config.js", "vite.config.ts", "vite.config.mjs") {
		frameworks = append(frameworks, "Vite")
	}
	if hasAnyEntry(entries, "angular.json") {
		frameworks = append(frameworks, "Angular")
	}
	if hasAnyEntry(entries, "nuxt.config.js", "nuxt.config.ts") {
		frameworks = append(frameworks, "Nuxt")
	}
	if hasAnyEntry(entries, "remix.config.js") {
		frameworks = append(frameworks, "Remix")
	}
	if hasAnyEntry(entries, "gatsby-config.js") {
		frameworks = append(frameworks, "Gatsby")
	}
	if hasAnyEntry(entries, "astro.config.mjs", "astro.config.js", "astro.config.ts") {
		frameworks = append(frameworks, "Astro")
	}
	if hasAnyEntry(entries, "svelte.config.js") {
		frameworks = append(frameworks, "Svelte")
	}
	if hasAnyEntry(entries, "nest-cli.json") {
		frameworks = append(frameworks, "NestJS")
	}
	if hasAnyEntry(entries, "package.json") {
		if packageJSON, ok := readPackageJSON(ops, directory); ok {
			if value, exists := mergedDependency(packageJSON, "react"); exists && isJavaScriptTruthy(value) {
				frameworks = append(frameworks, "React")
			}
		}
	}
	if hasAnyEntry(entries, "manage.py") {
		frameworks = append(frameworks, "Django")
	}
	if hasAnyEntry(entries, "Gemfile") {
		if data, err := ops.readFile(joinScanPath(directory, "Gemfile")); err == nil && gemfileHasRails(string(data)) {
			frameworks = append(frameworks, "Rails")
		}
	}
	if hasAnyEntry(entries, "pom.xml") {
		if data, err := ops.readFile(joinScanPath(directory, "pom.xml")); err == nil && strings.Contains(string(data), "spring-boot") {
			frameworks = append(frameworks, "Spring Boot")
		}
	}
	return frameworks
}

func gemfileHasRails(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		if comment := strings.IndexByte(line, '#'); comment >= 0 {
			line = line[:comment]
		}
		if containsASCIIWord(line, "rails") {
			return true
		}
	}
	return false
}

func containsASCIIWord(value, word string) bool {
	for offset := 0; offset <= len(value)-len(word); {
		index := strings.Index(value[offset:], word)
		if index < 0 {
			return false
		}
		index += offset
		beforeIsWord := index > 0 && isASCIIWordByte(value[index-1])
		after := index + len(word)
		afterIsWord := after < len(value) && isASCIIWordByte(value[after])
		if !beforeIsWord && !afterIsWord {
			return true
		}
		offset = index + 1
	}
	return false
}

func isASCIIWordByte(value byte) bool {
	return value == '_' || '0' <= value && value <= '9' ||
		'a' <= value && value <= 'z' || 'A' <= value && value <= 'Z'
}

func packageHasNonDevDependencies(ops workspaceScanOps, directory string) bool {
	packageJSON, ok := readPackageJSON(ops, directory)
	if !ok {
		return false
	}
	dependencies := packageJSON["dependencies"]
	switch value := dependencies.(type) {
	case map[string]any:
		return len(value) > 0
	case []any:
		return len(value) > 0
	case string:
		return value != ""
	default:
		return false
	}
}

func readPackageJSON(ops workspaceScanOps, directory string) (map[string]any, bool) {
	data, err := ops.readFile(joinScanPath(directory, "package.json"))
	if err != nil {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return nil, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, false
	}
	object, ok := document.(map[string]any)
	return object, ok
}

func mergedDependency(packageJSON map[string]any, name string) (any, bool) {
	value, exists := dependency(packageJSON["dependencies"], name)
	if peerValue, peerExists := dependency(packageJSON["peerDependencies"], name); peerExists {
		return peerValue, true
	}
	return value, exists
}

func dependency(field any, name string) (any, bool) {
	object, ok := field.(map[string]any)
	if !ok {
		return nil, false
	}
	value, exists := object[name]
	return value, exists
}

func isJavaScriptTruthy(value any) bool {
	switch value := value.(type) {
	case nil:
		return false
	case bool:
		return value
	case float64:
		return value != 0
	case json.Number:
		number, err := value.Float64()
		if err != nil && !errors.Is(err, strconv.ErrRange) {
			return false
		}
		return number != 0
	case string:
		return value != ""
	default:
		return true
	}
}

func parseGitmodules(content string) []Submodule {
	entries := []Submodule{}
	var current *Submodule
	finish := func() {
		if current != nil && isSafeSubmodulePath(current.Path) {
			entries = append(entries, *current)
		}
		current = nil
	}
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimFunc(rawLine, isJavaScriptWhitespace)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			finish()
			if name, ok := parseSubmoduleHeader(line); ok {
				current = &Submodule{Name: name}
			}
			continue
		}
		if current == nil {
			continue
		}
		equals := strings.IndexByte(line, '=')
		if equals < 0 {
			continue
		}
		key := strings.TrimFunc(line[:equals], isJavaScriptWhitespace)
		value := strings.TrimFunc(line[equals+1:], isJavaScriptWhitespace)
		switch key {
		case "path":
			current.Path = value
		case "url":
			current.URL = value
		}
	}
	finish()
	return entries
}

func parseSubmoduleHeader(line string) (string, bool) {
	const prefix = "[submodule"
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, "]") {
		return "", false
	}
	remainder := line[len(prefix) : len(line)-1]
	trimmed := strings.TrimLeftFunc(remainder, isJavaScriptWhitespace)
	if len(trimmed) == len(remainder) || len(trimmed) < 3 || trimmed[0] != '"' || trimmed[len(trimmed)-1] != '"' {
		return "", false
	}
	name := trimmed[1 : len(trimmed)-1]
	return name, name != ""
}

func isSafeSubmodulePath(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || isWindowsDriveAbsolute(name) {
		return false
	}
	for _, part := range strings.FieldsFunc(name, func(char rune) bool {
		return char == '/' || char == '\\'
	}) {
		if part == ".." {
			return false
		}
	}
	return true
}

func isWindowsDriveAbsolute(name string) bool {
	if len(name) < 3 || name[1] != ':' || name[2] != '/' && name[2] != '\\' {
		return false
	}
	return 'a' <= name[0] && name[0] <= 'z' || 'A' <= name[0] && name[0] <= 'Z'
}

func scanSubmodules(ops workspaceScanOps) []Submodule {
	data, err := ops.readFile(".gitmodules")
	if err != nil {
		return []Submodule{}
	}
	entries := parseGitmodules(string(data))
	for index := range entries {
		_, err := ops.stat(path.Join(entries[index].Path, ".git"))
		entries[index].Initialized = err == nil
	}
	return entries
}

func countLanguages(
	ops workspaceScanOps,
	directory string,
	counts *languageCounts,
	maxDepth int,
	skipSourceDirectories bool,
) {
	if maxDepth < 0 {
		return
	}
	entries, err := ops.readDir(directory)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if isScanExcluded(name) {
			continue
		}
		fullName := joinScanPath(directory, name)
		info, err := ops.lstat(fullName)
		if err != nil || info.Mode()&fs.ModeSymlink != 0 {
			continue
		}
		if info.IsDir() {
			if skipSourceDirectories && isScanSourceDirectory(name) {
				continue
			}
			countLanguages(ops, fullName, counts, maxDepth-1, false)
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if language := languageForFile(name); language != "" {
			counts.add(language, 1)
		}
	}
}

func languageForFile(name string) string {
	dot := strings.LastIndexByte(name, '.')
	if dot <= 0 {
		return ""
	}
	switch strings.ToLower(name[dot:]) {
	case ".ts", ".tsx":
		return "TypeScript"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "JavaScript"
	case ".py":
		return "Python"
	case ".java":
		return "Java"
	case ".kt":
		return "Kotlin"
	case ".go":
		return "Go"
	case ".rs":
		return "Rust"
	case ".rb":
		return "Ruby"
	case ".cs":
		return "C#"
	case ".cpp", ".hpp":
		return "C++"
	case ".c", ".h":
		return "C"
	case ".swift":
		return "Swift"
	case ".php":
		return "PHP"
	default:
		return ""
	}
}

func detectBuildSystem(
	ops workspaceScanOps,
	entries map[string]struct{},
	directory string,
) string {
	if _, ok := entries["package.json"]; ok {
		if hasAnyEntry(entries, "pnpm-lock.yaml") {
			return "pnpm (package.json)"
		}
		if hasAnyEntry(entries, "yarn.lock") {
			return "yarn (package.json)"
		}
		if hasAnyEntry(entries, "bun.lockb", "bun.lock") {
			return "bun (package.json)"
		}
		return "npm (package.json)"
	}
	if _, ok := entries["pyproject.toml"]; ok {
		if data, err := ops.readFile(joinScanPath(directory, "pyproject.toml")); err == nil {
			content := string(data)
			switch {
			case strings.Contains(content, "[tool.poetry]"):
				return "poetry (pyproject.toml)"
			case strings.Contains(content, "[tool.uv]"):
				return "uv (pyproject.toml)"
			case strings.Contains(content, "[tool.hatch]"):
				return "hatch (pyproject.toml)"
			}
		}
		return "python (pyproject.toml)"
	}
	if _, ok := entries["requirements.txt"]; ok {
		return "pip (requirements.txt)"
	}
	if _, ok := entries["setup.py"]; ok {
		return "setuptools (setup.py)"
	}
	if _, ok := entries["Cargo.toml"]; ok {
		return "cargo (Cargo.toml)"
	}
	if _, ok := entries["go.mod"]; ok {
		return "go modules (go.mod)"
	}
	if _, ok := entries["pom.xml"]; ok {
		return "maven (pom.xml)"
	}
	if hasAnyEntry(entries, "build.gradle", "build.gradle.kts") {
		return "gradle (build.gradle)"
	}
	if _, ok := entries["composer.json"]; ok {
		return "composer (composer.json)"
	}
	if _, ok := entries["Gemfile"]; ok {
		return "bundler (Gemfile)"
	}
	return "Unknown"
}

func hasAnyEntry(entries map[string]struct{}, names ...string) bool {
	for _, name := range names {
		if _, ok := entries[name]; ok {
			return true
		}
	}
	return false
}

func isScanSourceDirectory(name string) bool {
	switch name {
	case "src", "app", "lib", "pages", "components", "tests":
		return true
	default:
		return false
	}
}

func scanSourceDirectories() [6]string {
	return [6]string{"src", "app", "lib", "pages", "components", "tests"}
}

func isSourceManifest(name string) bool {
	switch name {
	case "requirements.txt", "pyproject.toml", "setup.py", "Cargo.toml", "go.mod",
		"pom.xml", "build.gradle", "build.gradle.kts", "composer.json", "Gemfile":
		return true
	default:
		return false
	}
}

func isScanExcluded(name string) bool {
	switch name {
	case ".claude", ".kiro", ".codex", ".opencode", ".aidlc", ".cursor",
		"aidlc-docs", "node_modules", ".git", "dist", "build", ".next",
		"target", "vendor":
		return true
	default:
		return false
	}
}

func joinScanPath(directory, name string) string {
	if directory == "." {
		return name
	}
	return path.Join(directory, name)
}
