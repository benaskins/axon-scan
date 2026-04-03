package layers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	scan "github.com/benaskins/axon-scan"
)

type securityScanLayer struct {
	runner scan.ExecRunner
}

// NewSecurityScanLayer constructs a Layer that runs gosec -fmt json ./... against
// projectDir. If gosec is not found in PATH, the layer passes with a skip message.
func NewSecurityScanLayer(runner scan.ExecRunner) scan.Layer {
	return &securityScanLayer{runner: runner}
}

func (l *securityScanLayer) Name() string {
	return "security-scan"
}

// gosecOutput is the top-level structure of gosec -fmt json output.
type gosecOutput struct {
	Issues []gosecIssue `json:"Issues"`
}

type gosecIssue struct {
	Severity   string `json:"severity"`
	RuleID     string `json:"rule_id"`
	Details    string `json:"details"`
	File       string `json:"file"`
	Line       string `json:"line"`
}

func (l *securityScanLayer) Run(ctx context.Context, projectDir string) (*scan.LayerResult, error) {
	start := time.Now()

	stdout, _, _, err := l.runner.Run(ctx, projectDir, "gosec", "-fmt", "json", "./...")
	if errors.Is(err, exec.ErrNotFound) {
		return &scan.LayerResult{
			Name:      l.Name(),
			Pass:      true,
			Duration:  time.Since(start),
			RawOutput: "gosec not installed, skipping",
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gosec: %w", err)
	}

	var out gosecOutput
	if err := json.Unmarshal(stdout, &out); err != nil {
		return nil, fmt.Errorf("gosec: parse output: %w", err)
	}

	pass := true
	findings := make([]scan.Finding, 0, len(out.Issues))
	for _, issue := range out.Issues {
		severity := mapSeverity(issue.Severity)
		if severity == scan.SeverityHigh || severity == scan.SeverityCritical {
			pass = false
		}
		lineNum, _ := strconv.Atoi(issue.Line)
		findings = append(findings, scan.Finding{
			Severity:    severity,
			File:        issue.File,
			Line:        lineNum,
			Description: issue.Details,
		})
	}

	return &scan.LayerResult{
		Name:      l.Name(),
		Pass:      pass,
		Findings:  findings,
		Duration:  time.Since(start),
		RawOutput: string(stdout),
	}, nil
}

// mapSeverity converts a gosec severity string to a scan.Severity.
func mapSeverity(s string) scan.Severity {
	switch strings.ToUpper(s) {
	case "CRITICAL":
		return scan.SeverityCritical
	case "HIGH":
		return scan.SeverityHigh
	case "MEDIUM":
		return scan.SeverityWarning
	default: // LOW, INFO, or unknown
		return scan.SeverityInfo
	}
}
