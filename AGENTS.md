# AGENTS.md — axon-scan

## What This Module Does

axon-scan is a Go library that orchestrates multi-layer static analysis, security scanning, test execution, and LLM-driven architectural review of Go projects. Callers construct a `Pipeline` of `Layer` implementations, execute it against a project directory, and receive a `PipelineResult` that can be written out as a signed attestation.

---

## Architecture

```
             ┌──────────────────────────────────────────┐
             │              Pipeline                    │
             │  NewPipeline(layers, opts...)            │
             │  Run(ctx, projectDir) *PipelineResult    │
             └────┬──────────┬────────────┬─────────┬───┘
                  │          │            │         │
         ┌────────▼──┐  ┌────▼──────┐ ┌──▼──────┐ ┌▼────────────────┐
         │  Static   │  │ Security  │ │  Test   │ │  AgentReview    │
         │ Analysis  │  │  Scan     │ │Execution│ │    Layer        │
         │  Layer    │  │  Layer    │ │  Layer  │ │                 │
         └────┬──────┘  └────┬──────┘ └──┬──────┘ └────────┬────────┘
              │              │           │                  │
         ┌────▼──────────────▼───────────▼──┐     ┌────────▼────────┐
         │           ExecRunner             │     │   axon-loop     │
         │   (interface, injected in tests) │     │   (LoopRunner   │
         │   DefaultExecRunner: os/exec     │     │    interface)   │
         └──────────────────────────────────┘     └────────┬────────┘
                                                           │
                                               ┌───────────▼──────────┐
                                               │  axon-talk (provider)│
                                               │  + axon-tool (tools) │
                                               └──────────────────────┘

Pipeline ──► AttestationWriter ──► attestation.json + ATTESTATION.md (+ .sig)
```

---

## Module Selections

| Module | Role | Why Required |
|--------|------|-------------|
| **axon-loop** | Drives the multi-turn LLM conversation in AgentReviewLayer | Orchestrates the turn cycle so the model can call tools repeatedly until review is complete |
| **axon-talk** | LLM provider adapter (Anthropic, Ollama, Cloudflare Workers AI) | Required by axon-loop to connect to any supported backend; not imported directly by axon-scan |
| **axon-tool** | Defines the `report_finding` tool schema and execution contract | Without it axon-loop has no structured tool to call; findings cannot be collected |

**Approved axon dependencies:** axon-loop, axon-talk, axon-tool only. No other axon modules.

---

## Deterministic / Non-Deterministic Boundary Classifications

| Caller → Callee | Classification | Reason |
|-----------------|---------------|--------|
| Pipeline → StaticAnalysisLayer | **det** | Runs `go build`, `go vet`, `staticcheck` — deterministic given identical source |
| Pipeline → SecurityScanLayer | **det** | Runs `gosec` — deterministic output for identical source |
| Pipeline → TestExecutionLayer | **det** | Runs `go test -race` — deterministic pass/fail for identical source |
| Pipeline → AgentReviewLayer | **non-det** | LLM output varies per invocation |
| AgentReviewLayer → axon-loop | **non-det** | Loop orchestrates LLM conversation |
| axon-loop → axon-talk | **det** | Provider selection and API call wiring is deterministic code |
| axon-loop → axon-tool | **det** | Tool dispatch is deterministic; LLM input is non-det but tool execution is not |
| Pipeline → AttestationWriter | **det** | JSON marshalling and file I/O |
| AttestationWriter → signer func | **det** | Caller-provided signing function (e.g. HMAC, Ed25519) |
| StaticAnalysisLayer → ExecRunner | **det** | Deterministic exec abstraction |
| SecurityScanLayer → ExecRunner | **det** | Deterministic exec abstraction |
| TestExecutionLayer → ExecRunner | **det** | Deterministic exec abstraction |

---

## Dependency Graph

```
axon-scan
├── github.com/benaskins/axon-loop   (require)
│   └── github.com/benaskins/axon-talk  (indirect via axon-loop)
│   └── github.com/benaskins/axon-tape  (indirect via axon-loop)
└── github.com/benaskins/axon-tool   (require)
```

Standard library only for all other concerns (encoding/json, os/exec, text/template, sync, etc.).

---

## ExecRunner Injection Pattern

All three deterministic layers (`StaticAnalysisLayer`, `SecurityScanLayer`, `TestExecutionLayer`) accept an `ExecRunner` in their constructors:

```go
type ExecRunner interface {
    Run(ctx context.Context, dir string, name string, args ...string) (stdout, stderr []byte, exitCode int, err error)
}
```

- **Production:** pass `&scan.DefaultExecRunner{}` — delegates to `os/exec`.
- **Tests:** pass a stub that returns canned `(stdout, stderr, exitCode)` tuples. This keeps tests hermetic: no Go toolchain, no gosec, no external binaries required.

`AgentReviewLayer` uses an analogous `LoopRunner` interface to abstract `loop.Run`.

---

## Package Layout

```
axon-scan/
  types.go                  # Severity, Finding, LayerResult, PipelineResult, Layer interface
  exec.go                   # ExecRunner interface + DefaultExecRunner
  pipeline.go               # Pipeline, PipelineOption, Deduplicate
  layers/
    static.go               # StaticAnalysisLayer (go build, go vet, staticcheck)
    security.go             # SecurityScanLayer (gosec JSON)
    test.go                 # TestExecutionLayer (go test -race)
    agent.go                # AgentReviewLayer (axon-loop + report_finding tool)
  attestation/
    writer.go               # WriteAttestation → attestation.json + ATTESTATION.md + .sig
  plans/
    2026-04-03-initial-build.md
```

---

## Constraints

1. **No main package.** This is a library — no `cmd/`, no `main()`, no HTTP server.
2. **Approved axon dependencies only:** axon-loop, axon-talk (indirect), axon-tool. No axon-fact, axon-memo, axon-base, or any other axon module.
3. **Graceful degradation for optional tools.** StaticAnalysisLayer skips `staticcheck` when not in PATH. SecurityScanLayer returns pass+note when `gosec` is absent. Never hard-fail on a missing optional binary.
4. **Tests must not require external binaries.** All `exec` calls go through injected `ExecRunner`/`LoopRunner` interfaces. Tests supply stubs; no real `go`, `gosec`, or `staticcheck` invocation in the test suite.
5. **Tests must not write outside `t.TempDir()`.** All file I/O in tests is scoped to the temp directory provided by the testing framework.
6. **No auto-remediation.** axon-scan only reports findings. It never modifies source files.
7. **No axon-fact / no historical tracking.** Attestation persistence is the caller's responsibility.
8. **Attestation JSON must be self-contained.** `attestation.json` is parseable via `encoding/json` into `map[string]any` without importing axon-scan.
9. **No third-party assertion libraries in tests.** Standard `testing` package only.
