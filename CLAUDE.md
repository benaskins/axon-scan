# CLAUDE.md — axon-scan

## What This Is

axon-scan is a Go library (`github.com/benaskins/axon-scan`) that orchestrates a multi-layer code quality pipeline: static analysis, security scanning, test execution, and LLM-driven architectural review. It is a library — no `main()`, no CLI entry point, no HTTP server.

## Module

- Module path: `github.com/benaskins/axon-scan`
- Project type: library (no main package)
- Go version: 1.26

## Build & Test

```bash
just build     # go build ./...
just test      # go test -race ./...
just vet       # go vet ./...
just check     # vet, then test
```

## Package Layout

```
axon-scan/
  types.go           # Severity, Finding, LayerResult, PipelineResult, Layer interface
  exec.go            # ExecRunner interface + DefaultExecRunner (os/exec)
  pipeline.go        # Pipeline, PipelineOption (WithParallel, WithLayers), Deduplicate
  layers/
    static.go        # StaticAnalysisLayer (go build, go vet, staticcheck)
    security.go      # SecurityScanLayer (gosec JSON output)
    test.go          # TestExecutionLayer (go test -race)
    agent.go         # AgentReviewLayer (axon-loop + report_finding tool)
  attestation/
    writer.go        # WriteAttestation → attestation.json + ATTESTATION.md + optional .sig
  plans/
    2026-04-03-initial-build.md
```

## Constraints

Do not violate these without an explicit change request.

- **No main package.** This is a library. Never add `cmd/` or `func main()`.
- **Approved axon dependencies only:** axon-loop, axon-talk (indirect), axon-tool. Do not import axon-fact, axon-memo, axon-base, or any other axon module.
- **Tests must not require external binaries.** All `exec` calls go through injected `ExecRunner`/`LoopRunner` interfaces. Tests supply stubs — no real `go`, `gosec`, or `staticcheck` invocations.
- **Tests must not write outside `t.TempDir()`.** All file I/O in tests must be scoped to the temp directory.
- **No auto-remediation.** axon-scan reports findings only. It must never modify source files.
- **No axon-fact / no historical tracking.** Attestation persistence is the caller's responsibility.
- **No third-party assertion libraries.** Standard `testing` package only.
- **Optional tools degrade gracefully.** `staticcheck` and `gosec` are skipped (not failed) when absent from PATH.
- **Attestation JSON must be self-contained.** Parseable via `encoding/json` into `map[string]any` without importing axon-scan.

## Key Interfaces

```go
// Layer — implement to add a new scan layer
type Layer interface {
    Name() string
    Run(ctx context.Context, projectDir string) (*LayerResult, error)
}

// ExecRunner — inject DefaultExecRunner{} in production, stub in tests
type ExecRunner interface {
    Run(ctx context.Context, dir string, name string, args ...string) (stdout, stderr []byte, exitCode int, err error)
}

// LoopRunner (layers package) — inject DefaultLoopRunner{} in production, stub in tests
type LoopRunner interface {
    Run(ctx context.Context, cfg loop.RunConfig) (*loop.Result, error)
}
```

## TDD Practice

For each plan step:

1. Write a failing test first.
2. Make it pass with the minimal implementation.
3. Clean up without breaking tests.
4. Run `just test` — all tests must pass before committing.
5. Stage only files related to the current step (`git status`).
6. One conventional commit per step (`feat:`, `fix:`, `refactor:`, `docs:`, `infra:`).

Stop if a step reveals a design question the plan did not anticipate, or tests fail for unrelated reasons.

## Axon Framework Notes

- **axon-loop**: Orchestrates multi-turn LLM conversation in `AgentReviewLayer`.
- **axon-talk**: LLM provider adapter — not imported directly; used transitively by axon-loop.
- **axon-tool**: Defines the `report_finding` tool schema. Required for structured output from the LLM.

Boundary classification:
- Caller → AgentReviewLayer: **non-det** (LLM output varies)
- Caller → all other layers: **det** (deterministic given identical source)
- axon-loop → axon-talk: **det** (provider wiring is deterministic code)
