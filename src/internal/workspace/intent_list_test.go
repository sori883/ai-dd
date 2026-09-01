package workspace

import (
	"fmt"
	"io/fs"
	"reflect"
	"slices"
	"testing"
	"testing/fstest"
)

func TestListIntentsRegistryRow(t *testing.T) {
	t.Parallel()

	scope := "repository"
	intentsFS := fstest.MapFS{
		"intents.json": &fstest.MapFile{Data: []byte(`[
            {
                "uuid": "018f-aaaa",
                "slug": "build-auth",
                "status": "construction",
                "scope": "repository",
                "repos": ["api", "web"]
            }
        ]`)},
	}
	want := []Intent{{
		UUID:   "018f-aaaa",
		Slug:   "build-auth",
		Status: "construction",
		Scope:  &scope,
		Repos:  []string{"api", "web"},
	}}

	got, err := ListIntents(intentsFS, new(string))
	if err != nil {
		t.Fatalf("ListIntents() error = %v, want nil", err)
	}
	assertIntents(t, got, want)
}

func TestListIntentsExactDirectoryAndActiveOverride(t *testing.T) {
	t.Parallel()

	dirName := "240901-build-auth"
	activeOverride := dirName
	intentsFS := fstest.MapFS{
		"intents.json": &fstest.MapFile{Data: []byte(`[
            {
                "uuid": "018f-aaaa",
                "slug": "build-auth",
                "status": "construction",
                "dirName": "240901-build-auth"
            }
        ]`)},
		"240901-build-auth/aidlc-state.md": &fstest.MapFile{Data: []byte("state")},
	}

	got, err := ListIntents(intentsFS, &activeOverride)
	if err != nil {
		t.Fatalf("ListIntents() error = %v, want nil", err)
	}
	assertIntents(t, got, []Intent{{
		UUID:    "018f-aaaa",
		Slug:    "build-auth",
		Status:  "construction",
		Repos:   []string{},
		DirName: &dirName,
		Active:  true,
	}})
}

func TestListIntentsLegacyDirectoryMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		dirNameField   string
		recordDir      string
		wantMatch      bool
		wantOrphanSlug string
	}{
		{name: "missing dirName", recordDir: "build-auth-aaaa", wantMatch: true},
		{name: "null dirName", dirNameField: `, "dirName": null`, recordDir: "build-auth-aaaa", wantMatch: true},
		{name: "empty dirName", dirNameField: `, "dirName": ""`, recordDir: "build-auth-aaaa", wantMatch: true},
		{name: "truthy exact mismatch", dirNameField: `, "dirName": "elsewhere"`, recordDir: "build-auth-aaaa", wantOrphanSlug: "build-auth"},
		{name: "uppercase hex suffix", recordDir: "build-auth-AAAA", wantOrphanSlug: "build-auth-AAAA"},
		{name: "non hex suffix", recordDir: "build-auth-zzzz", wantOrphanSlug: "build-auth-zzzz"},
		{name: "uuid suffix mismatch", recordDir: "build-auth-aaab", wantOrphanSlug: "build-auth"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			intentsFS := fstest.MapFS{
				"intents.json": &fstest.MapFile{Data: fmt.Appendf(nil, `[
                    {
                        "uuid": "018f-aaaa",
                        "slug": "build-auth",
                        "status": "construction"%s
                    }
                ]`, tt.dirNameField)},
				tt.recordDir + "/aidlc-state.md": &fstest.MapFile{Data: []byte("state")},
			}

			got, err := ListIntents(intentsFS, new(string))
			if err != nil {
				t.Fatalf("ListIntents() error = %v, want nil", err)
			}
			var wantDir *string
			if tt.wantMatch {
				wantDir = &tt.recordDir
			}
			want := []Intent{{
				UUID:    "018f-aaaa",
				Slug:    "build-auth",
				Status:  "construction",
				Repos:   []string{},
				DirName: wantDir,
			}}
			if tt.wantOrphanSlug != "" {
				want = append(want, Intent{
					Slug: tt.wantOrphanSlug, Status: "unknown", Repos: []string{}, DirName: &tt.recordDir,
				})
			}
			assertIntents(t, got, want)
		})
	}
}

