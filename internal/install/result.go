package install

type Step string

const (
	StepVerifyDocker    Step = "verify_docker"
	StepPrepareStateDir Step = "prepare_state_dir"
)

type StepStatus string

const (
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

func (r Result) CompletedCount() int {
	count := 0

	for _, step := range r.Steps {
		if step.Status == StatusCompleted {
			count++
		}
	}

	return count
}

func (r Result) Failed() bool {
	for _, step := range r.Steps {
		if step.Status == StatusFailed {
			return true
		}
	}

	return false
}
