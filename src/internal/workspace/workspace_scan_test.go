package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestDetectClassifiesRootSignals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files fstest.MapFS
		want  ScanResult
	}{
		{
			name:  "empty root",
			files: fstest.MapFS{},
			want: ScanResult{
				ProjectType: "Greenfield",
				Languages:   "Unknown",
				Frameworks:  "Unknown",
				BuildSystem: "Unknown",
				Submodules:  []Submodule{},
			},
		},
		{
			name: "root source file",
			files: fstest.MapFS{
				"main.go": {Data: []byte("package main\n")},
			},
			want: ScanResult{
				ProjectType: "Brownfield",
				Languages:   "Go",
				Frameworks:  "Unknown",
				BuildSystem: "Unknown",
				Submodules:  []Submodule{},
			},
		},
		{
			name: "manifest",
			files: fstest.MapFS{
				"requirements.txt": {Data: []byte("requests\n")},
			},
			want: ScanResult{
				ProjectType: "Brownfield",
				Languages:   "Unknown",
				Frameworks:  "Unknown",
				BuildSystem: "pip (requirements.txt)",
				Submodules:  []Submodule{},
			},
		},
		{
			name: "empty source directory",
			files: fstest.MapFS{
				"src": {Mode: fs.ModeDir},
			},
			want: ScanResult{
				ProjectType: "Brownfield",
				Languages:   "Unknown",
				Frameworks:  "Unknown",
				BuildSystem: "Unknown",
				Submodules:  []Submodule{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := detectWithScanOps(mapScanOps(tt.files))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Detect() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDetectReportsLanguages(t *testing.T) {
	t.Parallel()

	extensions := []struct {
		name      string
		extension string
		language  string
	}{
		{name: "typescript ts", extension: ".ts", language: "TypeScript"},
		{name: "typescript tsx", extension: ".tsx", language: "TypeScript"},
		{name: "javascript js", extension: ".js", language: "JavaScript"},
		{name: "javascript jsx", extension: ".jsx", language: "JavaScript"},
		{name: "javascript mjs", extension: ".mjs", language: "JavaScript"},
		{name: "javascript cjs", extension: ".cjs", language: "JavaScript"},
		{name: "python", extension: ".py", language: "Python"},
		{name: "java", extension: ".java", language: "Java"},
		{name: "kotlin", extension: ".kt", language: "Kotlin"},
		{name: "go", extension: ".go", language: "Go"},
		{name: "rust", extension: ".rs", language: "Rust"},
		{name: "ruby", extension: ".rb", language: "Ruby"},
		{name: "c sharp", extension: ".cs", language: "C#"},
		{name: "c plus plus cpp", extension: ".cpp", language: "C++"},
		{name: "c", extension: ".c", language: "C"},
		{name: "c header", extension: ".h", language: "C"},
		{name: "c plus plus header", extension: ".hpp", language: "C++"},
		{name: "swift", extension: ".swift", language: "Swift"},
		{name: "php", extension: ".php", language: "PHP"},
	}
	for _, tt := range extensions {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			files := fstest.MapFS{"main" + strings.ToUpper(tt.extension): {Data: []byte("source\n")}}
			got := detectWithScanOps(mapScanOps(files))
			if got.ProjectType != "Brownfield" || got.Languages != tt.language {
				t.Errorf("Detect() = (%q, %q), want (Brownfield, %q)", got.ProjectType, got.Languages, tt.language)
			}
		})
	}

	t.Run("hidden extension at index zero", func(t *testing.T) {
		t.Parallel()

		got := detectWithScanOps(mapScanOps(fstest.MapFS{".py": {Data: []byte("hidden\n")}}))
		if got.ProjectType != "Greenfield" || got.Languages != "Unknown" {
			t.Errorf("Detect() = (%q, %q), want hidden file ignored", got.ProjectType, got.Languages)
		}
	})

	t.Run("source depth six included and depth seven excluded", func(t *testing.T) {
		t.Parallel()

		files := fstest.MapFS{
			"src/a/b/c/d/e/f/deep.py":   {Data: []byte("included\n")},
			"src/a/b/c/d/e/f/g/deep.ts": {Data: []byte("excluded\n")},
		}
		got := detectWithScanOps(mapScanOps(files))
		if got.Languages != "Python" {
			t.Errorf("Detect().Languages = %q, want %q", got.Languages, "Python")
		}
	})

	t.Run("symlink and excluded directory ignored", func(t *testing.T) {
		t.Parallel()

		files := fstest.MapFS{
			"linked.go":           {Mode: fs.ModeSymlink},
			"node_modules/app.py": {Data: []byte("ignored\n")},
		}
		got := detectWithScanOps(mapScanOps(files))
		if got.ProjectType != "Greenfield" || got.Languages != "Unknown" {
			t.Errorf("Detect() = (%q, %q), want ignored files", got.ProjectType, got.Languages)
		}
	})

	t.Run("secondary threshold", func(t *testing.T) {
		t.Parallel()

		atThreshold := fstest.MapFS{"src/secondary.py": {Data: []byte("python\n")}}
		for index := range 5 {
			atThreshold[fmt.Sprintf("src/primary-%d.ts", index)] = &fstest.MapFile{Data: []byte("typescript\n")}
		}
		if got := detectWithScanOps(mapScanOps(atThreshold)).Languages; got != "TypeScript, Python" {
			t.Errorf("languages at threshold = %q, want %q", got, "TypeScript, Python")
		}

		belowThreshold := fstest.MapFS{"src/secondary.py": {Data: []byte("python\n")}}
		for index := range 10 {
			belowThreshold[fmt.Sprintf("src/primary-%d.ts", index)] = &fstest.MapFile{Data: []byte("typescript\n")}
		}
		if got := detectWithScanOps(mapScanOps(belowThreshold)).Languages; got != "TypeScript" {
			t.Errorf("languages below threshold = %q, want %q", got, "TypeScript")
		}
	})

	t.Run("equal counts keep native observation order", func(t *testing.T) {
		t.Parallel()

		files := fstest.MapFS{
			"a.py": {Data: []byte("python\n")},
			"z.go": {Data: []byte("go\n")},
		}
		got := detectWithScanOps(orderedRootScanOps(files, []string{"z.go", "a.py"}))
		if got.Languages != "Go, Python" {
			t.Errorf("Detect().Languages = %q, want native tie order %q", got.Languages, "Go, Python")
		}
	})
}

func TestDetectInterpretsWeakPackageJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		packageJSON string
		wantType    string
		wantFW      string
	}{
		{
			name:        "runtime dependency key is brownfield",
			packageJSON: `{"dependencies":{"left-pad":null}}`,
			wantType:    "Brownfield",
			wantFW:      "Unknown",
		},
		{
			name:        "development dependencies alone are greenfield",
			packageJSON: `{"devDependencies":{"vite":"1"}}`,
			wantType:    "Greenfield",
			wantFW:      "Unknown",
		},
		{
			name:        "peer react is framework signal",
			packageJSON: `{"peerDependencies":{"react":"18"}}`,
			wantType:    "Brownfield",
			wantFW:      "React",
		},
		{
			name:        "string dependencies expose object keys",
			packageJSON: `{"dependencies":"x"}`,
			wantType:    "Brownfield",
			wantFW:      "Unknown",
		},
		{
			name:        "empty string dependencies have no keys",
			packageJSON: `{"dependencies":""}`,
			wantType:    "Greenfield",
			wantFW:      "Unknown",
		},
		{
			name:        "array dependencies expose indexes",
			packageJSON: `{"dependencies":[null]}`,
			wantType:    "Brownfield",
			wantFW:      "Unknown",
		},
		{
			name:        "numeric dependencies have no keys",
			packageJSON: `{"dependencies":1}`,
			wantType:    "Greenfield",
			wantFW:      "Unknown",
		},
		{
			name:        "empty object react value is truthy",
			packageJSON: `{"dependencies":{"react":{}}}`,
			wantType:    "Brownfield",
			wantFW:      "React",
		},
		{
			name:        "peer react overrides dependency react",
			packageJSON: `{"dependencies":{"react":"18"},"peerDependencies":{"react":""}}`,
			wantType:    "Brownfield",
			wantFW:      "Unknown",
		},
		{
			name:        "false react value is not a framework signal",
			packageJSON: `{"dependencies":{"react":false}}`,
			wantType:    "Brownfield",
			wantFW:      "Unknown",
		},
		{
			name:        "positive overflow dependency is truthy",
			packageJSON: `{"dependencies":{"react":1e400}}`,
			wantType:    "Brownfield",
			wantFW:      "React",
		},
		{
			name:        "negative overflow peer dependency is truthy",
			packageJSON: `{"peerDependencies":{"react":-1e400}}`,
			wantType:    "Brownfield",
			wantFW:      "React",
		},
		{
			name:        "positive underflow dependency is falsy",
			packageJSON: `{"dependencies":{"react":1e-400}}`,
			wantType:    "Brownfield",
			wantFW:      "Unknown",
		},
		{
			name:        "negative underflow peer dependency overrides truthy dependency",
			packageJSON: `{"dependencies":{"react":"18"},"peerDependencies":{"react":-1e-400}}`,
			wantType:    "Brownfield",
			wantFW:      "Unknown",
		},
		{
			name:        "positive zero dependency is falsy",
			packageJSON: `{"dependencies":{"react":0}}`,
			wantType:    "Brownfield",
			wantFW:      "Unknown",
		},
		{
			name:        "negative zero peer dependency is falsy",
			packageJSON: `{"peerDependencies":{"react":-0}}`,
			wantType:    "Greenfield",
			wantFW:      "Unknown",
		},
		{
			name:        "non object package document",
			packageJSON: `[{"dependencies":{"react":"18"}}]`,
			wantType:    "Greenfield",
			wantFW:      "Unknown",
		},
		{
			name:        "invalid package document",
			packageJSON: `{"dependencies":`,
			wantType:    "Greenfield",
			wantFW:      "Unknown",
		},
		{
			name:        "trailing JSON token rejects the document",
			packageJSON: `{"dependencies":{"react":"18"}} true`,
			wantType:    "Greenfield",
			wantFW:      "Unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			files := fstest.MapFS{"package.json": {Data: []byte(tt.packageJSON)}}
			got := detectWithScanOps(mapScanOps(files))
			if got.ProjectType != tt.wantType || got.Frameworks != tt.wantFW || got.BuildSystem != "npm (package.json)" {
				t.Errorf(
					"Detect() = (%q, %q, %q), want (%q, %q, npm (package.json))",
					got.ProjectType,
					got.Frameworks,
					got.BuildSystem,
					tt.wantType,
					tt.wantFW,
				)
			}
		})
	}
}

