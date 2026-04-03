package layers

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	scan "github.com/benaskins/axon-scan"
)

type staticAnalysisLayer struct {
	runner scan.ExecRunner
}

// NewStaticAnalysisLayer constructs a Layer that runs go build, go vet, and
// staticcheck (skipped gracefully if not installed) against projectDir.
func NewStaticAnalysisLayer(runner scan.ExecRunner) scan.Layer {
	return &staticAnalysisLayer{runner: runner}
}

func (l *staticAnalysisLayer) Name() string {
	return "static-analysis"
}

func (l *staticAnalysisLayer) Run(ctx context.Context, projectDir string) (*scan.LayerResult, error) {
	start := time.Now()
	pass := true
	var rawParts []string
	var findings []scan.Finding

	// 1. go build ./...
	stdout, stderr, code, err := l.runner.Run(ctx, projectDir, "go", "build", "./...")
	if err != nil {
		return nil, fmt.Errorf("go build: %w", err)
	}
	rawParts = append(rawParts, combined(stdout, stderr))
	if code != 0 {
		pass = false
	}

	// 2. go vet ./...
	stdout, stderr, code, err = l.runner.Run(ctx, projectDir, "go", "vet", "./...")
	if err != nil {
		return nil, fmt.Errorf("go vet: %w", err)
	}
	rawParts = append(rawParts, combined(stdout, stderr))
	if code != 0 {
		pass = false
		findings = append(findings, parseFindings(stderr, scan.SeverityWarning)...)
	}

	// 3. staticcheck ./... — skip gracefully if the binary is not installed.
	stdout, stderr, code, err = l.runner.Run(ctx, projectDir, "staticcheck", "./...")
	switch {
	case errors.Is(err, exec.ErrNotFound):
		// not installed — skip without failing
	case err != nil:
		return nil, fmt.Errorf("staticcheck: %w", err)
	default:
		rawParts = append(rawParts, combined(stdout, stderr))
		if code != 0 {
			pass = false
			findings = append(findings, parseFindings(stderr, scan.SeverityWarning)...)
		}
	}

	return &scan.LayerResult{
		Name:      l.Name(),
		Pass:      pass,
		Findings:  findings,
		Duration:  time.Since(start),
		RawOutput: strings.Join(rawParts, "\n"),
	}, nil
}

// combined concatenates stdout and stderr into a single trimmed string.
func combined(stdout, stderr []byte) string {
	return strings.TrimRight(string(append(stdout, stderr...)), "\n")
}

// parseFindings parses lines of the form "<file>:<line>:<col>: <message>" into
// Findings. Lines starting with '#' (go vet package headers) are skipped.
func parseFindings(output []byte, severity scan.Severity) []scan.Finding {
	var findings []scan.Finding
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		f, ok := parseLine(scanner.Text(), severity)
		if ok {
			findings = append(findings, f)
		}
	}
	return findings
}

// parseLine parses a single diagnostic line. Accepted formats:
//
//	<file>:<line>:<col>: <message>
//	<file>:<line>: <message>
func parseLine(line string, severity scan.Severity) (scan.Finding, bool) {
	if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
		return scan.Finding{}, false
	}
	parts := strings.SplitN(line, ":", 4)
	if len(parts) < 3 {
		return scan.Finding{}, false
	}
	file := parts[0]
	lineNum, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return scan.Finding{}, false
	}
	var description string
	if len(parts) == 4 {
		description = strings.TrimSpace(parts[3])
	} else {
		description = strings.TrimSpace(parts[2])
	}
	if description == "" {
		return scan.Finding{}, false
	}
	return scan.Finding{
		Severity:    severity,
		File:        file,
		Line:        lineNum,
		Description: description,
	}, true
}
