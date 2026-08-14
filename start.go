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

// StartWebServerbserver starts the web server at the specified host and port and listens
// for incoming requests.
//
// Example:
//
//	StartWebServer(Options{
//	 Host: "localhost",
//	 Port: "8080",
//	 Handler: func(w http.ResponseWriter, r *http.Request) {},
//	 Mode: "production",
//	})
//
// Parameters:
// - none
//
// Returns:
// - none
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

	// Start the server in a separate goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			if options.Mode == TestingMode {
				if options.LogLevel != LogLevelNone {
					slog.Error("❌ Error starting server", "err", err)
				}
			} else {
				if options.LogLevel != LogLevelNone {
					slog.Error("❌ Error starting server", "err", err)
				}
				os.Exit(1)
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
		os.Exit(0)
	}

	return server, nil
}
