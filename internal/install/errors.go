package install

import "fmt"

type PrerequisiteError struct {
	Check Step
	Err   error
}

func (e PrerequisiteError) Error() string {
	return fmt.Sprintf("%s failed: %v", e.Check, e.Err)
}

func (e PrerequisiteError) Unwrap() error {
	return e.Err
}
