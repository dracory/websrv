package websrv

import (
	"bytes"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestStartWebServer(t *testing.T) {
	port := freePort(t)
	url := "http://localhost:" + port
	var wg sync.WaitGroup
	wg.Add(1)

	shutdown := make(chan os.Signal, 1)

	// Start the web server in a goroutine
	go func() {
		defer wg.Done()
		Start(Options{
			Host:         "localhost",
			Port:         port,
			Handler:      func(w http.ResponseWriter, r *http.Request) {},
			Mode:         `testing`,
			ShutdownChan: shutdown,
		})
	}()

	// Wait for the server to start
	deadline := time.Now().Add(5 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("server did not start within 5s")
		}
		_, err := http.Get(url)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Check if the server is running
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to make a request to the server: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Server should return status OK, got %d", resp.StatusCode)
	}

	// Send a shutdown signal
	shutdown <- syscall.SIGTERM

	// Wait for the server to shut down
	wg.Wait()

	// Check if the server is shut down
	_, err = http.Get(url)
	if err == nil {
		t.Errorf("Server should be shut down")
	}
}

// freePort returns a port that is currently free by opening a listener on
// port 0 and closing it. There is an inherent race window between closing
// the listener and the caller binding to the port, but it is short enough
// for test purposes.
func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	defer ln.Close()
	return strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
}

// TestStart_ProductionExitOnStartupFailure verifies that Start calls osExit
// with code 1 when the server fails to start in production mode (e.g. the
// port is already in use).
func TestStart_ProductionExitOnStartupFailure(t *testing.T) {
	// Occupy a port so the server cannot bind to it.
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to occupy port: %v", err)
	}
	defer ln.Close()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)

	exitCalled := make(chan int, 2)
	oldExit := osExit
	osExit = func(code int) { exitCalled <- code }
	defer func() { osExit = oldExit }()

	shutdown := make(chan os.Signal, 1)

	go func() {
		Start(Options{
			Host:         "localhost",
			Port:         port,
			Handler:      func(w http.ResponseWriter, r *http.Request) {},
			Mode:         ProductionMode,
			LogLevel:     LogLevelNone,
			ShutdownChan: shutdown,
		})
	}()

	select {
	case code := <-exitCalled:
		if code != 1 {
			t.Fatalf("expected exit code 1 on startup failure, got %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("osExit was not called on startup failure")
	}

	// Unblock Start so it can finish. The fake osExit is still installed,
	// so the osExit(0) on the clean-shutdown path won't kill the test
	// process. This avoids leaking a goroutine blocked on the shutdown
	// channel.
	shutdown <- syscall.SIGTERM

	select {
	case code := <-exitCalled:
		if code != 0 {
			t.Errorf("expected exit code 0 after unblocking, got %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not finish after shutdown signal")
	}
}

// TestStart_ProductionExitOnCleanShutdown verifies that Start calls osExit
// with code 0 after a graceful shutdown in production mode.
func TestStart_ProductionExitOnCleanShutdown(t *testing.T) {
	port := freePort(t)
	url := "http://localhost:" + port

	exitCalled := make(chan int, 1)
	oldExit := osExit
	osExit = func(code int) { exitCalled <- code }
	defer func() { osExit = oldExit }()

	shutdown := make(chan os.Signal, 1)

	go func() {
		Start(Options{
			Host:         "localhost",
			Port:         port,
			Handler:      func(w http.ResponseWriter, r *http.Request) {},
			Mode:         ProductionMode,
			LogLevel:     LogLevelNone,
			ShutdownChan: shutdown,
		})
	}()

	// Wait for the server to start.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("server did not start within 5s")
		}
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Trigger graceful shutdown.
	shutdown <- syscall.SIGTERM

	select {
	case code := <-exitCalled:
		if code != 0 {
			t.Errorf("expected exit code 0 on clean shutdown, got %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("osExit was not called on clean shutdown")
	}
}

// TestLogLevelFiltering verifies that the configured LogLevel is enforced
// by the slog handler's Enabled method rather than manual if-guards.
func TestLogLevelFiltering(t *testing.T) {
	tests := []struct {
		name     string
		logLevel LogLevel
		call     func()
		wantOut  bool
	}{
		{"Info at Debug", LogLevelDebug, func() { slog.Info("msg") }, true},
		{"Info at Info", LogLevelInfo, func() { slog.Info("msg") }, true},
		{"Info at Error", LogLevelError, func() { slog.Info("msg") }, false},
		{"Info at None", LogLevelNone, func() { slog.Info("msg") }, false},
		{"Error at Error", LogLevelError, func() { slog.Error("msg") }, true},
		{"Error at None", LogLevelNone, func() { slog.Error("msg") }, false},
		{"Debug at Debug", LogLevelDebug, func() { slog.Debug("msg") }, true},
		{"Debug at Info", LogLevelInfo, func() { slog.Debug("msg") }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			oldDefault := slog.Default()
			defer slog.SetDefault(oldDefault)
			slog.SetDefault(slog.New(newSimpleHandler(&buf, logLevelToSlog(tt.logLevel))))

			tt.call()

			gotOut := buf.Len() > 0
			if gotOut != tt.wantOut {
				t.Errorf("got output=%v, want %v (buf=%q)", gotOut, tt.wantOut, buf.String())
			}
		})
	}
}
