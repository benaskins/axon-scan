# axon-scan — Initial Build Plan
# 2026-04-03

Each step is commit-sized. Execute via `/iterate`.

## Step 1 — Scaffold library module and core types

Initialise the Go module (go mod init). Define all public types in types.go: Severity constants (info/warning/high/critical), Finding struct (Severity, File, Line, Description), LayerResult struct (Name, Pass, Findings, Duration, RawOutput), PipelineResult struct (LayerResults, Pass, StartedAt, FinishedAt). Define the Layer interface (Name() string, Run(ctx context.Context, projectDir string) (*LayerResult, error)). No logic yet — only type declarations. Test: compile the package with `go build ./...`.

Commit: `feat: scaffold library module and define core public types`

## Step 2 — Add ExecRunner abstraction for testable shell-out

Define an ExecRunner interface in exec.go: `type ExecRunner interface { Run(ctx context.Context, dir string, name string, args ...string) (stdout, stderr []byte, exitCode int, err error) }`. Provide a DefaultExecRunner implementation that wraps os/exec. This abstraction is the sole path through which layers invoke external binaries, making all exec calls injectable in tests. Test: unit-test DefaultExecRunner against a guaranteed-available binary (e.g. `echo hello`), asserting stdout and exit 0.

Commit: `feat: add ExecRunner abstraction for testable shell-out`

## Step 3 — Implement StaticAnalysisLayer

Implement StaticAnalysisLayer in layers/static.go. Constructor: `NewStaticAnalysisLayer(runner ExecRunner) Layer`. Run order: (1) `go build ./...`, (2) `go vet ./...`, (3) `staticcheck ./...` — skip step 3 gracefully if `staticcheck` is not found in PATH (detect via exec.LookPath before running). Capture combined stdout+stderr for each command into RawOutput. Pass=true only if all executed commands exit 0. Map vet/staticcheck stderr lines to Findings with appropriate severity. Test: inject a mock ExecRunner that simulates pass, fail, and missing-binary scenarios. Assert Pass field and Findings slice. Do not require go or staticcheck to be installed in the test environment.

Commit: `feat: implement StaticAnalysisLayer (go build, go vet, staticcheck)`

## Step 4 — Implement SecurityScanLayer with gosec JSON parsing

Implement SecurityScanLayer in layers/security.go. Constructor: `NewSecurityScanLayer(runner ExecRunner) Layer`. Run `gosec -fmt json ./...` in projectDir. If gosec is not found in PATH, return LayerResult{Pass: true, RawOutput: "gosec not installed, skipping"}. Parse the JSON output (gosec Issues array) into Findings. Pass=true if no findings with severity HIGH or CRITICAL. Findings with severity MEDIUM/LOW/INFO are recorded but do not fail the layer. Test: inject mock ExecRunner returning canned gosec JSON payloads (empty, low-only, high-present, gosec-missing). Assert Pass and Findings counts. All tests use t.TempDir() for any file I/O.

Commit: `feat: implement SecurityScanLayer (gosec JSON parsing)`

## Step 5 — Implement TestExecutionLayer

Implement TestExecutionLayer in layers/test.go. Constructor: `NewTestExecutionLayer(runner ExecRunner) Layer`. Run `go test -race ./...` in projectDir. Capture stdout+stderr into RawOutput. Pass=true if exit code 0. No finding parsing — test failures are surfaced via RawOutput. Test: inject mock ExecRunner for pass (exit 0) and fail (exit 1) cases; assert Pass field and RawOutput content. No actual Go toolchain required in tests.

Commit: `feat: implement TestExecutionLayer (go test -race)`

## Step 6 — Implement AgentReviewLayer with axon-loop and report_finding tool

