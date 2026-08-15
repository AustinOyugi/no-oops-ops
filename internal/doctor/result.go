package doctor

import "context"

type Status string

const (
	StatusOK   Status = "ok"
	StatusFail Status = "fail"
	StatusSkip Status = "skip"
)

type Check struct {
	Name        string
	Status      Status
	Message     string
	Remediation string
}

type Result struct {
	Checks []Check
}

type checkDefinition struct {
	name        string
	requires    []string
	skipMessage string
	remediation string
	run         func(context.Context) Check
}

func (r *Result) Add(name string, status Status, message string, remediation string) {
	r.Checks = append(r.Checks, Check{
		Name:        name,
		Status:      status,
		Message:     message,
		Remediation: remediation,
	})
}

func (r *Result) Failed() bool {
	for _, check := range r.Checks {
		if check.Status == StatusFail {
			return true
		}
	}

	return false
}

func (r *Result) Count(status Status) int {
	count := 0
	for _, check := range r.Checks {
		if check.Status == status {
			count++
		}
	}

	return count
}

func (r *Result) FirstRemediation() string {
	for _, check := range r.Checks {
		if check.Status == StatusFail && check.Remediation != "" {
			return check.Remediation
		}
	}

	return ""
}
