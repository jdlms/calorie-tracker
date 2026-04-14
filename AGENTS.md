Go Development Standards

This file defines coding standards and conventions for Go projects. These rules apply to all code generation, refactoring, and review tasks. When in doubt, prefer clarity over cleverness and simplicity over abstraction.

---

## Language & Toolchain

- **Go version**: Use the latest stable release. Check `go.mod` for the project's declared minimum.
- **Module path**: Always define a meaningful module path in `go.mod` (e.g., `github.com/org/repo`).
- **No CGO by default**: Unless explicitly required, build with `CGO_ENABLED=0`.

### Exploring Code & Dependencies

- To read source files from a dependency, run `go mod download -json MODULE` and use the returned `Dir` path to navigate and read the files directly.
- Use `go doc foo.Bar` to read documentation for a specific type or function, or `go doc -all foo` to read all documentation for a package.
- Use `go run .` or `go run ./cmd/foo` instead of `go build` when you just need to execute the program. It avoids leaving build artifacts behind.

---

## Project Structure

### CLI Tools

```
myapp/
├── cmd/
│   └── myapp/
│       └── main.go        # Entrypoint only. No business logic here.
├── internal/
│   ├── cli/               # Flag parsing, command dispatch
│   ├── config/            # Config loading and validation
│   └── <feature>/         # One package per domain concern
├── go.mod
├── go.sum
└── Makefile
```

### HTTP Services

```
myservice/
├── cmd/
│   └── server/
│       └── main.go        # Wire dependencies, start server. Nothing else.
├── internal/
│   ├── handler/           # HTTP handlers (thin layer — parse, call, respond)
│   ├── middleware/         # Auth, logging, tracing middleware
│   ├── service/           # Business logic (no HTTP types here)
│   ├── store/             # DB / external storage layer
│   └── config/            # Config loading and validation
├── go.mod
├── go.sum
└── Makefile
```

### Rules

- `cmd/` contains only entrypoints. `main()` should wire dependencies and call into `internal/`.
- `internal/` enforces package boundaries — nothing outside the module can import it.
- Do **not** use a `pkg/` directory. It is a Go anti-pattern that signals nothing meaningful.
- Do **not** create packages named `util`, `helpers`, or `common`. Name packages by what they do, not what they are.
- Flat is better than deeply nested. Avoid more than 3 levels of nesting in `internal/`.

---

## Code Style & Idioms

### General

