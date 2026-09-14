package cli

import (
	"slices"
	"testing"
)

func TestWorkspaceArguments(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		allowJSON bool
		dir       string
		json      bool
	}{
		{"first split literal", []string{"--project-dir", " path ", "space", "list"}, true, " path ", false},
		{"middle equals", []string{"space", "--project-dir=path", "list"}, true, "path", false},
		{"end split", []string{"space", "list", "--project-dir", "path"}, true, "path", false},
		{"JSON before", []string{"--json", "--project-dir=path", "space", "list"}, true, "path", true},
		{"JSON after", []string{"space", "list", "--project-dir", "path", "--json"}, true, "path", true},
		{"without JSON", []string{"space", "--project-dir", "path", "list"}, false, "path", false},
		{"equals dash literal", []string{"space", "list", "--project-dir=-dir"}, true, "-dir", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command, dir, json, err := workspaceArguments(tc.args, tc.allowJSON)
			if err != nil || !slices.Equal(command, []string{"space", "list"}) || dir != tc.dir || json != tc.json {
				t.Fatalf("%v %q %v %v", command, dir, json, err)
			}
		})
	}
	for _, tc := range []struct {
		name      string
		args      []string
		allowJSON bool
	}{
		{"unknown", []string{"--force"}, true},
		{"missing path", []string{"--project-dir"}, true},
		{"empty path", []string{"--project-dir="}, true},
		{"duplicate path", []string{"--project-dir=a", "--project-dir", "b"}, true},
		{"split dash path", []string{"--project-dir", "-dir"}, true},
		{"duplicate JSON", []string{"--json", "--json"}, true},
		{"JSON disabled", []string{"--json"}, false},
		{"JSON value", []string{"--json=true"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, _, err := workspaceArguments(tc.args, tc.allowJSON); err == nil {
				t.Fatal("invalid arguments accepted")
			}
		})
	}
}
