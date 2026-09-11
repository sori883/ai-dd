package flow

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestVerificationDigest(t *testing.T) {
	write := func(t *testing.T, root, p, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	digest := func(t *testing.T, root string, paths ...string) VerificationDigest {
		t.Helper()
		d, err := ComputeVerification(root, paths)
		if err != nil {
			t.Fatal(err)
		}
		if len(d.SHA256) != 64 || d.Version != 1 {
			t.Fatalf("invalid digest: %+v", d)
		}
		return d
	}
	t.Run("stable content", func(t *testing.T) {
		a, b := t.TempDir(), t.TempDir()
		for _, r := range []string{a, b} {
			write(t, r, "src/a", "a\x00b")
			write(t, r, "config", "c")
		}
		first := digest(t, a, "src", "config", "missing")
		second := digest(t, b, "missing", "config", "src", "src")
		if !reflect.DeepEqual(first, second) || first.Files != 2 || first.Bytes != 4 || !reflect.DeepEqual(first.Missing, []string{"missing"}) {
			t.Fatalf("not stable: %+v / %+v", first, second)
		}
		if err := os.Chtimes(filepath.Join(a, "src/a"), time.Unix(12, 0), time.Unix(12, 0)); err != nil {
			t.Fatal(err)
		}
		if got := digest(t, a, "src", "config", "missing"); got.SHA256 != first.SHA256 {
			t.Fatal("mtime changed digest")
		}
		alias := filepath.Join(t.TempDir(), "alias")
		if err := os.Symlink(a, alias); err != nil {
			t.Fatal(err)
		}
		if digest(t, alias, "src", "config", "missing").SHA256 != first.SHA256 {
			t.Fatal("root alias changed digest")
		}
	})
	for _, change := range []string{"bytes", "add", "delete", "rename", "missing", "scope"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "src/a", "before")
			paths := []string{"src", "missing"}
			before := digest(t, root, paths...)
			switch change {
			case "bytes":
				write(t, root, "src/a", "after")
			case "add":
				write(t, root, "src/b", "b")
			case "delete":
				if err := os.Remove(filepath.Join(root, "src/a")); err != nil {
					t.Fatal(err)
				}
			case "rename":
				if err := os.Rename(filepath.Join(root, "src/a"), filepath.Join(root, "src/b")); err != nil {
					t.Fatal(err)
				}
			case "missing":
				write(t, root, "missing", "new")
			case "scope":
				paths = []string{"src"}
			}
			if digest(t, root, paths...).SHA256 == before.SHA256 {
				t.Fatal("change not detected")
			}
		})
	}
	t.Run("managed exclusions", func(t *testing.T) {
		r := t.TempDir()
		write(t, r, "src/a", "a")
		before := digest(t, r, ".")
		for _, p := range []string{"aidlc/result.json", ".git/config", "src/.git/config"} {
			write(t, r, p, "x")
		}
		if digest(t, r, ".").SHA256 != before.SHA256 {
			t.Fatal("managed content affected code digest")
		}
		for _, p := range []string{"aidlc", "aidlc/result.json", ".git", "src/.git/config", "../escape", "/absolute"} {
			if _, err := ComputeVerification(r, []string{p}); err == nil {
				t.Fatalf("accepted %s", p)
			}
		}
	})
	t.Run("symlink", func(t *testing.T) {
		r := t.TempDir()
		write(t, r, "src/a", "a")
		if err := os.Symlink("a", filepath.Join(r, "src/link")); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{"src", "src/link", "src/link/child"} {
			if _, err := ComputeVerification(r, []string{p}); err == nil {
				t.Fatalf("accepted symlink %s", p)
			}
		}
	})
	t.Run("special file", func(t *testing.T) {
		r, err := os.MkdirTemp("", "vd-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(r) })
		listener, err := net.Listen("unix", filepath.Join(r, "socket"))
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		if _, err := ComputeVerification(r, []string{"."}); err == nil {
			t.Fatal("accepted socket")
		}
	})
	t.Run("limits and changed read", func(t *testing.T) {
		r := t.TempDir()
		write(t, r, "a", "abc")
		write(t, r, "b", "def")
		for _, options := range []verificationOptions{{maxFiles: 1, maxBytes: 100}, {maxFiles: 10, maxBytes: 5}, {maxFiles: 10, maxBytes: 100, between: func() { write(t, r, "a", "changed") }}} {
			if _, err := computeVerification(r, []string{"."}, options); err == nil {
				t.Fatal("accepted partial or unstable digest")
			}
		}
	})
}

func verificationTestSHA(t *testing.T, root string, paths []string) string {
	t.Helper()
	digest, err := ComputeVerification(root, paths)
	if err != nil {
		t.Fatal(err)
	}
	return digest.SHA256
}

// prepareUnitResultFixture creates synthetic command output for tests of Unit
// lifecycle, whose subject is binding/persistence rather than test execution.
func prepareUnitResultFixture(t *testing.T, s Store, st State, r UnitRequest) UnitRequest {
	t.Helper()
	var unit Unit
	for _, u := range st.Config.Units {
		if u.ID == r.Unit {
			unit = u
		}
	}
	r.VerificationSHA256 = verificationTestSHA(t, r.Root, unitVerificationPaths(st.Config, unit))
	if r.Action == "confirm" {
		return r
	}
	zero := 0
	runs := []resultRun{}
	for _, command := range unit.Tests {
		runs = append(runs, resultRun{UnitID: unit.ID, Command: command, ExitCode: &zero, OutputPath: "aidlc/evidence/unit-output.txt"})
	}
	doc := resultDocument{StepID: st.CurrentStepID, Stage: st.Stage, VerificationScope: "unit", VerificationSHA256: r.VerificationSHA256, UnitID: unit.ID, RunID: r.RunID, Runs: runs}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	worker := Store{Root: r.Root, Space: s.Space}
	boundaryFile(t, worker, "aidlc/evidence/unit-output.txt", "synthetic successful command output")
	boundaryFile(t, worker, "aidlc/evidence/unit.json", string(raw))
	return r
}
