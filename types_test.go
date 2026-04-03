package scan_test

import (
	"context"
	"testing"
	"time"

	scan "github.com/benaskins/axon-scan"
)

func TestSeverityConstants(t *testing.T) {
	severities := []scan.Severity{
		scan.SeverityInfo,
		scan.SeverityWarning,
		scan.SeverityHigh,
		scan.SeverityCritical,
	}
	if len(severities) != 4 {
		t.Fatalf("expected 4 severity constants, got %d", len(severities))
	}
}

func TestFindingStruct(t *testing.T) {
	f := scan.Finding{
		Severity:    scan.SeverityHigh,
		File:        "main.go",
		Line:        42,
		Description: "something bad",
	}
	if f.File != "main.go" {
		t.Errorf("unexpected File: %s", f.File)
	}
}

func TestLayerResultStruct(t *testing.T) {
	lr := scan.LayerResult{
		Name:      "static",
		Pass:      true,
		Findings:  []scan.Finding{},
		Duration:  100 * time.Millisecond,
		RawOutput: "ok",
	}
	if !lr.Pass {
		t.Error("expected Pass to be true")
	}
}

func TestPipelineResultStruct(t *testing.T) {
	pr := scan.PipelineResult{
		LayerResults: []scan.LayerResult{},
		Pass:         true,
		StartedAt:    time.Now(),
		FinishedAt:   time.Now(),
	}
	if !pr.Pass {
		t.Error("expected Pass to be true")
	}
}

// LayerImpl is a compile-time check that Layer interface is satisfied.
type LayerImpl struct{}

func (l *LayerImpl) Name() string { return "test" }
func (l *LayerImpl) Run(_ context.Context, _ string) (*scan.LayerResult, error) {
	return &scan.LayerResult{Name: "test", Pass: true}, nil
}

func TestLayerInterface(t *testing.T) {
	var _ scan.Layer = &LayerImpl{}
}
