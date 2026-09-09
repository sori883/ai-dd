package flow

import (
	"bytes"
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"os"
	"path/filepath"
	"testing"
)

func TestExecutionPlanReviewHistoryMutation(t *testing.T) {
	for _, mode := range []string{"missing", "hash", "old head", "invalid record", "invalid UTF8"} {
		for _, action := range []string{"configure", "pause", "source"} {
			t.Run(mode+"/"+action, func(t *testing.T) {
				s := executionFixture(t)
				st, err := s.Create("history")
				if err != nil {
					t.Fatal(err)
				}
				first := st.HistoryHead
				st, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "pause", Reason: "paused"})
				if err != nil {
					t.Fatal(err)
				}
				st, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "resume", Reason: "resume"})
				if err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "missing":
					err = os.Remove(filepath.Join(s.Root, s.historyPath(st.ID, st.HistoryHead)))
				case "hash":
					err = filestore.WriteFile(s.Root, s.historyPath(st.ID, st.HistoryHead), []byte("{}"))
				case "invalid record", "invalid UTF8":
					raw, readErr := filestore.ReadFile(s.Root, s.historyPath(st.ID, st.HistoryHead))
					if readErr != nil {
						t.Fatal(readErr)
					}
					var record HistoryRecord
					if err = json.Unmarshal(raw, &record); err != nil {
						t.Fatal(err)
					}
					if mode == "invalid record" {
						record.State.Status = "invented"
					}
					raw, _ = json.Marshal(record)
					if mode == "invalid UTF8" {
						raw = bytes.Replace(raw, []byte(`"name":"history"`), []byte("\"name\":\"bad"+string([]byte{255})+"\""), 1)
					}
					st.HistoryHead = filestore.Hash(raw)
					if err = filestore.WriteFile(s.Root, s.historyPath(st.ID, st.HistoryHead), raw); err != nil {
						t.Fatal(err)
					}
					raw, _ = json.Marshal(st)
					err = filestore.WriteFile(s.Root, s.path(st.ID), raw)
				case "old head":
					st.HistoryHead = first
					raw, _ := json.Marshal(st)
					err = filestore.WriteFile(s.Root, s.path(st.ID), raw)
				}
				if err != nil {
					t.Fatal(err)
				}
				before, err := filestore.ReadFile(s.Root, s.path(st.ID))
				if err != nil {
					t.Fatal(err)
				}
				writes := 0
				s.write = func(root, name string, raw []byte) error { writes++; return filestore.WriteFile(root, name, raw) }
				switch action {
				case "configure":
					st.Config.Objective = "change"
					_, err = s.Save(st, st.Revision)
				case "pause":
					_, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "pause", Reason: "again"})
				case "source":
					err = s.CaptureApproval(st.ID, "user", "", "A", "approve")
				}
				if err == nil {
					t.Error("corrupt history allowed mutation")
				}
				after, _ := filestore.ReadFile(s.Root, s.path(st.ID))
				if string(before) != string(after) || writes != 0 {
					t.Error("rejection wrote state/history/log")
				}
				if _, err = os.Stat(filepath.Join(s.Root, s.approvalSourcePath(st.ID))); !os.IsNotExist(err) {
					t.Error("rejection wrote source")
				}
			})
		}
	}
}
func TestExecutionPlanReviewHistoryRevisionOrder(t *testing.T) {
	s := executionFixture(t)
	st, err := s.Create("history")
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "pause", Reason: "paused"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := filestore.ReadFile(s.Root, s.historyPath(st.ID, st.HistoryHead))
	if err != nil {
		t.Fatal(err)
	}
	var record HistoryRecord
	if err = json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	record.State.Revision = 1
	raw, _ = json.Marshal(record)
	st.HistoryHead = filestore.Hash(raw)
	st.HistoryRevision = 1
	if err = filestore.WriteFile(s.Root, s.historyPath(st.ID, st.HistoryHead), raw); err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(st)
	if err = filestore.WriteFile(s.Root, s.path(st.ID), raw); err != nil {
		t.Fatal(err)
	}
	if _, err = s.History(st.ID); err == nil {
		t.Fatal("non-decreasing history revision accepted")
	}
}
func TestExecutionPlanReviewHistoryConfigKeepsHead(t *testing.T) {
	s := executionFixture(t)
	st, err := s.Create("history")
	if err != nil {
		t.Fatal(err)
	}
	head := st.HistoryHead
	revision := st.Revision
	st.Config.Objective = "updated"
	st, err = s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if st.HistoryHead != head || st.HistoryRevision != revision {
		t.Fatal("config update lost committed anchor")
	}
	if _, err = s.History(st.ID); err != nil {
		t.Fatal(err)
	}
}
