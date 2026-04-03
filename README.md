# axon-scan

Initialise the Go module (go mod init). Define all public types in types.go: Severity constants (info/warning/high/critical), Finding struct (Severity, File, Line, Description), LayerResult struct (Name, Pass, Findings, Duration, RawOutput), PipelineResult struct (LayerResults, Pass, StartedAt, FinishedAt). Define the Layer interface (Name() string, Run(ctx context.Context, projectDir string) (*LayerResult, error)). No logic yet — only type declarations. Test: compile the package with `go build ./...`.

## Prerequisites

- Go 1.24+
- [just](https://github.com/casey/just)

## Build & Run

```bash
just build
just install
axon-scan --help
```

## Development

```bash
just test   # run tests
just vet    # run go vet
```
