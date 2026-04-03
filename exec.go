package scan

import (
	"context"
	"os/exec"
)

// ExecRunner is the interface through which layers invoke external binaries.
// Using this abstraction makes all exec calls injectable in tests.
type ExecRunner interface {
	Run(ctx context.Context, dir string, name string, args ...string) (stdout, stderr []byte, exitCode int, err error)
}

// DefaultExecRunner executes real commands via os/exec.
type DefaultExecRunner struct{}

func (r *DefaultExecRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, []byte, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	stdout, err := cmd.Output()
	var stderr []byte
	if exitErr, ok := err.(*exec.ExitError); ok {
		stderr = exitErr.Stderr
		return stdout, stderr, exitErr.ExitCode(), nil
	}
	if err != nil {
		return nil, nil, -1, err
	}
	return stdout, nil, 0, nil
}
