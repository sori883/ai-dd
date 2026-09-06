package cli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/okf"
)

func TestKnowledgeSearchPublicContract(t *testing.T) {
	var stdout, stderr bytes.Buffer
	calls := 0
	code := cli.Run([]string{"knowledge", "search", "--tag", "One", "--tag=Two", "--type", "A", "--type=B", "--query", "日本語", "--limit", "10", "--project-dir", "/project"}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
		SearchKnowledge: func(options okf.SearchOptions, dir string) (okf.SearchResult, error) {
			calls++
			if dir != "/project" || len(options.Tags) != 2 || len(options.Types) != 2 || options.Query != "日本語" || !options.HasQuery || options.Limit != 10 {
				t.Fatalf("request %+v %q", options, dir)
			}
			return okf.SearchResult{Results: []okf.Result{{ConceptID: "a", Path: "bundle/a.md", Type: "A", Title: "<&>", Tags: []string{}, Status: "stable", TrustTier: "unverified"}}, Warnings: []okf.Warning{}}, nil
		},
	})
	want := "{\"results\":[{\"concept_id\":\"a\",\"path\":\"bundle/a.md\",\"type\":\"A\",\"title\":\"<&>\",\"description\":\"\",\"resource\":\"\",\"tags\":[],\"status\":\"stable\",\"generated_at\":null,\"stale\":false,\"trust_tier\":\"unverified\"}],\"warnings\":[]}\n"
	if code != 0 || calls != 1 || stderr.Len() != 0 || stdout.String() != want {
		t.Fatalf("code %d calls %d stdout %s stderr %s", code, calls, &stdout, &stderr)
	}
}

func TestKnowledgeSearchSyntax(t *testing.T) {
	for _, args := range [][]string{{"knowledge"}, {"knowledge", "other"}, {"knowledge", "search"}, {"knowledge", "search", "--query", "!?"}, {"knowledge", "search", "--tag"}, {"knowledge", "search", "--type=A", "--limit=0"}, {"knowledge", "search", "--type=A", "--limit=101"}, {"knowledge", "search", "--type=A", "--query=a", "--query=b"}, {"knowledge", "search", "--type=A", "extra"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, errout bytes.Buffer
			code := cli.Run(args, &out, &errout, buildinfo.Info{}, cli.Dependencies{SearchKnowledge: func(okf.SearchOptions, string) (okf.SearchResult, error) {
				t.Fatal("called invalid request")
				return okf.SearchResult{}, nil
			}})
			if code != 2 || out.Len() != 0 || !strings.HasPrefix(errout.String(), "aidlc:") {
				t.Fatalf("code %d out %s err %s", code, &out, &errout)
			}
		})
	}
}

func TestKnowledgeSearchRuntimeAndEmpty(t *testing.T) {
	var out, errout bytes.Buffer
	dep := cli.Dependencies{SearchKnowledge: func(okf.SearchOptions, string) (okf.SearchResult, error) {
		return okf.SearchResult{}, errors.New("root failed")
	}}
	if code := cli.Run([]string{"knowledge", "search", "--tag=A"}, &out, &errout, buildinfo.Info{}, dep); code != 1 || out.Len() != 0 || !strings.HasPrefix(errout.String(), "aidlc:") {
		t.Fatalf("code %d out %s err %s", code, &out, &errout)
	}
	out.Reset()
	errout.Reset()
	dep.SearchKnowledge = func(options okf.SearchOptions, _ string) (okf.SearchResult, error) {
		if options.Limit != 4 {
			t.Fatalf("default %d", options.Limit)
		}
		return okf.SearchResult{}, nil
	}
	if code := cli.Run([]string{"knowledge", "search", "--tag=A"}, &out, &errout, buildinfo.Info{}, dep); code != 0 || out.String() != "{\"results\":[],\"warnings\":[]}\n" {
		t.Fatalf("empty code %d out %s", code, &out)
	}
}
