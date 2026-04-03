package layers_test

import (
	"context"
	"os/exec"
	"testing"

	scan "github.com/benaskins/axon-scan"
	"github.com/benaskins/axon-scan/layers"
)

// canned gosec JSON payloads

var gosecEmpty = []byte(`{"Issues":[],"Stats":{"files":3,"lines":100,"nosec":0,"found":0}}`)

var gosecLowOnly = []byte(`{
  "Issues": [
    {
      "severity": "LOW",
      "confidence": "HIGH",
      "rule_id": "G304",
      "details": "File path provided as taint input",
      "file": "main.go",
      "line": "42",
      "column": "3"
    }
  ],
  "Stats": {"files":1,"lines":50,"nosec":0,"found":1}
}`)

var gosecHighPresent = []byte(`{
  "Issues": [
    {
      "severity": "LOW",
      "confidence": "HIGH",
      "rule_id": "G304",
      "details": "File path provided as taint input",
      "file": "main.go",
      "line": "42",
      "column": "3"
    },
    {
      "severity": "HIGH",
      "confidence": "HIGH",
      "rule_id": "G101",
      "details": "Potential hardcoded credentials",
      "file": "auth.go",
      "line": "15",
      "column": "10"
    }
  ],
  "Stats": {"files":2,"lines":100,"nosec":0,"found":2}
}`)

var gosecCriticalPresent = []byte(`{
  "Issues": [
    {
      "severity": "CRITICAL",
      "confidence": "HIGH",
      "rule_id": "G501",
      "details": "Import blocklist: crypto/md5",
      "file": "crypto.go",
      "line": "8",
      "column": "2"
    }
  ],
  "Stats": {"files":1,"lines":20,"nosec":0,"found":1}
}`)

var gosecMediumOnly = []byte(`{
  "Issues": [
    {
      "severity": "MEDIUM",
      "confidence": "MEDIUM",
      "rule_id": "G401",
      "details": "Use of weak cryptographic primitive",
      "file": "hash.go",
      "line": "33",
      "column": "5"
    }
  ],
  "Stats": {"files":1,"lines":40,"nosec":0,"found":1}
}`)

func TestSecurityScanLayer_Name(t *testing.T) {
	layer := layers.NewSecurityScanLayer(&mockRunner{responses: map[string]mockResponse{}})
	if layer.Name() != "security-scan" {
		t.Errorf("expected name %q, got %q", "security-scan", layer.Name())
	}
}

func TestSecurityScanLayer_EmptyFindings(t *testing.T) {
	runner := &mockRunner{
		responses: map[string]mockResponse{
			"gosec -fmt": {stdout: gosecEmpty, exitCode: 0},
		},
	}
	layer := layers.NewSecurityScanLayer(runner)
	result, err := layer.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Errorf("expected Pass=true for empty findings")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result.Findings))
	}
}

func TestSecurityScanLayer_LowOnly(t *testing.T) {
	runner := &mockRunner{
		responses: map[string]mockResponse{
			"gosec -fmt": {stdout: gosecLowOnly, exitCode: 1},
		},
	}
	layer := layers.NewSecurityScanLayer(runner)
	result, err := layer.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Errorf("expected Pass=true when only LOW findings present")
	}
	if len(result.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(result.Findings))
	}
	if result.Findings[0].Severity != scan.SeverityInfo {
		t.Errorf("expected SeverityInfo for LOW gosec finding, got %q", result.Findings[0].Severity)
	}
}

func TestSecurityScanLayer_MediumOnly(t *testing.T) {
	runner := &mockRunner{
		responses: map[string]mockResponse{
			"gosec -fmt": {stdout: gosecMediumOnly, exitCode: 1},
		},
	}
	layer := layers.NewSecurityScanLayer(runner)
	result, err := layer.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Errorf("expected Pass=true when only MEDIUM findings present")
	}
	if len(result.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(result.Findings))
	}
	if result.Findings[0].Severity != scan.SeverityWarning {
		t.Errorf("expected SeverityWarning for MEDIUM gosec finding, got %q", result.Findings[0].Severity)
	}
}

func TestSecurityScanLayer_HighPresent(t *testing.T) {
	runner := &mockRunner{
		responses: map[string]mockResponse{
			"gosec -fmt": {stdout: gosecHighPresent, exitCode: 1},
		},
	}
	layer := layers.NewSecurityScanLayer(runner)
	result, err := layer.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Errorf("expected Pass=false when HIGH finding present")
	}
	if len(result.Findings) != 2 {
		t.Errorf("expected 2 findings, got %d", len(result.Findings))
	}
}

func TestSecurityScanLayer_CriticalPresent(t *testing.T) {
	runner := &mockRunner{
		responses: map[string]mockResponse{
			"gosec -fmt": {stdout: gosecCriticalPresent, exitCode: 1},
		},
	}
	layer := layers.NewSecurityScanLayer(runner)
	result, err := layer.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Errorf("expected Pass=false when CRITICAL finding present")
	}
	if len(result.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(result.Findings))
	}
	if result.Findings[0].Severity != scan.SeverityCritical {
		t.Errorf("expected SeverityCritical, got %q", result.Findings[0].Severity)
	}
}

func TestSecurityScanLayer_GosecMissing(t *testing.T) {
	runner := &mockRunner{
		responses: map[string]mockResponse{
			"gosec -fmt": {
				exitCode: -1,
				err:      exec.ErrNotFound,
			},
		},
	}
	layer := layers.NewSecurityScanLayer(runner)
	result, err := layer.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error when gosec missing: %v", err)
	}
	if !result.Pass {
		t.Errorf("expected Pass=true when gosec is not installed")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings when gosec is skipped, got %d", len(result.Findings))
	}
	if result.RawOutput == "" {
		t.Errorf("expected non-empty RawOutput for missing gosec message")
	}
}

func TestSecurityScanLayer_FindingFields(t *testing.T) {
	runner := &mockRunner{
		responses: map[string]mockResponse{
			"gosec -fmt": {stdout: gosecHighPresent, exitCode: 1},
		},
	}
	layer := layers.NewSecurityScanLayer(runner)
	result, err := layer.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// find the HIGH finding
	var high *scan.Finding
	for i := range result.Findings {
		if result.Findings[i].Severity == scan.SeverityHigh {
			high = &result.Findings[i]
			break
		}
	}
	if high == nil {
		t.Fatal("expected a HIGH finding")
	}
	if high.File != "auth.go" {
		t.Errorf("expected File=auth.go, got %q", high.File)
	}
	if high.Line != 15 {
		t.Errorf("expected Line=15, got %d", high.Line)
	}
	if high.Description == "" {
		t.Errorf("expected non-empty Description")
	}
}
