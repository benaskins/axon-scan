package layers_test

import (
	"context"
	"os/exec"
	"testing"

	scan "github.com/benaskins/axon-scan"
	"github.com/benaskins/axon-scan/layers"
)

// mockRunner is a test double for scan.ExecRunner.
// Responses are keyed as "<name> <args[0]>" (e.g. "go build", "staticcheck ./...").
type mockRunner struct {
	responses map[string]mockResponse
}

type mockResponse struct {
	stdout   []byte
	stderr   []byte
	exitCode int
	err      error
}

func (m *mockRunner) Run(_ context.Context, _ string, name string, args ...string) ([]byte, []byte, int, error) {
	key := name
	if len(args) > 0 {
		key = name + " " + args[0]
	}
	if r, ok := m.responses[key]; ok {
		return r.stdout, r.stderr, r.exitCode, r.err
	}
	return nil, nil, 0, nil
}

func TestStaticAnalysisLayer_Name(t *testing.T) {
	layer := layers.NewStaticAnalysisLayer(&mockRunner{responses: map[string]mockResponse{}})
	if layer.Name() != "static-analysis" {
		t.Errorf("expected name %q, got %q", "static-analysis", layer.Name())
	}
}

func TestStaticAnalysisLayer_AllPass(t *testing.T) {
	runner := &mockRunner{
		responses: map[string]mockResponse{
			"go build":        {exitCode: 0},
			"go vet":          {exitCode: 0},
			"staticcheck ./...": {exitCode: 0},
		},
	}
	layer := layers.NewStaticAnalysisLayer(runner)
	result, err := layer.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Errorf("expected Pass=true, got false")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result.Findings))
	}
}

func TestStaticAnalysisLayer_BuildFail(t *testing.T) {
	runner := &mockRunner{
		responses: map[string]mockResponse{
			"go build": {
				stderr:   []byte("./main.go:5:1: undefined: Foo\n"),
				exitCode: 1,
			},
			"go vet":          {exitCode: 0},
			"staticcheck ./...": {exitCode: 0},
		},
	}
	layer := layers.NewStaticAnalysisLayer(runner)
	result, err := layer.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Errorf("expected Pass=false when go build fails")
	}
}

func TestStaticAnalysisLayer_VetFailWithFindings(t *testing.T) {
	vetOutput := []byte("./foo.go:12:3: unreachable code\n./bar.go:8:1: redundant type in composite literal\n")
	runner := &mockRunner{
		responses: map[string]mockResponse{
			"go build": {exitCode: 0},
			"go vet": {
				stderr:   vetOutput,
				exitCode: 1,
			},
			"staticcheck ./...": {exitCode: 0},
		},
	}
	layer := layers.NewStaticAnalysisLayer(runner)
	result, err := layer.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Errorf("expected Pass=false when go vet fails")
	}
	if len(result.Findings) != 2 {
		t.Errorf("expected 2 findings, got %d", len(result.Findings))
	}
	for _, f := range result.Findings {
		if f.Severity != scan.SeverityWarning {
			t.Errorf("expected SeverityWarning for vet finding, got %q", f.Severity)
		}
		if f.File == "" {
			t.Errorf("expected non-empty File in finding")
		}
		if f.Line == 0 {
			t.Errorf("expected non-zero Line in finding")
		}
	}
}

func TestStaticAnalysisLayer_StaticcheckFailWithFindings(t *testing.T) {
	scOutput := []byte("foo.go:7:2: SA1006: unnecessary use of fmt.Sprintf\nbar.go:20:5: SA4006: this value is never used\n")
	runner := &mockRunner{
		responses: map[string]mockResponse{
			"go build": {exitCode: 0},
			"go vet":   {exitCode: 0},
			"staticcheck ./...": {
				stderr:   scOutput,
				exitCode: 1,
			},
		},
	}
	layer := layers.NewStaticAnalysisLayer(runner)
	result, err := layer.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Errorf("expected Pass=false when staticcheck fails")
	}
	if len(result.Findings) != 2 {
		t.Errorf("expected 2 findings, got %d", len(result.Findings))
	}
}

func TestStaticAnalysisLayer_StaticcheckMissing(t *testing.T) {
	runner := &mockRunner{
		responses: map[string]mockResponse{
			"go build": {exitCode: 0},
			"go vet":   {exitCode: 0},
			"staticcheck ./...": {
				exitCode: -1,
				err:      exec.ErrNotFound,
			},
		},
	}
	layer := layers.NewStaticAnalysisLayer(runner)
	result, err := layer.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error when staticcheck missing: %v", err)
	}
	if !result.Pass {
		t.Errorf("expected Pass=true when staticcheck is missing (skipped gracefully)")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings when staticcheck is skipped, got %d", len(result.Findings))
	}
}

func TestStaticAnalysisLayer_RawOutputPopulated(t *testing.T) {
	runner := &mockRunner{
		responses: map[string]mockResponse{
			"go build":        {stdout: []byte("build output"), exitCode: 0},
			"go vet":          {stderr: []byte("vet output"), exitCode: 0},
			"staticcheck ./...": {stdout: []byte("sc output"), exitCode: 0},
		},
	}
	layer := layers.NewStaticAnalysisLayer(runner)
	result, err := layer.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RawOutput == "" {
		t.Errorf("expected non-empty RawOutput")
	}
}
