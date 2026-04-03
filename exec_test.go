package scan

import (
	"context"
	"strings"
	"testing"
)

func TestDefaultExecRunner_Echo(t *testing.T) {
	runner := &DefaultExecRunner{}
	stdout, stderr, exitCode, err := runner.Run(context.Background(), t.TempDir(), "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}
	if !strings.Contains(string(stdout), "hello") {
		t.Fatalf("expected stdout to contain 'hello', got %q", stdout)
	}
	_ = stderr
}
