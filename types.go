package scan

import (
	"context"
	"time"
)

// Severity represents the criticality of a finding.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Finding is a single issue reported by a scan layer.
type Finding struct {
	Severity    Severity
	File        string
	Line        int
	Description string
}

// LayerResult holds the output of a single scan layer.
type LayerResult struct {
	Name      string
	Pass      bool
	Findings  []Finding
	Duration  time.Duration
	RawOutput string
}

// PipelineResult aggregates results from all layers in a pipeline run.
type PipelineResult struct {
	LayerResults []LayerResult
	Pass         bool
	StartedAt    time.Time
	FinishedAt   time.Time
}

// Layer is the interface implemented by every scan layer.
type Layer interface {
	Name() string
	Run(ctx context.Context, projectDir string) (*LayerResult, error)
}
