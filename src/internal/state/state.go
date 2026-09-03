package state

import "slices"

// WorkflowStatus is the lifecycle status of an AI-DLC workflow.
type WorkflowStatus string

const (
	// WorkflowStatusUnknown is the zero value and is not valid in a parsed state.
	WorkflowStatusUnknown WorkflowStatus = ""
	// WorkflowStatusRunning means that the workflow is in progress.
	WorkflowStatusRunning WorkflowStatus = "Running"
	// WorkflowStatusCompleted means that the workflow has finished.
	WorkflowStatusCompleted WorkflowStatus = "Completed"
)

// LifecyclePhase identifies the current high-level AI-DLC phase.
type LifecyclePhase string

const (
	// LifecyclePhaseUnknown is the zero value and is not valid in a parsed state.
	LifecyclePhaseUnknown LifecyclePhase = ""
	// LifecyclePhaseInitialization is the initialization phase.
	LifecyclePhaseInitialization LifecyclePhase = "INITIALIZATION"
	// LifecyclePhaseIdeation is the ideation phase.
	LifecyclePhaseIdeation LifecyclePhase = "IDEATION"
	// LifecyclePhaseInception is the inception phase.
	LifecyclePhaseInception LifecyclePhase = "INCEPTION"
	// LifecyclePhaseConstruction is the construction phase.
	LifecyclePhaseConstruction LifecyclePhase = "CONSTRUCTION"
	// LifecyclePhaseOperation is the operation phase.
	LifecyclePhaseOperation LifecyclePhase = "OPERATION"
)

// PhaseStatus is the display status of one lifecycle phase.
type PhaseStatus string

const (
	// PhaseStatusUnknown is the zero value and is not valid in a parsed state.
	PhaseStatusUnknown PhaseStatus = ""
	// PhaseStatusPending means that a phase has not started yet.
	PhaseStatusPending PhaseStatus = "Pending"
	// PhaseStatusActive means that a phase is currently active.
	PhaseStatusActive PhaseStatus = "Active"
	// PhaseStatusVerified means that a phase has been completed and verified.
	PhaseStatusVerified PhaseStatus = "Verified"
	// PhaseStatusSkipped means that a phase is omitted by the plan.
	PhaseStatusSkipped PhaseStatus = "Skipped"
)

// CheckboxState is the normalized meaning of a stage progress checkbox.
type CheckboxState string

const (
	// CheckboxStateUnknown is the zero value and is not valid in a parsed state.
	CheckboxStateUnknown CheckboxState = ""
	// CheckboxStatePending means that a stage has not started.
	CheckboxStatePending CheckboxState = "pending"
	// CheckboxStateInProgress means that a stage is currently in progress.
	CheckboxStateInProgress CheckboxState = "in-progress"
	// CheckboxStateAwaitingApproval means that a stage is awaiting approval.
	CheckboxStateAwaitingApproval CheckboxState = "awaiting-approval"
	// CheckboxStateRevising means that a stage is being revised.
	CheckboxStateRevising CheckboxState = "revising"
	// CheckboxStateCompleted means that a stage is complete.
	CheckboxStateCompleted CheckboxState = "completed"
	// CheckboxStateSkipped means that a stage was skipped.
	CheckboxStateSkipped CheckboxState = "skipped"
)

// PlanAction is the execution action recorded for a stage.
type PlanAction string

const (
	// PlanActionUnknown is the zero value and is not valid in a parsed state.
	PlanActionUnknown PlanAction = ""
	// PlanActionExecute includes a stage in the execution plan.
	PlanActionExecute PlanAction = "EXECUTE"
	// PlanActionSkip excludes a stage from the execution plan.
	PlanActionSkip PlanAction = "SKIP"
)

// Summary contains the execution plan counts and current stage.
type Summary struct {
	TotalStages int
	Completed   int
	InProgress  string
}

// PhaseProgress contains one canonical phase and its display status.
type PhaseProgress struct {
	Phase  LifecyclePhase
	Status PhaseStatus
}

// StageProgress contains one stage row from the state document.
//
// CheckboxMarker and Suffix preserve the document's canonical values after
// surrounding syntax has been validated. CheckboxState and PlanAction are
// derived typed values.
type StageProgress struct {
	Slug           string
	CheckboxMarker string
	CheckboxState  CheckboxState
	Suffix         string
	PlanAction     PlanAction
}

// State is a validated, immutable-in-practice snapshot of aidlc-state.md.
// Its slices are private so accessors can return defensive copies.
type State struct {
	version        int
	scope          string
	projectType    string
	workflowStatus WorkflowStatus
	lifecyclePhase LifecyclePhase
	currentStage   string
	nextStage      string
	summary        Summary
	phaseProgress  []PhaseProgress
	stages         []StageProgress
}

// Version returns the State Version.
func (s State) Version() int { return s.version }

// Scope returns the selected workflow scope.
func (s State) Scope() string { return s.scope }

// ProjectType returns the project type recorded in the state.
func (s State) ProjectType() string { return s.projectType }

// WorkflowStatus returns the workflow status.
func (s State) WorkflowStatus() WorkflowStatus { return s.workflowStatus }

// LifecyclePhase returns the current lifecycle phase.
func (s State) LifecyclePhase() LifecyclePhase { return s.lifecyclePhase }

// CurrentStage returns the current stage slug.
func (s State) CurrentStage() string { return s.currentStage }

// NextStage returns the next stage slug or the literal none value.
func (s State) NextStage() string { return s.nextStage }

// Summary returns the execution plan summary.
func (s State) Summary() Summary { return s.summary }

// PhaseProgress returns the five canonical phase rows in document order.
func (s State) PhaseProgress() []PhaseProgress { return slices.Clone(s.phaseProgress) }

// Phases is an alias for PhaseProgress for callers that describe the rows as phases.
func (s State) Phases() []PhaseProgress { return s.PhaseProgress() }

// Stages returns stage rows in their document order.
func (s State) Stages() []StageProgress { return slices.Clone(s.stages) }

// RawSuffix returns the validated suffix exactly as retained by the parser.
func (s StageProgress) RawSuffix() string { return s.Suffix }
