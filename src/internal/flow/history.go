package flow

import (
	"bytes"
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"io"
	"os"
	"reflect"
	"unicode/utf8"
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
	if err == nil {
		if err = s.verifyHistoryHead(old); err != nil {
			return err
		}
	}
	changed := os.IsNotExist(err) || !reflect.DeepEqual(old.ExecutionPlan, st.ExecutionPlan) || !reflect.DeepEqual(old.Approval, st.Approval) || old.Status != st.Status || old.CurrentStepID != st.CurrentStepID
	if changed {
		snapshot := *st
		snapshot.HistoryHead = ""
		snapshot.HistoryRevision = 0
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
		candidate.HistoryRevision = st.Revision
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
		st.HistoryRevision = st.Revision
	}
	return s.persist(*st)
}
func (s Store) readHistoryRecord(id, head string) (HistoryRecord, error) {
	var r HistoryRecord
	raw, err := filestore.ReadFile(s.Root, s.historyPath(id, head))
	if err != nil {
		return r, err
	}
	if !utf8.Valid(raw) {
		return r, invalid("invalid history UTF-8")
	}
	if filestore.Hash(raw) != head {
		return r, invalid("history hash mismatch")
	}
	if err = uniqueJSON(json.NewDecoder(bytes.NewReader(raw))); err != nil {
		return r, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&r); err != nil {
		return r, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return r, invalid("invalid history JSON")
	}
	if (r.Previous != "" && !validHash(r.Previous)) || r.SchemaVersion != 1 || r.State.ID != id || r.State.HistoryHead != "" || r.State.HistoryRevision != 0 {
		return r, invalid("invalid history identity")
	}
	if err = s.validate(r.State); err != nil {
		return r, err
	}
	return r, nil
}

// verifyHistoryHead checks the committed anchor without traversing old records.
func (s Store) verifyHistoryHead(st State) error {
	if st.HistoryHead == "" || st.HistoryRevision == 0 || st.HistoryRevision > st.Revision {
		return invalid("committed history anchor required")
	}
	record, err := s.readHistoryRecord(st.ID, st.HistoryHead)
	if err != nil {
		return err
	}
	if record.State.Revision != st.HistoryRevision {
		return invalid("history anchor revision mismatch")
	}
	return nil
}
func (s Store) History(id string) ([]HistoryRecord, error) {
	st, err := s.Read(id)
	if err != nil {
		return nil, err
	}
	if err = s.verifyHistoryHead(st); err != nil {
		return nil, err
	}
	head := st.HistoryHead
	seen := map[string]bool{}
	records := []HistoryRecord{}
	previousRevision := uint64(0)
	for head != "" {
		if seen[head] {
			return nil, invalid("history cycle")
		}
		seen[head] = true
		record, err := s.readHistoryRecord(id, head)
		if err != nil {
			return nil, err
		}
		if len(records) > 0 && record.State.Revision >= previousRevision {
			return nil, invalid("history revisions must decrease")
		}
		if record.State.Revision > st.Revision {
			return nil, invalid("future history revision")
		}
		previousRevision = record.State.Revision
		records = append(records, record)
		head = record.Previous
	}
	return records, nil
}
