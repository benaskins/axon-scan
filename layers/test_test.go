package layers_test

import (
	"context"
	"strings"
	"testing"

	"github.com/benaskins/axon-scan/layers"
)

func TestTestExecutionLayer_Name(t *testing.T) {
	layer := layers.NewTestExecutionLayer(&mockRunner{responses: map[string]mockResponse{}})
	if layer.Name() != "test-execution" {
		t.Errorf("expected name %q, got %q", "test-execution", layer.Name())
	}
}

func TestTestExecutionLayer_Pass(t *testing.T) {
	runner := &mockRunner{
		responses: map[string]mockResponse{
			"go test": {
				stdout:   []byte("ok  \tgithub.com/example/pkg\t0.123s"),
				stderr:   nil,
				exitCode: 0,
			},
		},
	}
	layer := layers.NewTestExecutionLayer(runner)
	result, err := layer.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Errorf("expected Pass=true for exit code 0")
	}
	if !strings.Contains(result.RawOutput, "ok") {
		t.Errorf("expected RawOutput to contain stdout, got %q", result.RawOutput)
	}
}

func TestTestExecutionLayer_Fail(t *testing.T) {
	failOutput := "--- FAIL: TestFoo (0.00s)\nFAIL\tgithub.com/example/pkg\t0.045s"
	runner := &mockRunner{
		responses: map[string]mockResponse{
			"go test": {
				stdout:   []byte(failOutput),
				stderr:   []byte("some stderr"),
				exitCode: 1,
			},
		},
	}
	layer := layers.NewTestExecutionLayer(runner)
	result, err := layer.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Errorf("expected Pass=false for exit code 1")
	}
	if !strings.Contains(result.RawOutput, "FAIL") {
		t.Errorf("expected RawOutput to contain failure output, got %q", result.RawOutput)
	}
	if !strings.Contains(result.RawOutput, "some stderr") {
		t.Errorf("expected RawOutput to contain stderr, got %q", result.RawOutput)
	}
}

func TestTestExecutionLayer_NoFindings(t *testing.T) {
	runner := &mockRunner{
		responses: map[string]mockResponse{
			"go test": {
				stdout:   []byte("--- FAIL: TestBar (0.01s)\nFAIL"),
				exitCode: 1,
			},
		},
	}
	layer := layers.NewTestExecutionLayer(runner)
	result, err := layer.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings (no parsing), got %d", len(result.Findings))
	}
}
