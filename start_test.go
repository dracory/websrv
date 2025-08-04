package websrv

import (
	"net/http"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestStartWebServer(t *testing.T) {
	url := "http://localhost:8080"
	var wg sync.WaitGroup
	wg.Add(1)

	// Start the web server in a goroutine
	go func() {
		defer wg.Done()
		Start(Options{
			Host:    "localhost",
			Port:    "8080",
			Handler: func(w http.ResponseWriter, r *http.Request) {},
			Mode:    `testing`,
		})
	}()

	// Wait for the server to start
	for {
		_, err := http.Get(url)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Check if the server is running
	resp, err := http.Get(url)
	if err != nil {
		t.Errorf("Failed to make a request to the server: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Server should return status OK, got %d", resp.StatusCode)
	}

	// Send a shutdown signal to the shutdownChan
	shutdownChan <- syscall.SIGTERM

	// Wait for the server to shut down
	wg.Wait()

	// Check if the server is shut down
	_, err = http.Get(url)
	if err == nil {
		t.Errorf("Server should be shut down")
	}
}
