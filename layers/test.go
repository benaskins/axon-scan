package layers

import (
	"context"
	"strings"
	"time"

	scan "github.com/benaskins/axon-scan"
)

type testExecutionLayer struct {
	runner scan.ExecRunner
}

// NewTestExecutionLayer constructs a Layer that runs `go test -race ./...` in projectDir.
// Test failures are surfaced via RawOutput; no individual findings are parsed.
func NewTestExecutionLayer(runner scan.ExecRunner) scan.Layer {
	return &testExecutionLayer{runner: runner}
}

func (l *testExecutionLayer) Name() string {
	return "test-execution"
}

func (l *testExecutionLayer) Run(ctx context.Context, projectDir string) (*scan.LayerResult, error) {
	start := time.Now()

	stdout, stderr, exitCode, err := l.runner.Run(ctx, projectDir, "go", "test", "-race", "./...")
	if err != nil {
		return nil, err
	}

	var raw strings.Builder
	if len(stdout) > 0 {
		raw.Write(stdout)
	}
	if len(stderr) > 0 {
		if raw.Len() > 0 {
			raw.WriteByte('\n')
		}
		raw.Write(stderr)
	}

	return &scan.LayerResult{
		Name:      l.Name(),
		Pass:      exitCode == 0,
		Duration:  time.Since(start),
		RawOutput: raw.String(),
	}, nil
}
