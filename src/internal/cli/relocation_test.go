package cli

import (
	"strings"
	"testing"
)

func TestRelocationCLI(t *testing.T) {
	valid := []string{"install", "codex", "--relocate", "--project-dir", "/new", "--from-project-dir", "/old/missing", "--from-binary", "/old/aidlc"}
	if _, err := ParseMinimal(valid); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseMinimal([]string{"unit", "reassign", strings.Repeat("a", 32), "--space", "default", "--expect", "1", "--file", "request.json"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
	}{{"old without relocate", []string{"install", "codex", "--project-dir", "/new", "--from-project-dir", "/old", "--from-binary", "/binary"}}, {"missing old binary", valid[:len(valid)-2]}, {"duplicate", append(append([]string{}, valid...), "--relocate")}, {"relative", []string{"install", "codex", "--relocate", "--project-dir", "/new", "--from-project-dir", "relative", "--from-binary", "/binary"}}} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseMinimal(tc.args); err == nil {
				t.Fatal("accepted invalid relocate")
			}
		})
	}
	for _, tc := range []struct {
		action []string
		words  []string
	}{{[]string{"install", "codex", "--help"}, []string{"--relocate", "--from-project-dir", "--from-binary", "hooks.json", "trust", "部分", "再試行"}}, {[]string{"unit", "reassign", "--help"}, []string{"previous_run_stopped", "true", "needs_confirmation", "reason", "run_id", "再試行"}}} {
		text, ok := Help(tc.action)
		if !ok {
			t.Fatal("missing help")
		}
		for _, word := range tc.words {
			if !strings.Contains(text, word) {
				t.Errorf("help lacks %s", word)
			}
		}
	}
}

func TestRelocationCLIRootHelp(t *testing.T) {
	text, ok := Help([]string{"--help"})
	if !ok || !strings.Contains(text, "--from-project-dir") || !strings.Contains(text, "confirm|reassign") {
		t.Fatal("root help omits relocation operations")
	}
}

func TestRelocationCLIEmptySourceFlags(t *testing.T) {
	for _, flag := range []string{"--from-project-dir", "--from-binary"} {
		t.Run(flag, func(t *testing.T) {
			if _, err := ParseMinimal([]string{"install", "codex", "--project-dir", "/new", flag, ""}); err == nil {
				t.Fatal("source flag mixed into normal install")
			}
		})
	}
}