func TestDetectFrameworkAndBuildSignals(t *testing.T) {
	t.Parallel()

	t.Run("frameworks use fixed order", func(t *testing.T) {
		t.Parallel()

		files := fstest.MapFS{
			"next.config.ts":   {Data: []byte("next\n")},
			"vite.config.ts":   {Data: []byte("vite\n")},
			"angular.json":     {Data: []byte("{}\n")},
			"nuxt.config.ts":   {Data: []byte("nuxt\n")},
			"remix.config.js":  {Data: []byte("remix\n")},
			"gatsby-config.js": {Data: []byte("gatsby\n")},
			"astro.config.mjs": {Data: []byte("astro\n")},
			"svelte.config.js": {Data: []byte("svelte\n")},
			"nest-cli.json":    {Data: []byte("{}\n")},
			"package.json":     {Data: []byte(`{"dependencies":{"react":"18"}}`)},
			"manage.py":        {Data: []byte("django\n")},
			"Gemfile":          {Data: []byte("# rails in a comment\ngem \"rails\"\n")},
			"pom.xml":          {Data: []byte("<artifactId>spring-boot</artifactId>\n")},
		}
		got := detectWithScanOps(mapScanOps(files))
		want := "Next.js, Vite, Angular, Nuxt, Remix, Gatsby, Astro, Svelte, NestJS, React, Django, Rails, Spring Boot"
		if got.Frameworks != want {
			t.Errorf("Detect().Frameworks = %q, want %q", got.Frameworks, want)
		}
	})

	t.Run("framework reads fail without a signal", func(t *testing.T) {
		t.Parallel()

		files := fstest.MapFS{
			"package.json": {Data: []byte(`{"dependencies":{"react":"18"}}`)},
			"Gemfile":      {Data: []byte("gem \"rails\"\n")},
			"pom.xml":      {Data: []byte("spring-boot\n")},
		}
		ops := mapScanOps(files)
		ops.readFile = func(string) ([]byte, error) {
			return nil, errors.New("injected read failure")
		}
		if got := detectWithScanOps(ops).Frameworks; got != "Unknown" {
			t.Errorf("Detect().Frameworks = %q, want read failures absorbed", got)
		}
	})

	builds := []struct {
		name  string
		files fstest.MapFS
		want  string
	}{
		{
			name: "pnpm before other package locks",
			files: fstest.MapFS{
				"package.json":   {Data: []byte(`{}`)},
				"pnpm-lock.yaml": {Data: []byte("lock\n")},
				"yarn.lock":      {Data: []byte("lock\n")},
				"bun.lock":       {Data: []byte("lock\n")},
			},
			want: "pnpm (package.json)",
		},
		{name: "yarn", files: fstest.MapFS{"package.json": {Data: []byte(`{}`)}, "yarn.lock": {}}, want: "yarn (package.json)"},
		{name: "bun lockb", files: fstest.MapFS{"package.json": {Data: []byte(`{}`)}, "bun.lockb": {}}, want: "bun (package.json)"},
		{name: "npm", files: fstest.MapFS{"package.json": {Data: []byte(`{}`)}}, want: "npm (package.json)"},
		{name: "poetry", files: fstest.MapFS{"pyproject.toml": {Data: []byte("[tool.poetry]\n")}}, want: "poetry (pyproject.toml)"},
		{name: "uv", files: fstest.MapFS{"pyproject.toml": {Data: []byte("[tool.uv]\n")}}, want: "uv (pyproject.toml)"},
		{name: "hatch", files: fstest.MapFS{"pyproject.toml": {Data: []byte("[tool.hatch]\n")}}, want: "hatch (pyproject.toml)"},
		{name: "generic python", files: fstest.MapFS{"pyproject.toml": {Data: []byte("[project]\n")}}, want: "python (pyproject.toml)"},
		{name: "pip", files: fstest.MapFS{"requirements.txt": {}}, want: "pip (requirements.txt)"},
		{name: "setuptools", files: fstest.MapFS{"setup.py": {}}, want: "setuptools (setup.py)"},
		{name: "cargo", files: fstest.MapFS{"Cargo.toml": {}}, want: "cargo (Cargo.toml)"},
		{name: "go modules", files: fstest.MapFS{"go.mod": {}}, want: "go modules (go.mod)"},
		{name: "maven", files: fstest.MapFS{"pom.xml": {}}, want: "maven (pom.xml)"},
		{name: "gradle", files: fstest.MapFS{"build.gradle.kts": {}}, want: "gradle (build.gradle)"},
		{name: "composer", files: fstest.MapFS{"composer.json": {}}, want: "composer (composer.json)"},
		{name: "bundler", files: fstest.MapFS{"Gemfile": {}}, want: "bundler (Gemfile)"},
	}
	for _, tt := range builds {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := detectWithScanOps(mapScanOps(tt.files)).BuildSystem; got != tt.want {
				t.Errorf("Detect().BuildSystem = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("pyproject read failure falls back to generic python", func(t *testing.T) {
		t.Parallel()

		files := fstest.MapFS{"pyproject.toml": {Data: []byte("[tool.poetry]\n")}}
		ops := mapScanOps(files)
		ops.readFile = func(string) ([]byte, error) {
			return nil, errors.New("injected read failure")
		}
		if got := detectWithScanOps(ops).BuildSystem; got != "python (pyproject.toml)" {
			t.Errorf("Detect().BuildSystem = %q, want generic fallback", got)
		}
	})
}

func TestDetectFindsNestedWorkspaces(t *testing.T) {
	t.Parallel()

	t.Run("root signal prevents fallback", func(t *testing.T) {
		t.Parallel()

		files := fstest.MapFS{
			"main.go":        {Data: []byte("package main\n")},
			"nested/main.py": {Data: []byte("print(1)\n")},
		}
		got := detectWithScanOps(mapScanOps(files))
		if got.NestedRoot != "" || got.Languages != "Go" {
			t.Errorf("Detect() = (nested %q, languages %q), want root-only result", got.NestedRoot, got.Languages)
		}
	})

	t.Run("depth three included and depth four excluded", func(t *testing.T) {
		t.Parallel()

		files := fstest.MapFS{
			"one/two/three/main.go":        {Data: []byte("package main\n")},
			"other/two/three/four/main.py": {Data: []byte("print(1)\n")},
		}
		got := detectWithScanOps(mapScanOps(files))
		if got.ProjectType != "Brownfield" || got.NestedRoot != "one/two/three" || got.Languages != "Go" {
			t.Errorf("Detect() = (%q, %q, %q), want depth-three Go hit", got.ProjectType, got.NestedRoot, got.Languages)
		}
	})

	t.Run("multiple hits use utf16 order and stable language tie", func(t *testing.T) {
		t.Parallel()

		files := fstest.MapFS{
			"\U00010000/main.go": {Data: []byte("package main\n")},
			"\ue000/main.py":     {Data: []byte("print(1)\n")},
		}
		got := detectWithScanOps(mapScanOps(files))
		if got.NestedRoot != "\U00010000, \ue000" || got.Languages != "Go, Python" {
			t.Errorf("Detect() = (nested %q, languages %q), want UTF-16 hit order", got.NestedRoot, got.Languages)
		}
	})

	t.Run("excluded containers and symlink are ignored", func(t *testing.T) {
		t.Parallel()

		files := fstest.MapFS{
			"docs/main.py":     {Data: []byte("ignored\n")},
			"EXAMPLES/main.py": {Data: []byte("ignored\n")},
			"scripts/main.py":  {Data: []byte("ignored\n")},
			"fixtures/main.py": {Data: []byte("ignored\n")},
			".hidden/main.py":  {Data: []byte("ignored\n")},
			"linked":           {Mode: fs.ModeDir | fs.ModeSymlink},
			"linked/main.go":   {Data: []byte("ignored\n")},
			"plain-file":       {Data: []byte("not a directory\n")},
		}
		got := detectWithScanOps(mapScanOps(files))
		if got.ProjectType != "Greenfield" || got.NestedRoot != "" {
			t.Errorf("Detect() = (%q, %q), want excluded containers ignored", got.ProjectType, got.NestedRoot)
		}
	})

	t.Run("hit is not descended and source is counted once", func(t *testing.T) {
		t.Parallel()

		files := fstest.MapFS{
			"project/main.py":          {Data: []byte("print(1)\n")},
			"project/src/one.ts":       {Data: []byte("typescript\n")},
			"project/src/two.ts":       {Data: []byte("typescript\n")},
			"project/child/another.go": {Data: []byte("package child\n")},
		}
		got := detectWithScanOps(mapScanOps(files))
		if got.NestedRoot != "project" || got.Languages != "TypeScript, Python" {
			t.Errorf("Detect() = (nested %q, languages %q), want one non-descended hit", got.NestedRoot, got.Languages)
		}
	})

	t.Run("nested findings merge in traversal order", func(t *testing.T) {
		t.Parallel()

		files := fstest.MapFS{
			"api/go.mod":         {Data: []byte("module api\n")},
			"api/vite.config.ts": {Data: []byte("vite\n")},
			"web/package.json":   {Data: []byte(`{"dependencies":{"react":"18"}}`)},
		}
		got := detectWithScanOps(mapScanOps(files))
		if got.NestedRoot != "api, web" || got.Frameworks != "Vite, React" || got.BuildSystem != "go modules (go.mod)" {
			t.Errorf(
				"Detect() = (nested %q, frameworks %q, build %q), want ordered merge",
				got.NestedRoot,
				got.Frameworks,
				got.BuildSystem,
			)
		}
	})

	t.Run("greenfield root build takes precedence", func(t *testing.T) {
		t.Parallel()

		files := fstest.MapFS{
			"package.json":   {Data: []byte(`{"devDependencies":{"test":"1"}}`)},
			"service/go.mod": {Data: []byte("module service\n")},
		}
		got := detectWithScanOps(mapScanOps(files))
		if got.NestedRoot != "service" || got.BuildSystem != "npm (package.json)" {
			t.Errorf("Detect() = (nested %q, build %q), want root build precedence", got.NestedRoot, got.BuildSystem)
		}
	})
}

func TestParseGitmodules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []Submodule
	}{
		{
			name: "multiple entries preserve declaration order",
			content: `[submodule "services/api"]
	path = services/api
	url = https://example.com/api.git
[submodule "services/web"]
	path = services/web
	url = https://example.com/web.git
`,
			want: []Submodule{
				{Name: "services/api", Path: "services/api", URL: "https://example.com/api.git"},
				{Name: "services/web", Path: "services/web", URL: "https://example.com/web.git"},
			},
		},
		{
			name: "comments unknown keys and optional url",
			content: `# comment
; another comment
[submodule "lib"]
	path = lib/foo
	branch = main
`,
			want: []Submodule{{Name: "lib", Path: "lib/foo"}},
		},
		{
			name: "partial parse keeps valid entries",
			content: `[submodule "first"]
	path = first
[invalid]
	path = ignored
[submodule "missing"]
	url = missing
[submodule "last"]
	path = last
`,
			want: []Submodule{{Name: "first", Path: "first"}, {Name: "last", Path: "last"}},
		},
		{
			name: "unsafe paths are dropped before safe paths",
			content: `[submodule "absolute"]
	path = /outside
[submodule "drive-forward"]
	path = C:/outside
[submodule "drive-back"]
	path = D:\outside
[submodule "up-forward"]
	path = a/../outside
[submodule "up-back"]
	path = a\..\outside
[submodule "safe"]
	path = ./pkg//kept
`,
			want: []Submodule{{Name: "safe", Path: "./pkg//kept"}},
		},
		{
			name: "accepted unusual and duplicate paths remain verbatim",
			content: `[submodule "dot"]
	path = .
[submodule "root-relative-backslash"]
	path = \server
[submodule "colon"]
	path = name:part
[submodule "duplicate-one"]
	path = same
[submodule "duplicate-two"]
	path = same
`,
			want: []Submodule{
				{Name: "dot", Path: "."},
				{Name: "root-relative-backslash", Path: `\server`},
				{Name: "colon", Path: "name:part"},
				{Name: "duplicate-one", Path: "same"},
				{Name: "duplicate-two", Path: "same"},
			},
		},
		{
			name:    "total garbage",
			content: "not ini\n=headerless\n[core]\nfoo=bar\n",
			want:    []Submodule{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := parseGitmodules(tt.content); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseGitmodules() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDetectReportsSubmodulesAfterNestedFallback(t *testing.T) {
	t.Parallel()

	t.Run("uninitialized declaration is brownfield", func(t *testing.T) {
		t.Parallel()

		content := `[submodule "services/api"]
	path = services/api
	url = https://example.com/api.git
`
		got := detectWithScanOps(mapScanOps(fstest.MapFS{".gitmodules": {Data: []byte(content)}}))
		want := []Submodule{{
			Name: "services/api", Path: "services/api", URL: "https://example.com/api.git",
		}}
		if got.ProjectType != "Brownfield" || got.Languages != "Unknown" || !reflect.DeepEqual(got.Submodules, want) {
			t.Errorf("Detect() = (%q, %q, %#v), want uninitialized Brownfield", got.ProjectType, got.Languages, got.Submodules)
		}
	})

	t.Run("git entry marks initialized", func(t *testing.T) {
		t.Parallel()

		content := `[submodule "services/api"]
	path = services/api
`
		files := fstest.MapFS{
			".gitmodules":       {Data: []byte(content)},
			"services/api/.git": {Data: []byte("gitdir: elsewhere\n")},
		}
		got := detectWithScanOps(mapScanOps(files))
		if len(got.Submodules) != 1 || !got.Submodules[0].Initialized {
			t.Errorf("Detect().Submodules = %#v, want initialized declaration", got.Submodules)
		}
	})

	t.Run("submodules are evaluated after nested scan", func(t *testing.T) {
		t.Parallel()

		content := `[submodule "external"]
	path = external
`
		files := fstest.MapFS{
			".gitmodules":     {Data: []byte(content)},
			"service/main.go": {Data: []byte("package service\n")},
		}
		got := detectWithScanOps(mapScanOps(files))
		if got.ProjectType != "Brownfield" || got.NestedRoot != "service" || got.Languages != "Go" || len(got.Submodules) != 1 {
			t.Errorf(
				"Detect() = (%q, %q, %q, %#v), want nested findings plus submodule",
				got.ProjectType,
				got.NestedRoot,
				got.Languages,
				got.Submodules,
			)
		}
	})

	t.Run("missing and malformed declarations have no signal", func(t *testing.T) {
		t.Parallel()

		for name, files := range map[string]fstest.MapFS{
			"missing":   {},
			"malformed": {".gitmodules": {Data: []byte("[core]\nfoo=bar\n")}},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				got := detectWithScanOps(mapScanOps(files))
				if got.ProjectType != "Greenfield" || got.Submodules == nil || len(got.Submodules) != 0 {
					t.Errorf("Detect() = (%q, %#v), want non-nil empty no-signal", got.ProjectType, got.Submodules)
				}
			})
		}
	})

	t.Run("stat failure leaves declaration uninitialized", func(t *testing.T) {
		t.Parallel()

		content := `[submodule "service"]
	path = service
`
		files := fstest.MapFS{
			".gitmodules":  {Data: []byte(content)},
			"service/.git": {Data: []byte("gitdir\n")},
		}
		ops := mapScanOps(files)
		ops.stat = func(string) (fs.FileInfo, error) {
			return nil, errors.New("injected stat failure")
		}
		got := detectWithScanOps(ops)
		if len(got.Submodules) != 1 || got.Submodules[0].Initialized {
			t.Errorf("Detect().Submodules = %#v, want stat failure absorbed as uninitialized", got.Submodules)
		}
	})
}

func TestDetectAbsorbsInternalIOFailures(t *testing.T) {
	t.Parallel()

	t.Run("partial root entries with read error are discarded", func(t *testing.T) {
		t.Parallel()

		ops := mapScanOps(fstest.MapFS{"main.go": {Data: []byte("package main\n")}})
		readDir := ops.readDir
		ops.readDir = func(name string) ([]fs.DirEntry, error) {
			entries, err := readDir(name)
			if err == nil && name == "." {
				return entries, errors.New("injected read failure")
			}
			return entries, err
		}
		got := detectWithScanOps(ops)
		if got.ProjectType != "Greenfield" || got.Languages != "Unknown" {
			t.Errorf("Detect() = (%q, %q), want partial read discarded", got.ProjectType, got.Languages)
		}
	})

	t.Run("lstat failure skips only that entry", func(t *testing.T) {
		t.Parallel()

		files := fstest.MapFS{
			"a.go": {Data: []byte("package a\n")},
			"b.py": {Data: []byte("print(1)\n")},
		}
		ops := orderedRootScanOps(files, []string{"a.go", "b.py"})
		lstat := ops.lstat
		ops.lstat = func(name string) (fs.FileInfo, error) {
			if name == "a.go" {
				return nil, errors.New("injected lstat failure")
			}
			return lstat(name)
		}
		if got := detectWithScanOps(ops).Languages; got != "Python" {
			t.Errorf("Detect().Languages = %q, want later entry retained", got)
		}
	})

	t.Run("source directory read failure keeps directory signal", func(t *testing.T) {
		t.Parallel()

		ops := mapScanOps(fstest.MapFS{"src/main.go": {Data: []byte("package main\n")}})
		readDir := ops.readDir
		ops.readDir = func(name string) ([]fs.DirEntry, error) {
			if name == "src" {
				return nil, errors.New("injected source read failure")
			}
			return readDir(name)
		}
		got := detectWithScanOps(ops)
		if got.ProjectType != "Brownfield" || got.Languages != "Unknown" {
			t.Errorf("Detect() = (%q, %q), want source-directory-only signal", got.ProjectType, got.Languages)
		}
	})

	t.Run("package read failure removes package signals", func(t *testing.T) {
		t.Parallel()

		ops := mapScanOps(fstest.MapFS{
			"package.json": {Data: []byte(`{"dependencies":{"react":"18"}}`)},
		})
		ops.readFile = func(string) ([]byte, error) {
			return nil, errors.New("injected package read failure")
		}
		got := detectWithScanOps(ops)
		if got.ProjectType != "Greenfield" || got.Frameworks != "Unknown" || got.BuildSystem != "npm (package.json)" {
			t.Errorf(
				"Detect() = (%q, %q, %q), want package signals absorbed but build marker retained",
				got.ProjectType,
				got.Frameworks,
				got.BuildSystem,
			)
		}
	})

	t.Run("nested lstat failure skips candidate and continues", func(t *testing.T) {
		t.Parallel()

		files := fstest.MapFS{
			"a/main.go": {Data: []byte("package a\n")},
			"b/main.py": {Data: []byte("print(1)\n")},
		}
		ops := mapScanOps(files)
		lstat := ops.lstat
		ops.lstat = func(name string) (fs.FileInfo, error) {
			if name == "a" {
				return nil, errors.New("injected nested lstat failure")
			}
			return lstat(name)
		}
		got := detectWithScanOps(ops)
		if got.NestedRoot != "b" || got.Languages != "Python" {
			t.Errorf("Detect() = (%q, %q), want later nested candidate", got.NestedRoot, got.Languages)
		}
	})

	t.Run("partial gitmodules read error has no signal", func(t *testing.T) {
		t.Parallel()

		ops := mapScanOps(fstest.MapFS{})
		ops.readFile = func(name string) ([]byte, error) {
			if name == ".gitmodules" {
				return []byte("[submodule \"x\"]\npath=x\n"), errors.New("injected read failure")
			}
			return nil, fs.ErrNotExist
		}
		got := detectWithScanOps(ops)
		if got.ProjectType != "Greenfield" || len(got.Submodules) != 0 {
			t.Errorf("Detect() = (%q, %#v), want partial gitmodules discarded", got.ProjectType, got.Submodules)
		}
	})
}

func mapScanOps(files fstest.MapFS) workspaceScanOps {
	return workspaceScanOps{
		readDir: func(name string) ([]fs.DirEntry, error) {
			return fs.ReadDir(files, name)
		},
		lstat: func(name string) (fs.FileInfo, error) {
			return fs.Stat(files, name)
		},
		stat: func(name string) (fs.FileInfo, error) {
			return fs.Stat(files, name)
		},
		readFile: func(name string) ([]byte, error) {
			return fs.ReadFile(files, name)
		},
	}
}

func orderedRootScanOps(files fstest.MapFS, order []string) workspaceScanOps {
	ops := mapScanOps(files)
	readDir := ops.readDir
	ops.readDir = func(name string) ([]fs.DirEntry, error) {
		entries, err := readDir(name)
		if err != nil || name != "." {
			return entries, err
		}
		byName := make(map[string]fs.DirEntry, len(entries))
		for _, entry := range entries {
			byName[entry.Name()] = entry
		}
		ordered := make([]fs.DirEntry, 0, len(entries))
		for _, entryName := range order {
			if entry, ok := byName[entryName]; ok {
				ordered = append(ordered, entry)
				delete(byName, entryName)
			}
		}
		for _, entry := range entries {
			if _, ok := byName[entry.Name()]; ok {
				ordered = append(ordered, entry)
			}
		}
		return ordered, nil
	}
	return ops
}