- Run `gofmt` (or `goimports`) on all files. No exceptions.
- Follow the [Effective Go](https://go.dev/doc/effective_go) guide and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).
- Line length is not formally enforced, but prefer readability over compactness. ~100 chars is a soft limit.
- Avoid `init()` functions. Prefer explicit initialization in `main()` or constructors.

### Naming

- Packages: short, lowercase, single words. No underscores, no camelCase (`store`, not `dataStore`).
- Interfaces: single-method interfaces get an `-er` suffix (`Reader`, `Closer`, `Fetcher`).
- Acronyms: keep consistent casing (`userID`, `httpClient`, `parseURL` — not `userId`, `HttpClient`).
- Unexported names: use concise names. Exported names should be self-documenting.
- Avoid stuttering: don't prefix a type or function with its package name (`store.Store` not `store.StoreRepository`).

### Errors

- Always handle errors. Never assign to `_` unless you have a documented reason.
- Wrap errors with context using `fmt.Errorf("doing X: %w", err)`.
- Define sentinel errors with `errors.New` at the package level for errors callers need to check.
- Use `errors.Is` / `errors.As` for error inspection. Never string-match error messages.
- Do **not** use `panic` for control flow. Reserve it for truly unrecoverable programmer errors (e.g., invalid state at startup).

```go
// Good
if err := store.Save(ctx, record); err != nil {
    return fmt.Errorf("saving record %s: %w", record.ID, err)
}

// Bad
store.Save(ctx, record) // unchecked error
```

### Functions & Interfaces

- Functions should do one thing. If you need a comment to explain what a section of a function does, it should probably be its own function.
- Accept interfaces, return concrete types.
- Keep interfaces small. Prefer single-method interfaces. If an interface exceeds 3–4 methods, question whether it belongs together.
- Avoid constructors that take more than 4–5 parameters. Use a config struct instead.

```go
// Prefer this for complex construction
type ServerConfig struct {
    Addr         string
    ReadTimeout  time.Duration
    WriteTimeout time.Duration
    Logger       *slog.Logger
}

func NewServer(cfg ServerConfig) *Server { ... }
```

### Structs & Data

- Do **not** embed types for the sake of "inheriting" methods. Embed only when the outer type truly _is_ the inner type.
- Prefer named return values only when they add meaningful documentation. Do not use them to avoid declaring variables.
- Zero values should be useful wherever possible.

### Context

- `context.Context` is always the **first** parameter of any function that does I/O, calls external services, or runs a goroutine.
- Never store a context in a struct field. Pass it through the call chain.
- Respect context cancellation. Check `ctx.Err()` or use `select` with `ctx.Done()` in loops.

```go
// Good
func (s *Store) Get(ctx context.Context, id string) (*Record, error)

// Bad
type Store struct {
    ctx context.Context // never do this
}
```

### Logging

- Use `log/slog` (stdlib, Go 1.21+). Do not use `fmt.Println` for application logging.
- Always log with structured key-value pairs. Never interpolate variables into log messages.
- Pass a `*slog.Logger` as an explicit dependency — do not use a global logger.

```go
// Good
logger.InfoContext(ctx, "request complete", "status", status, "duration_ms", ms)

// Bad
log.Printf("request complete: status=%d duration=%dms", status, ms)
```

---

## Concurrency Patterns

The mental model: goroutines are independent workers; channels are the pipes between them. Don't communicate by sharing memory — share memory by communicating. When you find yourself reaching for a mutex, ask whether ownership transfer via a channel is a better fit. When you find yourself reaching for a channel to guard shared state, use a mutex instead.

### First Principles

- Do not start a goroutine unless you know how it will stop. Every goroutine needs an owner.
- Goroutines are cheap, not free. A leak is a bug.
- Unbuffered channel: sender blocks until receiver is ready. This is synchronization, not just data transfer.
- Buffered channel: sender only blocks when the buffer is full. Use only when the capacity is meaningful and understood.

### Channel Ownership

The goroutine that creates a channel owns it. The owner is the only one who closes it. Receivers never close. Closing from the wrong side causes a panic.

Encode ownership in the type system using directional channels:

```go
// producer owns the channel — returns receive-only to callers
func generate(nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out) // owner closes
        for _, n := range nums {
            out <- n
        }
    }()
    return out
}

// consumer accepts receive-only — cannot close or send
func square(in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for n := range in {
            out <- n * n
        }
    }()
    return out
}
```

Directional types (`chan<-`, `<-chan`) are compiler-enforced documentation. Use them in all function signatures.

### Pipeline Pattern

A pipeline is a series of stages connected by channels. Each stage consumes from an inbound channel and produces to an outbound channel. Stages are just functions — compose them freely.

```go
func main() {
    // pipeline: generate → square → print
    for n := range square(square(generate(2, 3))) {
        fmt.Println(n)
    }
}
```

Each stage is independently testable. Each runs in its own goroutine. Closing the first channel cascades through the pipeline via `range`.

### Cancellation with Done Channel / Context

A pipeline stage must be able to stop early. Pass a done signal — either a `chan struct{}` or `context.Context`. `context.Context` is preferred for anything crossing API boundaries or doing I/O.

```go
func generate(ctx context.Context, nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for _, n := range nums {
            select {
            case out <- n:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}
```

The `select` races the send against cancellation. When the context is cancelled, the goroutine exits cleanly and closes the channel, unblocking any downstream range loops.

### Fan-Out / Fan-In

Fan-out: multiple goroutines reading from the same channel in parallel to distribute work.  
Fan-in: merging multiple channels into one.

```go
// fan-out: start n workers reading from in
func fanOut(ctx context.Context, in <-chan int, n int) []<-chan int {
    outs := make([]<-chan int, n)
    for i := range n {
        outs[i] = square(ctx, in)
    }
    return outs
}

// fan-in: merge multiple channels into one
func merge(ctx context.Context, cs ...<-chan int) <-chan int {
    var wg sync.WaitGroup
    out := make(chan int)

    forward := func(c <-chan int) {
        defer wg.Done()
        for n := range c {
            select {
            case out <- n:
            case <-ctx.Done():
                return
            }
        }
    }

    wg.Add(len(cs))
    for _, c := range cs {
        go forward(c)
    }

    go func() {
        wg.Wait()
        close(out)
    }()

    return out
}
```

Fan-out without fan-in is a goroutine leak waiting to happen. Always merge results or drain channels.

### Semaphore (Bounded Concurrency)

Use a buffered channel as a counting semaphore to cap parallelism without a worker pool:

```go
const maxConcurrent = 10
sem := make(chan struct{}, maxConcurrent)

for _, item := range items {
    sem <- struct{}{} // acquire
    go func(item Item) {
        defer func() { <-sem }() // release
        process(item)
    }(item)
}

// drain: wait for all in-flight goroutines to finish
for i := 0; i < cap(sem); i++ {
    sem <- struct{}{}
}
```

This is simpler than a worker pool when you don't need result collection. Use `errgroup` with `SetLimit` when you do.

### Worker Pool

When you need bounded concurrency with result collection and error propagation:

```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(10) // at most 10 goroutines at once

for _, item := range items {
    item := item
    g.Go(func() error {
        return process(ctx, item)
    })
}

if err := g.Wait(); err != nil {
    return fmt.Errorf("processing batch: %w", err)
}
```

`errgroup.SetLimit` (added in `golang.org/x/sync` v0.1.0) replaces manual semaphore patterns when using errgroup.

### Mutex Usage

Channels transfer ownership. Mutexes protect shared state that has no single owner. Don't use one where the other fits.

```go
type Cache struct {
    mu    sync.RWMutex
    items map[string]string // protected by mu
}

func (c *Cache) Get(key string) (string, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    v, ok := c.items[key]
    return v, ok
}

func (c *Cache) Set(key, value string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.items[key] = value
}
```

- Do **not** copy a struct containing a `sync.Mutex` — pass by pointer.
- Keep the locked region as small as possible. Do not do I/O inside a lock.
- Document which fields a mutex protects.

### Avoid

- `time.Sleep` for synchronization. It is a guess, not a guarantee.
- Goroutine leaks. If a goroutine reads from a channel that no sender will ever close, it leaks forever.
- Closing a nil channel — it panics.
- Sending on a closed channel — it panics.
- `runtime.GOMAXPROCS` manipulation — let the runtime manage this.

---

## HTTP Services

- Use `net/http` stdlib. Do **not** introduce a framework (Gin, Echo, Fiber) unless performance profiling or API surface complexity justifies it.
- Register routes using `http.ServeMux` (Go 1.22+ supports method and path parameter patterns natively).
- Handlers are thin: parse the request, call a service function, write the response. No business logic in handlers.
- Always set timeouts on `http.Server`:

```go
srv := &http.Server{
    Addr:         cfg.Addr,
    Handler:      mux,
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 10 * time.Second,
    IdleTimeout:  60 * time.Second,
}
```

- Implement graceful shutdown using `srv.Shutdown(ctx)` on `SIGTERM` / `SIGINT`.
- Return structured JSON errors with consistent shape. Do not expose internal error messages to clients.

---

## CLI Tools

- Use `flag` (stdlib) for simple CLIs. For subcommands, use [`github.com/spf13/cobra`](https://github.com/spf13/cobra) only when the command tree genuinely needs it — don't add it by default.
- Read config from environment variables first, then flags. Use a config struct to consolidate both.
- Exit codes matter: `os.Exit(0)` success, `os.Exit(1)` general error, `os.Exit(2)` misuse / bad arguments.
- Write errors to `os.Stderr`, not `os.Stdout`. `os.Stdout` is for program output only.
- Never call `os.Exit` outside of `main()`. Return errors up the call chain.

```go
func main() {
    if err := run(os.Args, os.Stdout, os.Stderr); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}
```

---

## Tooling & Linting

### Required Tools

|Tool|Purpose|Install|
|---|---|---|
|`gofmt`|Formatting (built-in)|ships with Go|
|`goimports`|Formatting + import management|`go install golang.org/x/tools/cmd/goimports@latest`|
|`go vet`|Static analysis (built-in)|ships with Go|
|`staticcheck`|Extended static analysis|`go install honnef.co/go/tools/cmd/staticcheck@latest`|
|`golangci-lint`|Lint runner (aggregates many linters)|https://golangci-lint.run/usage/install/|

### Recommended `.golangci.yml` Linters

```yaml
linters:
  enable:
    - errcheck        # unhandled errors
    - gosimple        # simplification suggestions
    - govet           # go vet checks
    - ineffassign     # unused assignments
    - staticcheck     # comprehensive static analysis
    - unused          # unused code
    - goimports       # import formatting
    - misspell        # common spelling errors
    - bodyclose       # unclosed HTTP response bodies
    - noctx           # http requests without context
    - exhaustive      # exhaustive switch on enums
```

---

## Testing

- Write tests only when asked to do so.
- Use `testing` stdlib only. No testify, no gomock, no third-party assertion libraries.
- Test files live alongside the code they test (`foo_test.go` next to `foo.go`).
- Use the `_test` package suffix for black-box tests (`package store_test`). Use the same package for white-box tests when internal access is needed.
- Table-driven tests are the standard pattern for multiple input/output cases.
- Use `t.Helper()` in helper functions so failure lines point to the test, not the helper.
- Use `t.Parallel()` in tests that are safe to run concurrently. Put it at the top of the test body.

```go
func TestAdd(t *testing.T) {
    t.Parallel()

    cases := []struct {
        name string
        a, b int
        want int
    }{
        {"positive", 1, 2, 3},
        {"zero", 0, 0, 0},
        {"negative", -1, -2, -3},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            got := Add(tc.a, tc.b)
            if got != tc.want {
                t.Errorf("Add(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
            }
        })
    }
}
```

- For HTTP handler tests, use `net/http/httptest`.
- For time-dependent tests, inject a clock interface rather than calling `time.Now()` directly.
- Do **not** use `reflect.DeepEqual` for comparing structs in tests — compare fields explicitly or write a helper with `t.Helper()`.

---

## Dependencies

- Minimize dependencies. Standard library first. Ask: can this be done in 20 lines of stdlib code? If yes, do that.
- Before adding a dependency, check: Is it actively maintained? Does it have a stable API? What is its transitive dependency footprint?
- `go mod tidy` after every dependency change.
- Vendor dependencies (`go mod vendor`) if the build environment may not have network access.
- Pin dependencies to a specific version in `go.mod`. Never use pseudo-versions for production dependencies unless unavoidable.
