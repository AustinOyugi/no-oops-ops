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