func TestListIntentsRegistryOrderDuplicatesAndOrphans(t *testing.T) {
	t.Parallel()

	claimedDir := "240901-build-auth"
	legacyDir := "legacy-cafe"
	dateDir := "240902-build-api"
	nonBMPDir := "𐀀"
	privateUseDir := "\ue000"
	activeOverride := claimedDir
	intentsFS := fstest.MapFS{
		"intents.json": &fstest.MapFile{Data: []byte(`[
            {"uuid":"first","slug":"first-row","status":"construction","dirName":"240901-build-auth"},
            {"uuid":"second","slug":"second-row","status":"planning","dirName":"240901-build-auth"},
            {"uuid":"third","slug":"registry-only","status":"complete"}
        ]`)},
		claimedDir + "/aidlc-state.md":    &fstest.MapFile{Data: []byte("state")},
		legacyDir + "/aidlc-state.md":     &fstest.MapFile{Data: []byte("state")},
		dateDir + "/aidlc-state.md":       &fstest.MapFile{Data: []byte("state")},
		nonBMPDir + "/aidlc-state.md":     &fstest.MapFile{Data: []byte("state")},
		privateUseDir + "/aidlc-state.md": &fstest.MapFile{Data: []byte("state")},
	}

	got, err := ListIntents(intentsFS, &activeOverride)
	if err != nil {
		t.Fatalf("ListIntents() error = %v, want nil", err)
	}
	assertIntents(t, got, []Intent{
		{UUID: "first", Slug: "first-row", Status: "construction", Repos: []string{}, DirName: &claimedDir, Active: true},
		{UUID: "second", Slug: "second-row", Status: "planning", Repos: []string{}, DirName: &claimedDir, Active: true},
		{UUID: "third", Slug: "registry-only", Status: "complete", Repos: []string{}},
		{Slug: dateDir[len("240902-"):], Status: "unknown", Repos: []string{}, DirName: &dateDir},
		{Slug: legacyDir[:len("legacy")], Status: "unknown", Repos: []string{}, DirName: &legacyDir},
		{Slug: nonBMPDir, Status: "unknown", Repos: []string{}, DirName: &nonBMPDir},
		{Slug: privateUseDir, Status: "unknown", Repos: []string{}, DirName: &privateUseDir},
	})
}

func TestListIntentsActiveResolution(t *testing.T) {
	t.Parallel()

	t.Run("nil override reads cursor", func(t *testing.T) {
		t.Parallel()

		intentsFS := &intentTrackingFS{MapFS: fstest.MapFS{
			"intents.json":              &fstest.MapFile{Data: []byte(`[{"uuid":"one","slug":"one","status":"planning","dirName":"chosen"}]`)},
			"active-intent":             &fstest.MapFile{Data: []byte("chosen")},
			"chosen/aidlc-state.md":     &fstest.MapFile{Data: []byte("state")},
			"not-chosen/aidlc-state.md": &fstest.MapFile{Data: []byte("state")},
		}}

		got, err := ListIntents(intentsFS, nil)
		if err != nil {
			t.Fatalf("ListIntents() error = %v, want nil", err)
		}
		if !got[0].Active {
			t.Errorf("first intent Active = false, want true")
		}
		if !slices.Equal(intentsFS.readPaths, []string{"intents.json", "active-intent"}) {
			t.Errorf("read paths = %q, want registry then active cursor", intentsFS.readPaths)
		}
	})

	t.Run("nil override uses lone-directory fallback", func(t *testing.T) {
		t.Parallel()

		intentsFS := fstest.MapFS{
			"intents.json":        &fstest.MapFile{Data: []byte(`[]`)},
			"only/aidlc-state.md": &fstest.MapFile{Data: []byte("state")},
		}
		got, err := ListIntents(intentsFS, nil)
		if err != nil {
			t.Fatalf("ListIntents() error = %v, want nil", err)
		}
		if len(got) != 1 || !got[0].Active {
			t.Errorf("intents = %#v, want lone orphan active", got)
		}
	})

	t.Run("non-nil empty override suppresses cursor and fallback", func(t *testing.T) {
		t.Parallel()

		intentsFS := &intentTrackingFS{MapFS: fstest.MapFS{
			"intents.json":        &fstest.MapFile{Data: []byte(`[]`)},
			"active-intent":       &fstest.MapFile{Data: []byte("only")},
			"only/aidlc-state.md": &fstest.MapFile{Data: []byte("state")},
		}}
		explicitNone := ""
		got, err := ListIntents(intentsFS, &explicitNone)
		if err != nil {
			t.Fatalf("ListIntents() error = %v, want nil", err)
		}
		if len(got) != 1 || got[0].Active {
			t.Errorf("intents = %#v, want lone orphan inactive", got)
		}
		if !slices.Equal(intentsFS.readPaths, []string{"intents.json"}) {
			t.Errorf("read paths = %q, want only registry", intentsFS.readPaths)
		}
	})
}

