# axon-scan

Initialise the Go module (go mod init). Define all public types in types.go: Severity constants (info/warning/high/critical), Finding struct (Severity, File, Line, Description), LayerResult struct (Name, Pass, Findings, Duration, RawOutput), PipelineResult struct (LayerResults, Pass, StartedAt, FinishedAt). Define the Layer interface (Name() string, Run(ctx context.Context, projectDir string) (*LayerResult, error)). No logic yet — only type declarations. Test: compile the package with `go build ./...`.

## Build & Test

```bash
go test ./...
go vet ./...
just build     # builds to bin/axon-scan
just install   # copies to ~/.local/bin/axon-scan
```

## Module Selections

- **axon-loop**: Agent Review layer (Layer 3) drives an LLM conversation to perform architectural review of source files. axon-loop orchestrates the multi-turn conversation loop required for this. (non-deterministic)
- **axon-talk**: Provides the LLM provider adapter required by axon-loop to connect to Anthropic, Ollama, or Cloudflare Workers AI for the agent review layer. (non-deterministic)
- **axon-tool**: Defines the structured tool the agent calls to return findings (severity, file, line, description) from the architectural review. Without axon-tool the loop cannot produce structured output. (deterministic)

## Deterministic / Non-deterministic Boundary

| From | To | Type |
|------|----|------|
| Pipeline | StaticAnalysisLayer | det |
| Pipeline | SecurityScanLayer | det |
| Pipeline | AgentReviewLayer | non-det |
| Pipeline | TestExecutionLayer | det |
| AgentReviewLayer | axon-loop | non-det |
| axon-loop | axon-talk | det |
| axon-loop | axon-tool | det |
| Pipeline | AttestationWriter | det |
| AttestationWriter | SigningFunction | det |
| StaticAnalysisLayer | ExecRunner | det |
| SecurityScanLayer | ExecRunner | det |
| TestExecutionLayer | ExecRunner | det |

