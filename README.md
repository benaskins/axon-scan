# axon-scan

axon-scan is a Go library that runs a multi-layer code quality pipeline over a Go project directory and produces a signed attestation of the results.

**Layers:**

| # | Layer | What it does |
|---|-------|-------------|
| 1 | StaticAnalysisLayer | `go build`, `go vet`, `staticcheck` (optional) |
| 2 | SecurityScanLayer | `gosec` JSON output (optional, degrades gracefully) |
| 3 | AgentReviewLayer | LLM-driven architectural review via axon-loop |
| 4 | TestExecutionLayer | `go test -race ./...` |

Each layer produces a `LayerResult` (pass/fail, findings, duration, raw output). The `Pipeline` aggregates these into a `PipelineResult`. The `attestation` package writes `attestation.json` and `ATTESTATION.md` — optionally signed.

## Import

```
go get github.com/benaskins/axon-scan
```

## Usage

```go
package main

import (
    "context"
    "log"

    scan "github.com/benaskins/axon-scan"
    "github.com/benaskins/axon-scan/attestation"
    "github.com/benaskins/axon-scan/layers"
    loop "github.com/benaskins/axon-loop"
    talk "github.com/benaskins/axon-talk"
    tool "github.com/benaskins/axon-tool"
)

func main() {
    ctx := context.Background()
    projectDir := "/path/to/your/go/project"

    // Configure the LLM provider for the agent review layer.
    provider := talk.NewAnthropicProvider(talk.AnthropicConfig{
        Model: "claude-opus-4-6",
    })
    _ = tool.RegisterProvider(provider) // axon-tool wires talk into loop

    runner := &scan.DefaultExecRunner{}

    pipeline := scan.NewPipeline([]scan.Layer{
        layers.NewStaticAnalysisLayer(runner),
        layers.NewSecurityScanLayer(runner),
        layers.NewAgentReviewLayer(layers.DefaultLoopRunner{}),
        layers.NewTestExecutionLayer(runner),
    })

    result, err := pipeline.Run(ctx, projectDir)
    if err != nil {
        log.Fatal(err)
    }

    // Write attestation.json and ATTESTATION.md into the project directory.
    // Pass a signer func to also produce attestation.json.sig, or nil to skip signing.
    if err := attestation.WriteAttestation(result, projectDir, nil); err != nil {
        log.Fatal(err)
    }

    if result.Pass {
        log.Println("pipeline passed")
    } else {
        log.Println("pipeline failed — see ATTESTATION.md for findings")
    }

    // Optionally deduplicate findings across layers (highest severity wins per file:line).
    findings := result.Deduplicate()
    for _, f := range findings {
        log.Printf("[%s] %s:%d — %s\n", f.Severity, f.File, f.Line, f.Description)
    }
}
```

## Attestation Output

`WriteAttestation` produces two files in the target directory:

- **`attestation.json`** — full `PipelineResult` as JSON; self-contained, parseable without importing axon-scan.
- **`ATTESTATION.md`** — human-readable summary: overall pass/fail, per-layer table, findings grouped by severity.
- **`attestation.json.sig`** — written only when a `signer` function is provided (e.g. HMAC-SHA256, Ed25519).

## Graceful Degradation

- `staticcheck` absent from PATH → `StaticAnalysisLayer` skips it, still runs `go build` and `go vet`.
- `gosec` absent from PATH → `SecurityScanLayer` returns pass with a note in `RawOutput`.

## Custom Layers

Implement the `Layer` interface to add your own layers:

```go
type Layer interface {
    Name() string
    Run(ctx context.Context, projectDir string) (*LayerResult, error)
}
```

## Testing

All exec calls are injected via `ExecRunner` and `LoopRunner` interfaces — no real Go toolchain or LLM needed in tests:

```go
// In your tests, inject a stub instead of DefaultExecRunner:
type stubRunner struct{}

func (s stubRunner) Run(_ context.Context, _ string, name string, args ...string) ([]byte, []byte, int, error) {
    return []byte("ok"), nil, 0, nil
}
```

## Prerequisites

- Go 1.26+
- [just](https://github.com/casey/just) (for development targets)
- `staticcheck` and `gosec` are optional — layers skip them gracefully if absent.