func TestListIntentsRegistryFallback(t *testing.T) {
	t.Parallel()

	baseFS := fstest.MapFS{
		"orphan/aidlc-state.md": &fstest.MapFile{Data: []byte("state")},
	}
	tests := []struct {
		name      string
		intentsFS fs.FS
	}{
		{name: "missing registry", intentsFS: baseFS},
		{name: "malformed JSON", intentsFS: fstest.MapFS{
			"intents.json":          &fstest.MapFile{Data: []byte(`{`)},
			"orphan/aidlc-state.md": &fstest.MapFile{Data: []byte("state")},
		}},
		{name: "top level object", intentsFS: fstest.MapFS{
			"intents.json":          &fstest.MapFile{Data: []byte(`{"uuid":"ignored"}`)},
			"orphan/aidlc-state.md": &fstest.MapFile{Data: []byte("state")},
		}},
		{name: "top level null", intentsFS: fstest.MapFS{
			"intents.json":          &fstest.MapFile{Data: []byte(`null`)},
			"orphan/aidlc-state.md": &fstest.MapFile{Data: []byte("state")},
		}},
		{name: "partial data with read error", intentsFS: readFileErrorFS{
			FS: baseFS, data: []byte(`[{"uuid":"discarded","slug":"discarded","status":"planning"}]`), err: fs.ErrPermission,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ListIntents(tt.intentsFS, new(string))
			if err != nil {
				t.Fatalf("ListIntents() error = %v, want nil", err)
			}
			orphan := "orphan"
			assertIntents(t, got, []Intent{{
				Slug: "orphan", Status: "unknown", Repos: []string{}, DirName: &orphan,
			}})
		})
	}
}

func TestListIntentsRejectsInvalidRegistryRows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		row  string
	}{
		{name: "non object null", row: `null`},
		{name: "non object string", row: `"row"`},
		{name: "missing uuid", row: `{"slug":"slug","status":"planning"}`},
		{name: "missing slug", row: `{"uuid":"uuid","status":"planning"}`},
		{name: "missing status", row: `{"uuid":"uuid","slug":"slug"}`},
		{name: "uuid wrong type", row: `{"uuid":1,"slug":"slug","status":"planning"}`},
		{name: "slug null", row: `{"uuid":"uuid","slug":null,"status":"planning"}`},
		{name: "status wrong type", row: `{"uuid":"uuid","slug":"slug","status":true}`},
		{name: "dirName wrong type", row: `{"uuid":"uuid","slug":"slug","status":"planning","dirName":1}`},
		{name: "scope wrong type", row: `{"uuid":"uuid","slug":"slug","status":"planning","scope":{}}`},
		{name: "repos wrong type", row: `{"uuid":"uuid","slug":"slug","status":"planning","repos":"api"}`},
		{name: "repos member wrong type", row: `{"uuid":"uuid","slug":"slug","status":"planning","repos":["api",1]}`},
		{name: "repos null member", row: `{"uuid":"uuid","slug":"slug","status":"planning","repos":["api",null]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			intentsFS := fstest.MapFS{
				"intents.json":          &fstest.MapFile{Data: fmt.Appendf(nil, `[{"uuid":"valid","slug":"valid","status":"planning"},%s]`, tt.row)},
				"orphan/aidlc-state.md": &fstest.MapFile{Data: []byte("state")},
			}
			got, err := ListIntents(intentsFS, new(string))
			if err == nil {
				t.Fatalf("ListIntents() error = nil, want invalid row error; intents = %#v", got)
			}
			if got != nil {
				t.Errorf("ListIntents() intents = %#v, want nil on error", got)
			}
		})
	}
}

func TestListIntentsAcceptsStructuralRegistryValues(t *testing.T) {
	t.Parallel()

	intentsFS := fstest.MapFS{
		"intents.json": &fstest.MapFile{Data: []byte(`[
            {
                "uuid":"",
                "slug":"",
                "status":"",
                "dirName":null,
                "scope":null,
                "repos":null,
                "unknown":{"is":"ignored"}
            },
            {"uuid":"empty-repos","slug":"empty-repos","status":"planning","repos":[]}
        ]`)},
	}
	got, err := ListIntents(intentsFS, new(string))
	if err != nil {
		t.Fatalf("ListIntents() error = %v, want nil", err)
	}
	assertIntents(t, got, []Intent{
		{Repos: []string{}},
		{UUID: "empty-repos", Slug: "empty-repos", Status: "planning", Repos: []string{}},
	})
}

func assertIntents(t *testing.T, got, want []Intent) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Errorf("intents = %#v, want %#v", got, want)
	}
}
