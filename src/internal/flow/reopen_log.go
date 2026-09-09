package flow

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sori883/ai-dd/src/internal/filestore"
)

type PendingReopen struct {
	Revision uint64 `json:"revision"`
	From     string `json:"from"`
	To       string `json:"to"`
	Reason   string `json:"reason"`
	At       string `json:"at"`
	LogHash  string `json:"log_hash"`
	HadLog   bool   `json:"had_log"`
}

func logHash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func (p PendingReopen) block() string {
	return fmt.Sprintf("\n## Reopen revision %d\nUTC: %s\nFrom: %s\nTo: %s\nReason: %q\nConfirmed only when state revision exceeds %d.\n", p.Revision, p.At, p.From, p.To, p.Reason, p.Revision)
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
	name := strings.TrimSuffix(s.path(id), "state.json") + "work-log.md"
	raw, readErr := filestore.ReadFile(s.Root, name)
	if readErr != nil && !os.IsNotExist(readErr) {
		return State{}, readErr
	}
	if st.PendingReopen == nil {
		if err = s.guardReassignment(st, nil); err != nil {
			return State{}, err
		}

		if os.IsNotExist(readErr) {
			write := s.write
			if write == nil {
				write = filestore.WriteFile
			}
			if err = write(s.Root, name, []byte{}); err != nil {
				return State{}, err
			}
			raw = []byte{}
			readErr = nil
		}
		st.PendingReopen = &PendingReopen{Revision: expect, From: st.Stage, To: r.Stage, Reason: r.Reason, At: time.Now().UTC().Format(time.RFC3339Nano), LogHash: logHash(raw), HadLog: readErr == nil}
		if err = s.persist(st); err != nil {
			return State{}, err
		}
	}
	p := st.PendingReopen
	if p.Revision != expect || p.From != st.Stage || p.To != r.Stage || p.Reason != r.Reason {
		return State{}, invalid("different reopen pending")
	}
	block := p.block()
	before := logHash(raw) == p.LogHash && (readErr == nil || !p.HadLog)
	after := readErr == nil && strings.HasSuffix(string(raw), block) && logHash(raw[:len(raw)-len(block)]) == p.LogHash
	if !before && !after {
		return State{}, invalid("work-log changed or missing; restore the recorded previous version")
	}
	if before {
		write := s.write
		if write == nil {
			write = filestore.WriteFile
		}
		if err = write(s.Root, name, append(raw, []byte(block)...)); err != nil {
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
