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
