package scan

import (
	"context"
	"sync"
	"time"
)

// Pipeline runs a sequence (or concurrent set) of Layers and aggregates results.
type Pipeline struct {
	layers   []Layer
	parallel bool
}

// PipelineOption configures a Pipeline.
type PipelineOption func(*Pipeline)

// WithParallel enables or disables concurrent layer execution.
func WithParallel(parallel bool) PipelineOption {
	return func(p *Pipeline) {
		p.parallel = parallel
	}
}

// WithLayers overrides the layer set at run time, replacing whatever was passed
// to NewPipeline.
func WithLayers(layers []Layer) PipelineOption {
	return func(p *Pipeline) {
		p.layers = layers
	}
}

// NewPipeline constructs a Pipeline with the given layers and options.
func NewPipeline(layers []Layer, opts ...PipelineOption) *Pipeline {
	p := &Pipeline{layers: layers}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Run executes all layers and returns the aggregated PipelineResult.
// Layers run sequentially by default; use WithParallel(true) for concurrent
// execution.
func (p *Pipeline) Run(ctx context.Context, projectDir string) (*PipelineResult, error) {
	result := &PipelineResult{
		StartedAt: time.Now(),
	}

	if p.parallel {
		result.LayerResults = p.runParallel(ctx, projectDir)
	} else {
		result.LayerResults = p.runSequential(ctx, projectDir)
	}

	result.FinishedAt = time.Now()

	pass := true
	for _, lr := range result.LayerResults {
		if !lr.Pass {
			pass = false
			break
		}
	}
	result.Pass = pass

	return result, nil
}

func (p *Pipeline) runSequential(ctx context.Context, projectDir string) []LayerResult {
	results := make([]LayerResult, 0, len(p.layers))
	for _, layer := range p.layers {
		lr, err := layer.Run(ctx, projectDir)
		if err != nil {
			results = append(results, LayerResult{
				Name:      layer.Name(),
				Pass:      false,
				RawOutput: err.Error(),
			})
			continue
		}
		results = append(results, *lr)
	}
	return results
}

func (p *Pipeline) runParallel(ctx context.Context, projectDir string) []LayerResult {
	results := make([]LayerResult, len(p.layers))
	var wg sync.WaitGroup
	for i, layer := range p.layers {
		wg.Add(1)
		go func(idx int, l Layer) {
			defer wg.Done()
			lr, err := l.Run(ctx, projectDir)
			if err != nil {
				results[idx] = LayerResult{
					Name:      l.Name(),
					Pass:      false,
					RawOutput: err.Error(),
				}
				return
			}
			results[idx] = *lr
		}(i, layer)
	}
	wg.Wait()
	return results
}
