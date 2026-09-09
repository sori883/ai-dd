package flow

import "github.com/sori883/ai-dd/src/internal/workflow"

func (s Store) definition() (workflow.Definition, error) { return workflow.Load(s.Root) }
func supportedStage(stage string) bool {
	switch stage {
	case "discovery", "planning", "tdd", "integration":
		return true
	}
	return false
}

func (s Store) boundDefinition(st State) (workflow.Definition, error) {
	d, err := s.definition()
	if err != nil {
		return d, err
	}
	if st.DefinitionHash != d.Hash {
		return d, invalid("workflow definition changed; restore the original definition or create a new Intent")
	}
	return d, nil
}
func (s Store) guardWorkflow(st State) error {
	if _, err := s.boundDefinition(st); err != nil {
		return err
	}
	if st.PendingReopen != nil {
		return invalid("reopen save pending; retry the identical request")
	}
	return nil
}
