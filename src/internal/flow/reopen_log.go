package flow

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
)

type PendingReopen struct {
	StepID       string `json:"step_id"`
	PlanRevision uint64 `json:"plan_revision"`
	PlanHash     string `json:"plan_hash"`
	Revision     uint64 `json:"revision"`
	From         string `json:"from"`
	To           string `json:"to"`
	Reason       string `json:"reason"`
	At           string `json:"at"`
	LogHash      string `json:"log_hash"`
	LogAfterHash string `json:"log_after_hash"`
	HadLog       bool   `json:"had_log"`
}

func logHash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func (p PendingReopen) block() string {
	return fmt.Sprintf("\n## Reopen revision %d\nUTC: %s\nFrom: %s\nTo: %s\nReason: %q\nConfirmed only when state revision exceeds %d.\n", p.Revision, p.At, p.From, p.To, p.Reason, p.Revision) + fmt.Sprintf("Plan revision: %d\nPlan hash: %s\nReopen step: %s\n", p.PlanRevision, p.PlanHash, p.StepID)
}
func (s Store) reopen(id string, expect uint64, r TransitionRequest) (State, error) {
	if err := s.check(); err != nil {
		return State{}, err
	}
	release, err := filestore.Lock(s.Root, "flow-"+s.Space)
	if err != nil {
		return State{}, err
	}
	defer release()
	st, err := s.Read(id)
	if err != nil {
		return State{}, err
	}
	if st.Revision != expect || expect == ^uint64(0) {
		return State{}, invalid("revision conflict")
	}
	d, err := s.boundDefinition(st)
	if err != nil {
		return State{}, err
	}
	if strings.TrimSpace(r.Reason) == "" || !d.CanReopen(st.Stage, r.Stage) {
		return State{}, invalid("reason and graph reopen candidate required")
	}
	name := "aidlc/spaces/" + s.Space + "/knowledge/log/" + id + "-work-log.md"
	raw, readErr := readWorkLog(s.Root, name)
	if readErr != nil && !os.IsNotExist(readErr) {
		return State{}, readErr
	}
	if st.PendingReopen == nil {
		if err = s.guardReassignment(st, nil); err != nil {
			return State{}, err
		}

		pending := &PendingReopen{
			Revision: expect, From: st.Stage, To: r.Stage, Reason: r.Reason,
			At: time.Now().UTC().Format(time.RFC3339Nano), HadLog: true,
		}
		if os.IsNotExist(readErr) {
			raw, err = workLogBytes(st, pending.At)
			if err != nil {
				return State{}, err
			}
		}
		pending.LogHash = logHash(raw)
		completed, err := appendWorkLog(st, raw, *pending)
		if err != nil {
			return State{}, err
		}
		pending.LogAfterHash = logHash(completed)

		if os.IsNotExist(readErr) {
			write := s.write
			if write == nil {
				write = filestore.WriteFile
			}
			if err = write(s.Root, name, raw); err != nil {
				return State{}, err
			}
			readErr = nil
		}
		st.PendingReopen = pending
		if err = s.persist(st); err != nil {
			return State{}, err
		}
	}
	p := st.PendingReopen
	if p.Revision != expect || p.From != st.Stage || p.To != r.Stage || p.Reason != r.Reason {
		return State{}, invalid("different reopen pending")
	}
	before := readErr == nil && logHash(raw) == p.LogHash
	after := readErr == nil && logHash(raw) == p.LogAfterHash
	if !before && !after {
		return State{}, invalid("work-log changed or missing; restore the recorded previous version")
	}
	if before {
		completed, err := appendWorkLog(st, raw, *p)
		if err != nil {
			return State{}, err
		}
		if logHash(completed) != p.LogAfterHash {
			return State{}, invalid("work-log completed hash mismatch")
		}
		write := s.write
		if write == nil {
			write = filestore.WriteFile
		}
		if err = write(s.Root, name, completed); err != nil {
			return State{}, err
		}
	}
	for stage := range st.Accepted {
		if stage == r.Stage || d.Before(r.Stage, stage) {
			delete(st.Accepted, stage)
		}
	}
	st.Entry = nil
	st.Sensor = Gate{}
	st.Review = Gate{}
	st.Stage = r.Stage
	st.Status = "active"
	st.Reason = r.Reason
	st.ResumeCondition = ""
	st.PendingReopen = nil
	st.Revision++
	if err = s.persist(st); err != nil {
		return State{}, err
	}
	return st, nil
}

func workLogBytes(st State, at string) ([]byte, error) {
	now, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return nil, err
	}
	kind, title, description := "work-log", st.Name+" work-log", "Intentの差戻し記録"
	input := okfmemory.MetadataInput{
		Type: &kind, Title: &title, Description: &description,
		IntentID: &st.ID, Tags: []string{"work-log"}, Actor: "process:aidlc",
	}
	body := []byte{}
	doc, err := okfmemory.BuildMetadata(nil, body, input, now)
	if err != nil {
		return nil, err
	}
	return doc.Bytes()
}

func appendWorkLog(st State, raw []byte, p PendingReopen) ([]byte, error) {
	old, err := okfmemory.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("read work-log: %w", err)
	}
	if old.String("type") != "work-log" || old.String("intent_id") != st.ID {
		return nil, invalid("work-log type or intent mismatch")
	}
	now, err := time.Parse(time.RFC3339Nano, p.At)
	if err != nil {
		return nil, err
	}
	doc, err := okfmemory.BuildMetadata(
		&old,
		[]byte(old.Body+p.block()),
		okfmemory.MetadataInput{Actor: "process:aidlc"},
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("append work-log: %w", err)
	}
	return doc.Bytes()
}

// readWorkLog checks the leaf before opening it so a FIFO cannot block reopen.
// filestore additionally rejects symlinks in every parent component.
func readWorkLog(root, name string) ([]byte, error) {
	directory, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	info, err := directory.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, invalid("work-log is not a regular file")
	}
	return filestore.ReadFile(root, name)
}
