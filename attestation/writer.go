package attestation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/template"
	"time"

	scan "github.com/benaskins/axon-scan"
)

var mdTmpl = template.Must(template.New("attestation").Parse(`# Attestation Report

**Overall:** {{ if .Pass }}PASS{{ else }}FAIL{{ end }}
**Started:** {{ .StartedAt }}
**Finished:** {{ .FinishedAt }}

## Layers

| Layer | Pass | Duration | Findings |
|-------|------|----------|----------|
{{ range .Layers -}}
| {{ .Name }} | {{ if .Pass }}PASS{{ else }}FAIL{{ end }} | {{ .Duration }} | {{ .FindingCount }} |
{{ end }}
## Findings
{{ if .BySeverity }}
{{ range .BySeverity -}}
### {{ .Severity }}

{{ range .Findings -}}
- **{{ .File }}:{{ .Line }}** — {{ .Description }}
{{ end }}
{{ end -}}
{{ else }}No findings.
{{ end }}`))

type layerRow struct {
	Name         string
	Pass         bool
	Duration     time.Duration
	FindingCount int
}

type severityGroup struct {
	Severity string
	Findings []scan.Finding
}

type templateData struct {
	Pass       bool
	StartedAt  string
	FinishedAt string
	Layers     []layerRow
	BySeverity []severityGroup
}

var severityOrder = []scan.Severity{
	scan.SeverityCritical,
	scan.SeverityHigh,
	scan.SeverityWarning,
	scan.SeverityInfo,
}

// WriteAttestation writes attestation.json and ATTESTATION.md into dir.
// If signer is non-nil, it is called with the JSON bytes and the result is
// written to attestation.json.sig.
func WriteAttestation(result *scan.PipelineResult, dir string, signer func([]byte) ([]byte, error)) error {
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal attestation: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "attestation.json"), jsonBytes, 0o644); err != nil {
		return fmt.Errorf("write attestation.json: %w", err)
	}

	if signer != nil {
		sig, err := signer(jsonBytes)
		if err != nil {
			return fmt.Errorf("sign attestation: %w", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "attestation.json.sig"), sig, 0o644); err != nil {
			return fmt.Errorf("write attestation.json.sig: %w", err)
		}
	}

	md, err := renderMarkdown(result)
	if err != nil {
		return fmt.Errorf("render ATTESTATION.md: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ATTESTATION.md"), md, 0o644); err != nil {
		return fmt.Errorf("write ATTESTATION.md: %w", err)
	}

	return nil
}

func renderMarkdown(result *scan.PipelineResult) ([]byte, error) {
	layers := make([]layerRow, len(result.LayerResults))
	for i, lr := range result.LayerResults {
		layers[i] = layerRow{
			Name:         lr.Name,
			Pass:         lr.Pass,
			Duration:     lr.Duration,
			FindingCount: len(lr.Findings),
		}
	}

	byGroup := make(map[scan.Severity][]scan.Finding)
	for _, lr := range result.LayerResults {
		for _, f := range lr.Findings {
			byGroup[f.Severity] = append(byGroup[f.Severity], f)
		}
	}

	var groups []severityGroup
	for _, sev := range severityOrder {
		if findings, ok := byGroup[sev]; ok {
			sort.Slice(findings, func(i, j int) bool {
				if findings[i].File != findings[j].File {
					return findings[i].File < findings[j].File
				}
				return findings[i].Line < findings[j].Line
			})
			groups = append(groups, severityGroup{
				Severity: string(sev),
				Findings: findings,
			})
		}
	}

	data := templateData{
		Pass:       result.Pass,
		StartedAt:  result.StartedAt.UTC().Format(time.RFC3339),
		FinishedAt: result.FinishedAt.UTC().Format(time.RFC3339),
		Layers:     layers,
		BySeverity: groups,
	}

	var buf bytes.Buffer
	if err := mdTmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
