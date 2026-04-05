# axon-scan

Multi-layer code quality pipeline: static analysis, security scanning, test execution, and LLM-driven architectural review.

## Module

- Module path: `github.com/benaskins/axon-scan`
- Project type: library (no main package)

## Build & Test

```bash
just test    # go test -race ./...
just vet     # go vet ./...
just build   # go build ./...
```

## Architecture

```
Pipeline → [StaticAnalysis, SecurityScan, TestExecution, AgentReview] → PipelineResult → Attestation
```

All deterministic layers inject `ExecRunner`; agent review injects `LoopRunner`. Tests are fully hermetic — no external binaries required.

Read [AGENTS.md](./AGENTS.md) for full architecture, boundary map, and exec injection pattern.

## Constraints

- No main package — this is a library
- Approved axon deps only: axon-loop, axon-talk (indirect), axon-tool
- External tools (staticcheck, gosec) degrade gracefully when absent
- Tests must not require external binaries — all exec calls mockable
- Tests must not write outside `t.TempDir()`
- No auto-remediation — report findings only, never modify source
- Attestation JSON must be self-contained (parseable without importing axon-scan)
- No third-party assertion libraries — standard `testing` package only
