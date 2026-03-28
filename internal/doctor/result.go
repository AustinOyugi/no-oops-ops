package doctor

type Status string

const (
	StatusOK   Status = "ok"
	StatusFail Status = "fail"
)

type Check struct {
	Name    string
	Status  Status
	Message string
}

type Result struct {
	Checks []Check
}

func (r *Result) Add(name string, status Status, message string) {
	r.Checks = append(r.Checks, Check{
		Name:    name,
		Status:  status,
		Message: message,
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
