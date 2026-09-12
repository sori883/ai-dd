package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/flow"
)

func TestOKFWorkLogSearchAndShow(t *testing.T) {
	s, st := setup(t)
	call := func(args ...string) []byte {
		t.Helper()
		request, err := cli.ParseCommand(args)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := s.Execute(request)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return raw
	}
	call("intent", "reopen", st.ID, "--space", "default", "--expect", strconv.FormatUint(st.Revision, 10), "--step", "s02", "--reason", "searchable reopen reason")
	store := flow.Store{Root: s.Root, Space: "default"}
	st, _ = store.Read(st.ID)
	st = approveFixturePlan(t, store, st)
	other, err := (flow.Store{Root: s.Root, Space: "default"}).Create("Other")
	if err != nil {
		t.Fatal(err)
	}
	other = executionFixtureState(t, store, other, "discovery")
	call("intent", "reopen", other.ID, "--space", "default", "--expect", strconv.FormatUint(other.Revision, 10), "--step", "s02", "--reason", "other reason")
	other, _ = store.Read(other.ID)
	other = approveFixturePlan(t, store, other)
	if err = os.MkdirAll(filepath.Join(s.Root, "aidlc/spaces/foreign/knowledge"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, space, id string
		count           int
	}{
		{name: "matching intent", space: "default", id: st.ID, count: 1},
		{name: "other intent", space: "default", id: other.ID, count: 1},
		{name: "unknown intent", space: "default", id: strings.Repeat("f", 32), count: 0},
		{name: "foreign space", space: "foreign", id: st.ID, count: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := call("memory", "search", "work-log", "--space", tc.space, "--intent-id", tc.id)
			var rows []map[string]string
			if err := json.Unmarshal(raw, &rows); err != nil {
				t.Fatal(err)
			}
			if len(rows) != tc.count {
				t.Fatalf("search %s", raw)
			}
			if tc.count == 1 && (rows[0]["intent_id"] != tc.id || rows[0]["concept_id"] != "log/"+tc.id+"-work-log") {
				t.Fatalf("wrong match %s", raw)
			}
		})
	}
	raw := call("memory", "show", "log/"+st.ID+"-work-log", "--space", "default")
	var shown map[string]string
	if err = json.Unmarshal(raw, &shown); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shown["content"], "searchable reopen reason") || len(shown["hash"]) != 64 {
		t.Fatalf("show %s", raw)
	}
}
