# Server Package

![tests](https://github.com/dracory/websrv/workflows/tests/badge.svg)

The `server` package provides a simple and configurable HTTP server implementation with support for graceful shutdown, different operating modes, and configurable logging levels.

## Quick Start

```go
package main

import (
    "net/http"

    "github.com/dracory/websrv"
)

func main() {
    // Define a simple handler
    handler := func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, World!"))
    }

    // Start the server
    websrv.Start(websrv.Options{
        Host:     "localhost",
        Port:     "8080",
        URL:      "http://localhost:8080",
        Handler:  handler,
        Mode:     websrv.ProductionMode,
        LogLevel: websrv.LogLevelInfo,
    })
}
```

## Features

- **Configurable Server Options**: Set host, port, URL, handler, mode, and log level
- **Multiple Operating Modes**: Production and testing modes
- **Configurable Logging**: Debug, info, error, and none log levels
- **Graceful Shutdown**: Handles OS signals (SIGINT, SIGTERM) for clean server termination
- **Structured Logging**: Uses the standard library `log/slog` for structured server status logs

## Configuration Options

The `Options` struct provides the following configuration options:

| Option | Type | Description | Default |
|--------|------|-------------|---------|
| Host | string | The host to bind the server to | Required |
| Port | string | The port to bind the server to | Required |
| URL | string | The URL displayed in logs | Optional |
| Handler | http.HandlerFunc | The HTTP handler function | Required |
| Mode | string | Server mode (production/testing) | "production" |
| LogLevel | LogLevel | Logging level | "info" |

### Log Levels

The package supports the following log levels:

- `LogLevelDebug`: Detailed debugging information
- `LogLevelInfo`: General information about server operations
- `LogLevelError`: Error messages only
- `LogLevelNone`: No logging

### Operating Modes

- `ProductionMode`: Standard production mode with normal error handling
- `TestingMode`: Special mode for testing with different error handling

## Testing

The package includes a test file (`start_test.go`) that demonstrates how to test the server functionality, including:

- Starting the server
- Making requests to verify it's running
- Gracefully shutting down the server
- Verifying the server has shut down

## Dependencies

- Standard library only (`log/slog` for structured logging)

## Advanced Usage

### Example with Router

```go
package main

import (
    "fmt"
    "net/http"

    "github.com/dracory/router"
    "github.com/dracory/websrv"
)

func main() {
    // Create a new router
    r := router.NewRouter()

    // Add routes to the router
    r.AddRoute(router.NewRoute().
        SetMethod("GET").
        SetPath("/").
        SetHandler(func(w http.ResponseWriter, r *http.Request) {
            fmt.Fprint(w, "Welcome to the homepage!")
        }))

    r.AddRoute(router.NewRoute().
        SetMethod("GET").
        SetPath("/api/users").
        SetHandler(func(w http.ResponseWriter, r *http.Request) {
            fmt.Fprint(w, "User list API endpoint")
        }))

    // Create an API group with middleware
    apiGroup := router.NewGroup().
        SetPrefix("/api").
        AddBeforeMiddlewares([]router.Middleware{
            func(next http.Handler) http.Handler {
                return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                    // Add API key validation or other middleware logic
                    w.Header().Set("X-API-Version", "1.0")
                    next.ServeHTTP(w, r)
                })
            },
        })

    // Add routes to the API group
    apiGroup.AddRoute(router.NewRoute().
        SetMethod("GET").
        SetPath("/products").
        SetHandler(func(w http.ResponseWriter, r *http.Request) {
            fmt.Fprint(w, "Products API endpoint")
        }))

    // Add the group to the router
    r.AddGroup(apiGroup)

    // Start the server with the router as the handler
    websrv.Start(websrv.Options{
        Host:     "localhost",
        Port:     "8080",
        URL:      "http://localhost:8080",
        Handler:  r.ServeHTTP,
        Mode:     websrv.ProductionMode,
        LogLevel: websrv.LogLevelInfo,
    })
}
```

## License

This package is licensed under the [MIT License](LICENSE).
