# calltrail-go

`calltrail-go` maps Go RPC/API handlers to the downstream calls they make.

It is built for code review, onboarding, documentation, and LLM-assisted
analysis. The goal is not a perfect whole-program call graph. The goal is a
clear per-RPC summary of "what this endpoint touches".

## Status

Experimental. The current version focuses on gRPC-style handlers and lightweight
AST heuristics.

## Install

```sh
go install github.com/usuginus/calltrail-go/cmd/calltrail-go@latest
```

For local development:

```sh
go build -o /tmp/calltrail-go ./cmd/calltrail-go
```

## Quick Start

Run it from a Go repository:

```sh
calltrail-go ./...
```

List detected handlers first:

```sh
calltrail-go ./... --list
```

```md
| rpc | handler | location |
| --- | --- | --- |
| `Translate` | `TranslationController.Translate` | `internal/controller/grpc/v1/translation.go:32` |
```

Analyze one RPC:

```sh
calltrail-go ./... --rpc Translate
```

Follow deeper calls:

```sh
calltrail-go ./... --rpc Translate --depth 5
```

The default output is Markdown summary format. Use JSON when you want raw data
for another tool:

```sh
calltrail-go ./... --rpc Translate --format json
calltrail-go ./... --list --format json
```

## Examples

This repository includes small Go examples that double as regression fixtures.

Run the generic gRPC-style example with the built-in rules:

```sh
go run ./cmd/calltrail-go ./examples/grpc-basic --rpc GetBook --depth 3
```

Run the custom-layer example with its project config:

```sh
go run ./cmd/calltrail-go ./examples/custom-layers \
  --config ./examples/custom-layers/.calltrail.yaml \
  --rpc PublishArticle \
  --depth 3
```

Run the branch-dispatch example to see switch and type-switch details:

```sh
go run ./cmd/calltrail-go ./examples/branch-dispatch \
  --config ./examples/branch-dispatch/.calltrail.yaml \
  --rpc ProcessDocument \
  --depth 3
```

Run the map-dispatch example to see static dispatch tables such as
`map[Kind]Processor`:

```sh
go run ./cmd/calltrail-go ./examples/map-dispatch \
  --config ./examples/map-dispatch/.calltrail.yaml \
  --rpc ProcessDocument \
  --depth 4
```

### Public Sample Repository

