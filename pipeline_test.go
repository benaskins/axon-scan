package scan_test

import (
	"context"
	"sync"
	"testing"
	"time"

	scan "github.com/benaskins/axon-scan"
)

// mockLayer is a test double for scan.Layer.
type mockLayer struct {
	name    string
	pass    bool
	delay   time.Duration
	mu      sync.Mutex
	called  bool
	calledAt time.Time
}

func (m *mockLayer) Name() string { return m.name }

func (m *mockLayer) Run(_ context.Context, _ string) (*scan.LayerResult, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	m.mu.Lock()
	m.called = true
	m.calledAt = time.Now()
	m.mu.Unlock()
	return &scan.LayerResult{
		Name: m.name,
		Pass: m.pass,
	}, nil
}

func TestPipeline_SequentialPassFail(t *testing.T) {
	layerA := &mockLayer{name: "layer-a", pass: true}
	layerB := &mockLayer{name: "layer-b", pass: false}

	p := scan.NewPipeline([]scan.Layer{layerA, layerB})
	result, err := p.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Pass {
		t.Error("expected PipelineResult.Pass=false when any layer fails")
	}
	if len(result.LayerResults) != 2 {
		t.Errorf("expected 2 LayerResults, got %d", len(result.LayerResults))
	}
	if result.StartedAt.IsZero() {
		t.Error("StartedAt not set")
	}
	if result.FinishedAt.IsZero() {
		t.Error("FinishedAt not set")
	}
	if !result.FinishedAt.After(result.StartedAt) {
		t.Error("FinishedAt should be after StartedAt")
	}
}

func TestPipeline_AllPass(t *testing.T) {
	layerA := &mockLayer{name: "layer-a", pass: true}
	layerB := &mockLayer{name: "layer-b", pass: true}

	p := scan.NewPipeline([]scan.Layer{layerA, layerB})
	result, err := p.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Pass {
		t.Error("expected PipelineResult.Pass=true when all layers pass")
	}
}

func TestPipeline_WithLayersOverride(t *testing.T) {
	original := &mockLayer{name: "original", pass: true}
	override := &mockLayer{name: "override", pass: false}

	p := scan.NewPipeline([]scan.Layer{original}, scan.WithLayers([]scan.Layer{override}))
	result, err := p.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Pass {
		t.Error("expected Pass=false (override layer fails)")
	}
	if len(result.LayerResults) != 1 || result.LayerResults[0].Name != "override" {
		t.Error("expected only the override layer to run")
	}
	if original.called {
		t.Error("original layer should not have been called when WithLayers overrides it")
	}
}

func TestPipeline_ParallelRunsConcurrently(t *testing.T) {
	// Both layers have a delay; in parallel they should finish in roughly one
	// delay duration, not two.
	delay := 50 * time.Millisecond
	layerA := &mockLayer{name: "layer-a", pass: true, delay: delay}
	layerB := &mockLayer{name: "layer-b", pass: true, delay: delay}

	p := scan.NewPipeline([]scan.Layer{layerA, layerB}, scan.WithParallel(true))

	start := time.Now()
	result, err := p.Run(context.Background(), t.TempDir())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Error("expected Pass=true")
	}
	if len(result.LayerResults) != 2 {
		t.Errorf("expected 2 LayerResults, got %d", len(result.LayerResults))
	}

	// Parallel run should complete well under 2× delay (use 1.5× as threshold).
	threshold := time.Duration(float64(delay) * 1.5)
	if elapsed > threshold {
		t.Errorf("parallel run took %v, expected < %v (suggests sequential execution)", elapsed, threshold)
	}
}

func TestPipelineResult_Deduplicate(t *testing.T) {
	// Layer A: one duplicate finding (warning) and one unique finding.
	layerA := scan.LayerResult{
		Name: "layer-a",
		Pass: true,
		Findings: []scan.Finding{
			{Severity: scan.SeverityWarning, File: "main.go", Line: 10, Description: "warning from A"},
			{Severity: scan.SeverityInfo, File: "main.go", Line: 20, Description: "unique to A"},
		},
	}
	// Layer B: same file+line as the first finding in A (higher severity) and
	// a second finding that is unique.
	layerB := scan.LayerResult{
		Name: "layer-b",
		Pass: true,
		Findings: []scan.Finding{
			{Severity: scan.SeverityHigh, File: "main.go", Line: 10, Description: "high from B"},
			{Severity: scan.SeverityInfo, File: "util.go", Line: 5, Description: "unique to B"},
		},
	}

	result := &scan.PipelineResult{
		LayerResults: []scan.LayerResult{layerA, layerB},
		Pass:         true,
	}

	deduped := result.Deduplicate()

	// Total unique file+line pairs: (main.go:10), (main.go:20), (util.go:5) → 3.
	if len(deduped) != 3 {
		t.Fatalf("expected 3 deduplicated findings, got %d", len(deduped))
	}

	// Find the finding for main.go:10 and verify the highest severity is kept.
	var found *scan.Finding
	for i := range deduped {
		if deduped[i].File == "main.go" && deduped[i].Line == 10 {
			found = &deduped[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected finding for main.go:10 in deduplicated results")
	}
	if found.Severity != scan.SeverityHigh {
		t.Errorf("expected highest severity %q for main.go:10, got %q", scan.SeverityHigh, found.Severity)
	}
}

func TestPipeline_EmptyLayers(t *testing.T) {
	p := scan.NewPipeline(nil)
	result, err := p.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Error("empty pipeline should pass")
	}
	if len(result.LayerResults) != 0 {
		t.Errorf("expected 0 LayerResults, got %d", len(result.LayerResults))
	}
}
