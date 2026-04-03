package layers

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	loop "github.com/benaskins/axon-loop"
	tool "github.com/benaskins/axon-tool"

	scan "github.com/benaskins/axon-scan"
)

// LoopRunner abstracts loop.Run for testability. The real implementation
// delegates to loop.Run; tests supply a stub.
type LoopRunner interface {
	Run(ctx context.Context, cfg loop.RunConfig) (*loop.Result, error)
}

// DefaultLoopRunner delegates to the axon-loop Run function.
type DefaultLoopRunner struct{}

func (DefaultLoopRunner) Run(ctx context.Context, cfg loop.RunConfig) (*loop.Result, error) {
	return loop.Run(ctx, cfg)
}

type agentReviewLayer struct {
	runner LoopRunner
}

// NewAgentReviewLayer constructs a Layer that performs an LLM-driven
// architectural review of the Go source files in projectDir.
// Inject DefaultLoopRunner{} for production use, or a stub in tests.
func NewAgentReviewLayer(runner LoopRunner) scan.Layer {
	return &agentReviewLayer{runner: runner}
}

func (l *agentReviewLayer) Name() string { return "agent-review" }

func (l *agentReviewLayer) Run(ctx context.Context, projectDir string) (*scan.LayerResult, error) {
	start := time.Now()

	prompt, err := buildAgentPrompt(projectDir)
	if err != nil {
		return nil, fmt.Errorf("agent-review: build prompt: %w", err)
	}

	// Closure state: findings accumulate as the LLM calls report_finding.
	var findings []scan.Finding

	reportFinding := tool.ToolDef{
		Name:        "report_finding",
		Description: "Report a code quality, architecture, or security finding from the review.",
		Parameters: tool.ParameterSchema{
			Type:     "object",
			Required: []string{"severity", "file", "line", "description"},
			Properties: map[string]tool.PropertySchema{
				"severity":    {Type: "string", Description: "Severity level: info, warning, high, or critical"},
				"file":        {Type: "string", Description: "File path relative to project root"},
				"line":        {Type: "integer", Description: "Line number where the finding occurs"},
				"description": {Type: "string", Description: "Clear description of the finding"},
			},
		},
		Execute: func(_ *tool.ToolContext, args map[string]any) tool.ToolResult {
			sev, _ := args["severity"].(string)
			file, _ := args["file"].(string)
			desc, _ := args["description"].(string)

			// JSON numbers arrive as float64 from map[string]any.
			var lineNum int
			switch v := args["line"].(type) {
			case float64:
				lineNum = int(v)
			case int:
				lineNum = v
			case int64:
				lineNum = int(v)
			}

			findings = append(findings, scan.Finding{
				Severity:    scan.Severity(sev),
				File:        file,
				Line:        lineNum,
				Description: desc,
			})
			return tool.ToolResult{Content: "finding recorded"}
		},
	}

	result, err := l.runner.Run(ctx, loop.RunConfig{
		Request: &loop.Request{
			Messages: []loop.Message{
				{Role: loop.RoleUser, Content: prompt},
			},
		},
		Tools: map[string]tool.ToolDef{"report_finding": reportFinding},
	})
	if err != nil {
		return nil, fmt.Errorf("agent-review: loop: %w", err)
	}

	pass := true
	for _, f := range findings {
		if f.Severity == scan.SeverityCritical {
			pass = false
			break
		}
	}

	rawOutput := ""
	if result != nil {
		rawOutput = result.Content
	}

	return &scan.LayerResult{
		Name:      l.Name(),
		Pass:      pass,
		Findings:  findings,
		Duration:  time.Since(start),
		RawOutput: rawOutput,
	}, nil
}

// buildAgentPrompt assembles the review prompt from project documentation and
// source files. AGENTS.md is included when present; its absence is not an error.
func buildAgentPrompt(projectDir string) (string, error) {
	var parts []string

	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		content, err := os.ReadFile(filepath.Join(projectDir, name))
		if err == nil {
			parts = append(parts, fmt.Sprintf("## %s\n\n%s", name, string(content)))
		}
		// Skip gracefully if the file is absent or unreadable.
	}

	err := filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, _ := filepath.Rel(projectDir, path)
		parts = append(parts, fmt.Sprintf("## %s\n\n```go\n%s\n```", rel, string(content)))
		return nil
	})
	if err != nil {
		return "", err
	}

	header := "You are performing an architectural review of a Go project. " +
		"Use the report_finding tool to report issues you find. " +
		"Focus on architecture, correctness, and security concerns. " +
		"When you have finished reviewing, respond with a short summary.\n\n"

	return header + strings.Join(parts, "\n\n"), nil
}
