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

var shutdownChan = make(chan os.Signal, 1)

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
	// Route slog output to stdout so logs appear immediately (the default
	// slog handler writes to stderr, which is line-buffered by many task
	// runners and only flushed on process exit). A custom handler is used
	// to emit human-readable lines without the verbose TextHandler prefix.
	slog.SetDefault(slog.New(newSimpleHandler(os.Stdout)))

	// Set default mode if not provided
	if options.Mode == "" {
		options.Mode = DefaultMode
	}

	// Set default log level if not provided
	if options.LogLevel == "" {
		options.LogLevel = LogLevelInfo
	}

	// Create the server address
	addr := options.Host + ":" + options.Port

	// Log server startup
	if options.LogLevel == LogLevelDebug || options.LogLevel == LogLevelInfo {
		slog.Info("🚀 Starting server", "addr", addr)
		if options.URL != "" {
			slog.Info("🌍 APP URL", "url", options.URL)
		}
	}

	// Create a new web server
	server = New(addr, options.Handler)

	// Register shutdown signals
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	// Start the server in a separate goroutine. Server.Start wraps
	// ListenAndServe and swallows http.ErrServerClosed (returned on graceful
	// shutdown), so any non-nil error here is a real startup failure.
	go func() {
		if err := server.Start(); err != nil {
			if options.LogLevel != LogLevelNone {
				slog.Error("❌ Error starting server", "err", err)
			}
			if options.Mode != TestingMode {
				osExit(1)
			}
		}
	}()

	// Wait for a shutdown signal
	if options.LogLevel == LogLevelDebug || options.LogLevel == LogLevelInfo {
		slog.Info("✅ Server is now running, press Ctrl+C to stop it.")
	}

	sig := <-shutdownChan

	if options.LogLevel == LogLevelDebug || options.LogLevel == LogLevelInfo {
		slog.Info("👋 Received signal", "sig", sig)
		slog.Info("👋 Shutting down server...")
	}

	// Shutdown the server
	if err := server.Shutdown(context.Background()); err != nil {
		if options.LogLevel != LogLevelNone {
			slog.Error("👋 Error shutting down server", "err", err)
		}
		return nil, err
	}

	if options.Mode != TestingMode {
		osExit(0)
	}

	return server, nil
}