`calltrail-go` is also tested manually against the public
[evrone/go-clean-template](https://github.com/evrone/go-clean-template)
repository because it uses a common Clean Architecture layout:

- `internal/controller/grpc`
- `internal/usecase`
- `internal/repo/persistent`
- `internal/repo/webapi`
- `internal/entity`

Example:

```sh
calltrail-go /path/to/go-clean-template --rpc Translate --depth 3
```

## What It Detects

`calltrail-go` currently detects methods shaped like this:

```go
func (c *TranslationController) Translate(ctx context.Context, req *v1.TranslateRequest) (*v1.TranslateResponse, error)
```

For each handler, it extracts:

- handler symbol, file, and line
- request and response types
- downstream calls grouped by configured layers, plus async and notable calls
- interface-typed calls and the implementation candidates inferred for them
- static map-dispatch calls such as `processor := a.processors[kind]`
- branch-specific calls for `switch` and `type switch` statements
- gRPC status codes returned via `status.Error` and `status.Errorf`

With `--depth` greater than 1, `calltrail-go` follows implementation candidates
when it can infer them from interface assertions such as:

```go
var _ Translation = (*UseCase)(nil)
```

It can also resolve common syntax-driven patterns when the relevant code is
visible to the analyzer:

- nested struct fields such as `u.repositories.Foo.Find`
- local variables initialized from constructors, such as `uc := NewUsecase()`
- chained constructor calls, such as `NewUsecase().Run(ctx)`
- static dispatch tables stored in struct fields, such as
  `handlers: map[Kind]Handler{KindA: newKindAHandler()}`

## How It Works

`calltrail-go` is intentionally syntax-driven:

1. Walk target paths and parse non-test Go files.
2. Build a lightweight project index of functions, methods, struct fields,
   interfaces, and implementation assertions.
3. Detect handlers using configurable rules.
4. Follow calls up to `--depth` using local type inference and layer rules.
5. Render a compact summary for humans, or raw JSON for tools.

It does not run `go list`, compile packages, or load external dependencies.
This keeps setup simple and makes the tool usable in partially configured
repositories, at the cost of some precision versus a full type checker.

## Output

Markdown output is deterministic and optimized for review, onboarding, and
LLM-assisted documentation. It renders a static call tree from syntax-driven
call relationships, then adds a compact function index with locations and
occurrence counts. Layer names come directly from the active rules, decision
points are rendered as tables, and unexported helper calls are omitted so the
output stays readable without project-specific presentation rules baked into the
binary. Interface implementations are static candidates, not runtime traces.
Decision-point tables focus on the calls selected directly by an interface,
branch, or dispatch; deeper dependencies stay in the call tree and function
index.

```markdown
## Translate

### execution summary

- kind: `grpc`
- handler: `TranslationController.Translate` (internal/controller/grpc/v1/translation.go:32)
- request: `*v1.TranslateRequest`
- response: `*v1.TranslateResponse`
- layers:
  - usecase: 1 call
  - external_client: 1 call
  - repository: 1 call
- decision points:
  - interface calls: 4
  - branches: 0
  - dispatches: 0

### call tree

- [handler] `TranslationController.Translate` (internal/controller/grpc/v1/translation.go:32)
  - [other] `grpcmw.UserIDFromContext` (internal/controller/grpc/v1/translation.go:33)
  - [usecase] `c.t.Translate` (internal/controller/grpc/v1/translation.go:38)
    - [usecase] `UseCase.Translate` (internal/usecase/translation/translation.go:36)
      - [external_client] `uc.webAPI.Translate` (internal/usecase/translation/translation.go:37)
        - [external_client] `TranslationWebAPI.Translate` (internal/repo/webapi/translation_google.go:29)
      - [repository] `uc.repo.Store` (internal/usecase/translation/translation.go:42)
        - [repository] `TranslationRepo.Store` (internal/repo/persistent/translation_postgres.go:57)
  - [other] `c.l.Error` (internal/controller/grpc/v1/translation.go:44)
    - [other] `Logger.Error` (pkg/logger/logger.go:75)

### function index

#### usecase

| function | location | occurrences |
| --- | --- | ---: |
| `c.t.Translate` | `internal/controller/grpc/v1/translation.go:38` | 1 |
| `UseCase.Translate` | `internal/usecase/translation/translation.go:36` | 1 |

#### external_client

| function | location | occurrences |
| --- | --- | ---: |
| `TranslationWebAPI.Translate` | `internal/repo/webapi/translation_google.go:29` | 1 |
| `uc.webAPI.Translate` | `internal/usecase/translation/translation.go:37` | 1 |

#### repository

| function | location | occurrences |
| --- | --- | ---: |
| `TranslationRepo.Store` | `internal/repo/persistent/translation_postgres.go:57` | 1 |
| `uc.repo.Store` | `internal/usecase/translation/translation.go:42` | 1 |

#### other

| function | location | occurrences |
| --- | --- | ---: |
| `grpcmw.UserIDFromContext` | `internal/controller/grpc/v1/translation.go:33` | 1 |
| `c.l.Error` | `internal/controller/grpc/v1/translation.go:44` | 1 |
| `Logger.Error` | `pkg/logger/logger.go:75` | 1 |

### decision points

#### interface calls

| call | interface | candidates | resolution |
| --- | --- | --- | --- |
| `c.l.Error` (internal/controller/grpc/v1/translation.go:44) | `Interface` | `Logger.Error` (pkg/logger/logger.go:75) expanded | single expanded |
| `c.t.Translate` (internal/controller/grpc/v1/translation.go:38) | `Translation` | `UseCase.Translate` (internal/usecase/translation/translation.go:36) expanded | single expanded |
| `uc.repo.Store` (internal/usecase/translation/translation.go:42) | `TranslationRepo` | `TranslationRepo.Store` (internal/repo/persistent/translation_postgres.go:57) expanded | single expanded |
| `uc.webAPI.Translate` (internal/usecase/translation/translation.go:37) | `TranslationWebAPI` | `TranslationWebAPI.Translate` (internal/repo/webapi/translation_google.go:29) expanded | single expanded |
```

JSON output keeps the raw trail data, including free-form layer names under
`trail.layers`, interface candidate details under `trail.interface_calls`, and
dispatch and branch details under `trail.dispatches` and `trail.branches`.
Error-code detection is kept in JSON because error handling is often
project-specific and can be noisy in the Markdown summary:

```sh
calltrail-go ./... --rpc Translate --format json
```

## Configuration

By default, `calltrail-go` uses conservative built-in generic rules for handler
detection, call classification, and utility-call filtering.

If `--config` is omitted, `calltrail-go` looks for `.calltrail.yaml` from each
target path upward. When a config file is found, it replaces the built-in
generic rules instead of merging with them.

```sh
calltrail-go ./... --config .calltrail.yaml
```

Start from `calltrail.example.yaml` when creating a project config. Because
project config replaces the built-in rules, keep the active rules small and
uncomment only the architecture aliases that match your project.

Example:

```yaml
version: 1

handlers:
  match:
    package_names:
      - grpc
    file_path_contains:
      - /grpc/
  signature:
    require_context_first_arg: true
    require_pointer_request: true
    require_pointer_response: true
    require_error_return: true

layers:
  - name: application
    match:
      file_path_contains:
        - /application/
  - name: repository
    match:
      receiver_type_contains:
        - repository

ignore:
  standard_library: true
  calls:
    full_names:
      - context.Background
```

The built-in generic rules auto-ignore calls made through standard-library
package imports. For example, `encoding/json` is ignored through the actual
local import name such as `json.Marshal`, and aliased imports are handled as
well.

## Flags

```text
--rpc string       filter by RPC/API handler name or receiver-qualified symbol
--list             list detected handlers and exit
--depth int        call extraction depth (default 3)
--format string    output format: markdown or json (default markdown)
--config string    path to .calltrail.yaml
```

Flags can be placed before or after paths:

```sh
calltrail-go ./... --rpc Translate
calltrail-go ./... --rpc TranslationController.Translate
calltrail-go --rpc Translate ./...
```

If multiple handlers share the same method name, use the receiver-qualified
symbol shown by `--list`, such as `TranslationController.Translate`.

## Troubleshooting

### No handlers found

Start with `--list`:

```sh
calltrail-go ./... --list
```

If the list is empty, check whether your handlers match the default generic
rules:

- package name is `grpc`, or file path contains `/grpc/`
- method has a receiver
- first argument is `context.Context`
- second argument is a pointer request
- first return value is a pointer response
- second return value is `error`

If your project uses a different layout, copy `calltrail.example.yaml` and
tune `handlers.match`:

```yaml
version: 1

handlers:
  match:
    file_path_contains:
      - /handler/
      - /transport/
  signature:
    require_context_first_arg: true
    require_pointer_request: true
    require_pointer_response: true
    require_error_return: true
```

When no handlers are found, `calltrail-go` prints diagnostics to stderr,
including scanned Go file count, active rules, and handler rules.

### Calls are classified as Unknown

Add or tune `layers` in `.calltrail.yaml`. A layer's `name` is free-form and is
used directly in Markdown and JSON output:

```yaml
version: 1

layers:
  - name: application
    match:
      call_name_contains:
        - usecase
      file_path_contains:
        - /application/
  - name: repository
    match:
      receiver_type_contains:
        - repository
```

## Benchmarking

Use Go benchmarks to track analyzer and CLI performance before and after
optimization work:

```sh
go test ./internal/analyzer -run '^$' -bench=. -benchmem
go test ./internal/cli -run '^$' -bench=. -benchmem
```

The analyzer benchmarks cover full trail extraction, RPC filtering, and handler
detection without downstream call trails. The CLI benchmarks cover `--list`,
Markdown output, and JSON output for representative fixtures.

## Roadmap

- Type-aware call resolution with `go/packages`
- Test candidate detection
- LLM template output
- HTTP handler support
