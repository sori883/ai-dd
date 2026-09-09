package flow

import (
	"bytes"
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"io"
	"os"
	"reflect"
)

type HistoryRecord struct {
	SchemaVersion int    `json:"schema_version"`
	Previous      string `json:"previous"`
	State         State  `json:"state"`
}

func (s Store) historyPath(id, hash string) string {
	return "aidlc/spaces/" + s.Space + "/intents/" + id + "/history/" + hash + ".json"
}
func (s Store) commit(st *State) error {
	old, err := s.Read(st.ID)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	changed := os.IsNotExist(err) || !reflect.DeepEqual(old.ExecutionPlan, st.ExecutionPlan) || !reflect.DeepEqual(old.Approval, st.Approval) || old.Status != st.Status || old.CurrentStepID != st.CurrentStepID
	if changed {
		snapshot := *st
		snapshot.HistoryHead = ""
		record := HistoryRecord{SchemaVersion: 1, Previous: st.HistoryHead, State: snapshot}
		raw, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if len(raw) > filestore.MaxBytes {
			return invalid("history exceeds 256 KiB")
		}
		head := filestore.Hash(raw)
		candidate := *st
		candidate.HistoryHead = head
		stateRaw, err := json.MarshalIndent(candidate, "", "  ")
		if err != nil {
			return err
		}
		if len(stateRaw)+1 > filestore.MaxBytes {
			return invalid("state exceeds 256 KiB")
		}
		if err = s.validate(candidate); err != nil {
			return err
		}
		existing, readErr := filestore.ReadFile(s.Root, s.historyPath(st.ID, head))
		if readErr == nil && !bytes.Equal(existing, raw) {
			return invalid("immutable history conflict")
		}
		if readErr != nil && !os.IsNotExist(readErr) {
			return readErr
		}
		if os.IsNotExist(readErr) {
			write := s.write
			if write == nil {
				write = filestore.WriteFile
			}
			if err = write(s.Root, s.historyPath(st.ID, head), raw); err != nil {
				return err
			}
		}
		st.HistoryHead = head
	}
	return s.persist(*st)
}
func (s Store) History(id string) ([]HistoryRecord, error) {
	st, err := s.Read(id)
	if err != nil {
		return nil, err
	}
	head := st.HistoryHead
	seen := map[string]bool{}
	records := []HistoryRecord{}
	for head != "" {
		if seen[head] {
			return nil, invalid("history cycle")
		}
		seen[head] = true
		raw, err := filestore.ReadFile(s.Root, s.historyPath(id, head))
		if err != nil {
			return nil, err
		}
		if filestore.Hash(raw) != head {
			return nil, invalid("history hash mismatch")
		}
		if err = uniqueJSON(json.NewDecoder(bytes.NewReader(raw))); err != nil {
			return nil, err
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		var r HistoryRecord
		if err = decoder.Decode(&r); err != nil {
			return nil, err
		}
		if decoder.Decode(new(any)) != io.EOF {
			return nil, invalid("invalid history JSON")
		}
		if (r.Previous != "" && !validHash(r.Previous)) || r.SchemaVersion != 1 || r.State.ID != id || r.State.HistoryHead != "" || r.State.Revision > st.Revision {
			return nil, invalid("invalid history identity")
		}
		if err = s.validate(r.State); err != nil {
			return nil, err
		}
		records = append(records, r)
		head = r.Previous
	}
	return records, nil
}