Implement AgentReviewLayer in layers/agent.go. Constructor: `NewAgentReviewLayer(loop axonloop.Loop) Layer`. Define a `report_finding` axon-tool with input schema {severity: string, file: string, line: int, description: string}. The tool handler appends findings to a slice held in closure state. Build the prompt by reading CLAUDE.md, AGENTS.md (skip gracefully if absent), and all *.go files from projectDir, then run the axon-loop with the prompt and the report_finding tool. After the loop completes, collect accumulated findings. Pass=true if no CRITICAL-severity findings were reported. Test: provide a stub axon-loop that immediately calls the tool with a canned payload; assert finding collection and Pass logic. Layer must work with any axon-talk provider — no provider is hard-coded.

Commit: `feat: define report_finding tool and implement AgentReviewLayer`

## Step 7 — Implement Pipeline with sequential and parallel execution

Implement Pipeline in pipeline.go. Constructor: `NewPipeline(layers []Layer, opts ...PipelineOption) *Pipeline`. PipelineOption supports: WithParallel(bool) to run all non-dependent layers concurrently, WithLayers([]Layer) to override layer selection at run time. Run(ctx, projectDir) executes layers (sequentially by default, concurrently with WithParallel), collects LayerResults, sets PipelineResult.Pass = all layers passed, records StartedAt/FinishedAt. Test: build a pipeline with two mock layers (one pass, one fail); assert PipelineResult.Pass=false. Test parallel mode with a mock that records call order. No external tools required.

Commit: `feat: implement Pipeline with sequential and parallel execution modes`

## Step 8 — Add cross-layer finding deduplication

Add a Deduplicate() method (or post-processing step in Pipeline.Run) that removes duplicate findings across all LayerResults where File+Line are identical. The canonical finding kept is the highest-severity duplicate. Test: construct a PipelineResult with two layers each reporting the same file+line finding and one unique finding; assert deduplicated slice length and that the highest severity is retained.

Commit: `feat: add finding deduplication to PipelineResult`

## Step 9 — Implement attestation generation and optional signing

Implement AttestationWriter in attestation/writer.go. Function: `WriteAttestation(result *PipelineResult, dir string, signer func([]byte) ([]byte, error)) error`. Produce: (1) attestation.json — marshalled PipelineResult using only stdlib-compatible types, no unexported fields, self-contained without library import; (2) ATTESTATION.md — human-readable summary rendered from a text/template: overall pass/fail, per-layer table (name, pass, duration, finding count), full findings list grouped by severity. If signer is non-nil, call signer(jsonBytes) and write the returned bytes to attestation.json.sig. All files written inside dir. Test: call WriteAttestation with a canned PipelineResult into t.TempDir(); assert both files are created, JSON round-trips cleanly, and .sig is written when a stub signer is provided. Verify attestation.json is decodable by encoding/json into a plain map[string]any without importing axon-scan.

Commit: `feat: implement attestation generation (ATTESTATION.md + attestation.json)`

## Step 10 — Add justfile with build, test, and vet targets

Add a justfile with targets: `build` (go build ./...), `test` (go test -race ./...), `vet` (go vet ./...), `check` (runs vet then test). No external tool targets — callers manage staticcheck/gosec themselves. Verify `just build` and `just test` succeed in a clean environment.

Commit: `infra: add justfile with build, test, and vet targets`

## Step 11 — Write AGENTS.md, CLAUDE.md, and README.md

AGENTS.md: document architecture, module selections (axon-loop, axon-talk, axon-tool), boundary classifications (det/non-det), dependency graph, ExecRunner injection pattern, and constraint list. CLAUDE.md: working instructions — how to run tests (just test), constraint reminders (no axon outside approved list, no writes outside TempDir in tests, no auto-remediation), package layout (types.go, exec.go, pipeline.go, layers/, attestation/). README.md: what axon-scan is, how to import it as a library, a minimal usage example constructing a pipeline with all four layers and writing an attestation.

Commit: `docs: write AGENTS.md, CLAUDE.md, and README.md`

## Step 12 — Write initial build plan document

Create plans/YYYY-MM-DD-initial-build.md listing all eleven plan steps with titles, commit messages, and descriptions. This serves as the implementation roadmap for the coding agent.

Commit: `docs: write initial build plan`

