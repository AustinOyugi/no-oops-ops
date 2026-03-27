package install

type Step string

const (
	StepVerifyDocker           Step = "verify_docker"
	StepEnsureSwarmInitialized Step = "ensure_swarm_initialized"
	StepEnsureSharedNetwork    Step = "ensure_shared_network"
	StepEnsureRegistry         Step = "ensure_registry"

	StepPrepareStateDir      Step = "prepare_state_dir"
	StepInitializeLocalState Step = "initialize_local_state"
	StepWriteInstallMetadata Step = "write_install_metadata"
)

type StepStatus string

const (
	StatusRunning   StepStatus = "running"
	StatusCompleted StepStatus = "completed"
	StatusFailed    StepStatus = "failed"
)

type StepResult struct {
	Name   Step
	Status StepStatus
	Error  string
}

type Result struct {
	Steps []StepResult
}

func (r *Result) CompletedCount() int {
	count := 0

	for _, step := range r.Steps {
		if step.Status == StatusCompleted {
			count++
		}
	}

	return count
}

func (r *Result) Failed() bool {
	for _, step := range r.Steps {
		if step.Status == StatusFailed {
			return true
		}
	}

	return false
}

func (r *Result) SetStep(step Step, status StepStatus, errMsg string) {
	index, ok := r.stepIndex(step)
	if ok {
		r.Steps[index].Status = status
		r.Steps[index].Error = errMsg
		return
	}

	r.Steps = append(r.Steps, StepResult{
		Name:   step,
		Status: status,
		Error:  errMsg,
	})
}

func (r *Result) LastStep() (StepResult, bool) {
	if len(r.Steps) == 0 {
		return StepResult{}, false
	}

	return r.Steps[len(r.Steps)-1], true
}

func (r *Result) Step(step Step) (StepResult, bool) {
	index, ok := r.stepIndex(step)
	if ok {
		return r.Steps[index], true
	}
	return StepResult{}, false
}

func (r *Result) stepIndex(step Step) (int, bool) {
	for index, current := range r.Steps {
		if current.Name == step {
			return index, true
		}
	}
	return 0, false
}
