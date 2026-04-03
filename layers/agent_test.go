package layers_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	loop "github.com/benaskins/axon-loop"
	tool "github.com/benaskins/axon-tool"

	scan "github.com/benaskins/axon-scan"
	"github.com/benaskins/axon-scan/layers"
)

// stubLoopRunner is a test double for layers.LoopRunner.
// For each cannedFinding, it directly invokes the report_finding tool before returning.
type stubLoopRunner struct {
	cannedFindings []map[string]any
}

func (s *stubLoopRunner) Run(ctx context.Context, cfg loop.RunConfig) (*loop.Result, error) {
	def, ok := cfg.Tools["report_finding"]
	if ok {
		tc := &tool.ToolContext{Ctx: ctx}
		for _, f := range s.cannedFindings {
			def.Execute(tc, f)
		}
	}
	return &loop.Result{Content: "review complete"}, nil
}

// makeProjectDir creates a minimal project dir for the agent to read.
func makeProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Test project\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestAgentReviewLayerName(t *testing.T) {
	layer := layers.NewAgentReviewLayer(&stubLoopRunner{})
	if got := layer.Name(); got != "agent-review" {
		t.Errorf("Name() = %q, want %q", got, "agent-review")
	}
}

func TestAgentReviewLayerNoFindings(t *testing.T) {
	stub := &stubLoopRunner{}
	layer := layers.NewAgentReviewLayer(stub)

	result, err := layer.Run(context.Background(), makeProjectDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Pass {
		t.Error("Pass = false, want true when no findings")
	}
	if len(result.Findings) != 0 {
		t.Errorf("Findings = %v, want empty", result.Findings)
	}
	if result.RawOutput != "review complete" {
		t.Errorf("RawOutput = %q, want %q", result.RawOutput, "review complete")
	}
}

func TestAgentReviewLayerNonCriticalFindingsPass(t *testing.T) {
	stub := &stubLoopRunner{
		cannedFindings: []map[string]any{
			{"severity": "warning", "file": "main.go", "line": float64(10), "description": "unused variable"},
			{"severity": "high", "file": "main.go", "line": float64(20), "description": "missing error check"},
		},
	}
	layer := layers.NewAgentReviewLayer(stub)

	result, err := layer.Run(context.Background(), makeProjectDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Pass {
		t.Error("Pass = false, want true when no CRITICAL findings")
	}
	if len(result.Findings) != 2 {
		t.Errorf("Findings count = %d, want 2", len(result.Findings))
	}
	if result.Findings[0].Severity != scan.SeverityWarning {
		t.Errorf("Findings[0].Severity = %q, want %q", result.Findings[0].Severity, scan.SeverityWarning)
	}
	if result.Findings[1].File != "main.go" {
		t.Errorf("Findings[1].File = %q, want %q", result.Findings[1].File, "main.go")
	}
}

func TestAgentReviewLayerCriticalFindingFails(t *testing.T) {
	stub := &stubLoopRunner{
		cannedFindings: []map[string]any{
			{"severity": "warning", "file": "util.go", "line": float64(5), "description": "minor issue"},
			{"severity": "critical", "file": "auth.go", "line": float64(42), "description": "sql injection"},
		},
	}
	layer := layers.NewAgentReviewLayer(stub)

	result, err := layer.Run(context.Background(), makeProjectDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if result.Pass {
		t.Error("Pass = true, want false when CRITICAL finding present")
	}
	if len(result.Findings) != 2 {
		t.Errorf("Findings count = %d, want 2", len(result.Findings))
	}
}

func TestAgentReviewLayerFindingLineNumbers(t *testing.T) {
	stub := &stubLoopRunner{
		cannedFindings: []map[string]any{
			{"severity": "info", "file": "foo.go", "line": float64(99), "description": "note"},
		},
	}
	layer := layers.NewAgentReviewLayer(stub)

	result, err := layer.Run(context.Background(), makeProjectDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if result.Findings[0].Line != 99 {
		t.Errorf("Line = %d, want 99", result.Findings[0].Line)
	}
}

func TestAgentReviewLayerSkipsAbsentAgentsMd(t *testing.T) {
	// AGENTS.md is absent — layer must succeed without it
	stub := &stubLoopRunner{}
	layer := layers.NewAgentReviewLayer(stub)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No AGENTS.md, no CLAUDE.md

	_, err := layer.Run(context.Background(), dir)
	if err != nil {
		t.Fatalf("Run failed when AGENTS.md absent: %v", err)
	}
}
