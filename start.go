package websrv

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

const TestingMode = "testing"
const ProductionMode = "production"

// DefaultMode is the default mode for the server.
const DefaultMode = ProductionMode

// Options represents the configuration for the web server.
type Options struct {
	Host     string
	Port     string
	URL      string // optional, displayed in logs
	Handler  func(w http.ResponseWriter, r *http.Request)
	Mode     string   // optional, default is production, can be development or testing
	LogLevel LogLevel // optional, default is "info", can be "debug", "info", "error", or "none"
	// ShutdownChan is an optional channel that Start listens on for shutdown
	// signals. If nil, an internal buffered channel is created. Providing a
	// channel allows callers (and tests) to trigger shutdown programmatically
	// by sending a signal to it. Real OS signals (SIGINT, SIGTERM) are also
	// forwarded to this channel via signal.Notify.
	ShutdownChan chan os.Signal
}

// LogLevel represents the level of logging.
type LogLevel string

const (
	// LogLevelDebug is the debug logging level.
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo is the info logging level.
	LogLevelInfo LogLevel = "info"
	// LogLevelError is the error logging level.
	LogLevelError LogLevel = "error"
	// LogLevelNone is the none logging level.
	LogLevelNone LogLevel = "none"
)

// osExit is indirection over os.Exit so tests can verify exit behavior
// without killing the test process. Defaults to os.Exit in production.
var osExit = os.Exit

// Start starts the web server at the specified host and port and listens
// for incoming requests. It blocks until a shutdown signal (SIGINT or
// SIGTERM) is received, then gracefully shuts the server down.
//
// Example:
//
//	websrv.Start(websrv.Options{
//		Host:    "localhost",
//		Port:    "8080",
//		Handler: func(w http.ResponseWriter, r *http.Request) {},
//		Mode:    websrv.ProductionMode,
//	})
//
// Parameters:
// - options: the server configuration (see Options)
//
// Returns:
// - server: the *Server instance (nil if shutdown failed)
// - err: a non-nil error only if graceful shutdown fails
func Start(options Options) (server *Server, err error) {
	// Set default mode if not provided
	if options.Mode == "" {
		options.Mode = DefaultMode
	}

	// Set default log level if not provided
	if options.LogLevel == "" {
		options.LogLevel = LogLevelInfo
	}

	// Route slog output to stdout so logs appear immediately (the default
	// slog handler writes to stderr, which is line-buffered by many task
	// runners and only flushed on process exit). A custom handler is used
	// to emit human-readable lines without the verbose TextHandler prefix.
	// The configured LogLevel is wired into the handler's Enabled method so
	// slog itself filters records — no manual if-guards needed at call sites.
	slog.SetDefault(slog.New(newSimpleHandler(os.Stdout, logLevelToSlog(options.LogLevel))))

	// Create the server address
	addr := options.Host + ":" + options.Port

	// Log server startup
	slog.Info("🚀 Starting server", "addr", addr)
	if options.URL != "" {
		slog.Info("🌍 APP URL", "url", options.URL)
	}

	// Create a new web server
	server = New(addr, options.Handler)

	// Resolve the shutdown channel: use the one provided by the caller
	// (or tests), or create an internal one. This keeps the package
	// reentrant — each Start call has its own channel instead of sharing
	// a package-level global.
	shutdownChan := options.ShutdownChan
	if shutdownChan == nil {
		shutdownChan = make(chan os.Signal, 1)
	}

	// Register shutdown signals
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	// Start the server in a separate goroutine. Server.Start wraps
	// ListenAndServe and swallows http.ErrServerClosed (returned on graceful
	// shutdown), so any non-nil error here is a real startup failure.
	go func() {
		if err := server.Start(); err != nil {
			slog.Error("❌ Error starting server", "err", err)
			if options.Mode != TestingMode {
				osExit(1)
			}
		}
	}()

	// Wait for a shutdown signal
	slog.Info("✅ Server is now running, press Ctrl+C to stop it.")

	sig := <-shutdownChan

	slog.Info("👋 Received signal", "sig", sig)
	slog.Info("👋 Shutting down server...")

	// Shutdown the server
	if err := server.Shutdown(context.Background()); err != nil {
		slog.Error("👋 Error shutting down server", "err", err)
		return nil, err
	}

	if options.Mode != TestingMode {
		osExit(0)
	}

	return server, nil
}
