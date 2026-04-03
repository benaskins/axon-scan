package attestation_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	scan "github.com/benaskins/axon-scan"
	"github.com/benaskins/axon-scan/attestation"
)

func cannedResult() *scan.PipelineResult {
	return &scan.PipelineResult{
		Pass:       false,
		StartedAt:  time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		FinishedAt: time.Date(2026, 4, 3, 12, 0, 5, 0, time.UTC),
		LayerResults: []scan.LayerResult{
			{
				Name:      "static",
				Pass:      true,
				Duration:  500 * time.Millisecond,
				Findings:  []scan.Finding{},
				RawOutput: "ok",
			},
			{
				Name:     "security",
				Pass:     false,
				Duration: 1200 * time.Millisecond,
				Findings: []scan.Finding{
					{Severity: scan.SeverityHigh, File: "main.go", Line: 42, Description: "unsafe pointer"},
					{Severity: scan.SeverityInfo, File: "util.go", Line: 7, Description: "shadowed var"},
				},
				RawOutput: "issues found",
			},
		},
	}
}

func TestWriteAttestation_FilesCreated(t *testing.T) {
	dir := t.TempDir()
	if err := attestation.WriteAttestation(cannedResult(), dir, nil); err != nil {
		t.Fatalf("WriteAttestation: %v", err)
	}
	for _, name := range []string{"attestation.json", "ATTESTATION.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to exist: %v", name, err)
		}
	}
}

func TestWriteAttestation_JSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := attestation.WriteAttestation(cannedResult(), dir, nil); err != nil {
		t.Fatalf("WriteAttestation: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "attestation.json"))
	if err != nil {
		t.Fatalf("read attestation.json: %v", err)
	}

	// Must decode into plain map without importing axon-scan types.
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("decode into map[string]any: %v", err)
	}
	if _, ok := m["Pass"]; !ok {
		t.Error("expected Pass key in JSON")
	}
	if _, ok := m["LayerResults"]; !ok {
		t.Error("expected LayerResults key in JSON")
	}
}

func TestWriteAttestation_SigWrittenWhenSignerProvided(t *testing.T) {
	dir := t.TempDir()

	stubSigner := func(data []byte) ([]byte, error) {
		return append([]byte("SIG:"), data...), nil
	}

	if err := attestation.WriteAttestation(cannedResult(), dir, stubSigner); err != nil {
		t.Fatalf("WriteAttestation: %v", err)
	}

	sigPath := filepath.Join(dir, "attestation.json.sig")
	if _, err := os.Stat(sigPath); err != nil {
		t.Fatalf("expected attestation.json.sig to exist: %v", err)
	}

	sigData, err := os.ReadFile(sigPath)
	if err != nil {
		t.Fatalf("read sig: %v", err)
	}
	if !strings.HasPrefix(string(sigData), "SIG:") {
		t.Errorf("expected sig to start with stub prefix, got: %q", string(sigData[:10]))
	}
}

func TestWriteAttestation_NoSigWithoutSigner(t *testing.T) {
	dir := t.TempDir()
	if err := attestation.WriteAttestation(cannedResult(), dir, nil); err != nil {
		t.Fatalf("WriteAttestation: %v", err)
	}
	sigPath := filepath.Join(dir, "attestation.json.sig")
	if _, err := os.Stat(sigPath); err == nil {
		t.Error("expected attestation.json.sig NOT to exist when signer is nil")
	}
}

func TestWriteAttestation_MarkdownContent(t *testing.T) {
	dir := t.TempDir()
	if err := attestation.WriteAttestation(cannedResult(), dir, nil); err != nil {
		t.Fatalf("WriteAttestation: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "ATTESTATION.md"))
	if err != nil {
		t.Fatalf("read ATTESTATION.md: %v", err)
	}
	md := string(data)

	for _, want := range []string{"FAIL", "static", "security", "main.go", "unsafe pointer"} {
		if !strings.Contains(md, want) {
			t.Errorf("ATTESTATION.md missing %q", want)
		}
	}
}
