package exec

import (
	"fmt"
	"os/exec"
	"strings"
)

// Runner abstracts command execution for testability.
type Runner interface {
	Run(command string) ([]byte, error)
}

// DefaultRunner executes commands using os/exec.
type DefaultRunner struct{}

func (r *DefaultRunner) Run(command string) ([]byte, error) {
	args := strings.Fields(command)
	if len(args) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("command %q failed: %s: %s", command, err, string(ee.Stderr))
		}
		return nil, fmt.Errorf("command %q failed: %s", command, err)
	}
	return out, nil
}
