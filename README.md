# calltrail-go

`calltrail-go` maps Go RPC/API handlers to the downstream operations they call.

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

Analyze one RPC:

```sh
calltrail-go ./... --rpc GetFoo
```

Follow deeper calls:

```sh
calltrail-go ./... --rpc GetFoo --depth 5
```

The default output is Markdown summary format. Use JSON when you want raw data
for another tool:

```sh
calltrail-go ./... --rpc GetFoo --format json
```

## What It Detects

`calltrail-go` currently detects methods shaped like this:

```go
func (s *Server) GetFoo(ctx context.Context, req *pb.GetFooRequest) (*pb.GetFooResponse, error)
```

For each handler, it extracts:

- handler symbol, file, and line
- request and response types
- downstream usecase, service, repository, external client, converter, async,
  model, and notable calls
- gRPC status codes returned via `status.Error` and `status.Errorf`

With `--depth` greater than 1, `calltrail-go` follows implementation candidates
when it can infer them from interface assertions such as:

```go
var _ FooUsecase = (*fooUsecase)(nil)
```

It can also resolve common syntax-driven patterns when the relevant code is
visible to the analyzer:

- nested struct fields such as `u.repos.Foo.Find`
- local variables initialized from constructors, such as `uc := NewUsecase()`
- chained constructor calls, such as `NewUsecase().Run(ctx)`

## How It Works

`calltrail-go` is intentionally syntax-driven:

1. Walk target paths and parse non-test Go files.
2. Build a lightweight project index of functions, methods, struct fields,
   interfaces, and implementation assertions.
3. Detect handlers using configurable rules.
4. Follow calls up to `--depth` using local type inference and classifier rules.
5. Render a compact summary for humans, or raw JSON for tools.

It does not run `go list`, compile packages, or load external dependencies.
This keeps setup simple and makes the tool usable in partially configured
repositories, at the cost of some precision versus a full type checker.

## Output

Markdown output is summarized by operation. Repeated calls to the same
implementation are deduplicated and grouped under one operation with all call
sites. Low-level internal helper calls are omitted from Markdown so the output
stays readable.

```markdown
## GetFoo

- kind: `grpc`
- handler: `Server.GetFoo` (internal/driver/grpc/foo.go:12)
- request: `*pb.GetFooRequest`
- response: `*pb.GetFooResponse`

### Usecases
- `fooUsecase.GetFoo`
  - called from: `s.fooUsecase.GetFoo` (internal/driver/grpc/foo.go:18)
  - implementation: internal/usecase/foo.go:20

### Repositories
- `FooRepository.FindFoo`
  - called from: `u.repos.Foo.FindFoo` (internal/usecase/foo.go:24)
  - implementation: internal/domain/repository/foo_repository.go:30
```

JSON output keeps the raw trail data:

```sh
calltrail-go ./... --rpc GetFoo --format json
```

## Configuration

`calltrail-go` uses rule presets for handler detection, call classification, and
utility-call filtering. The built-in `generic` preset is stored as data and read
through the same rule loader as project configuration.

If `--config` is omitted, `calltrail-go` looks for `.calltrail.yaml` from each
target path upward.

```sh
calltrail-go ./... --config .calltrail.yaml
```

Example:

```yaml
handlers:
  package_names:
    - grpc
  path_contains:
    - /transport/

ignore_calls:
  auto_stdlib: true
  packages:
    - log
  symbols:
    - helper.Noop

classifiers:
  - layer: usecase
    path_contains:
      - /application/
  - layer: repository
    type_contains:
      - repository
```

The `generic` preset auto-ignores calls made through standard-library package
imports. For example, `encoding/json` is ignored through the actual local import
name such as `json.Marshal`, and aliased imports are handled as well.

## Flags

```text
--rpc string       filter by RPC/API handler name
--list             list detected handlers and exit
--depth int        call extraction depth (default 3)
--format string    output format: markdown or json (default markdown)
--preset string    rule preset (default generic)
--config string    path to .calltrail.yaml
```

Flags can be placed before or after paths:

```sh
calltrail-go ./... --rpc GetFoo
calltrail-go --rpc GetFoo ./...
```

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

If your project uses a different layout, add `.calltrail.yaml`:

```yaml
handlers:
  path_contains:
    - /transport/
    - /handler/
```

When no handlers are found, `calltrail-go` prints diagnostics to stderr,
including scanned Go file count, active preset/config, and handler rules.

### Calls are classified as Unknown

Add or tune classifier rules in `.calltrail.yaml`:

```yaml
classifiers:
  - layer: usecase
    path_contains:
      - /application/
  - layer: repository
    type_contains:
      - store
```

## Roadmap

- Type-aware call resolution with `go/packages`
- Test candidate detection
- LLM template output
- HTTP handler support
