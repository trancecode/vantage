package util

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDebugIndexHandler(t *testing.T) {
	// Create a request to pass to our handler
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(debugIndexHandler)

	// Call the handler with our request and recorded response
	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the content type
	expectedContentType := "text/html"
	if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("handler returned wrong content type: got %v want %v",
			contentType, expectedContentType)
	}

	// Check that the response contains expected links
	body := rr.Body.String()
	expectedLinks := []string{
		"/debug/pprof/",
		"/debug/pprof/goroutine",
		"/debug/pprof/heap",
		"/debug/vars",
		"Vantage Debug Server",
	}

	for _, link := range expectedLinks {
		if !strings.Contains(body, link) {
			t.Errorf("Expected response to contain %q, but it didn't", link)
		}
	}
}

func TestDebugIndexHandler404(t *testing.T) {
	// Test that non-root paths return 404
	req, err := http.NewRequest("GET", "/nonexistent", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(debugIndexHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
	}
}

func TestStartDebugHTTPServerDisabled(t *testing.T) {
	// Start the server with debug mode disabled (should be a no-op)
	StartDebugHTTPServer(0, false) // Port 0 will auto-assign if it actually starts

	// Since the function should return early when debugMode is false,
	// there's not much to test except that it doesn't panic
	// This test mainly ensures the early return logic works
}

func TestStartDebugHTTPServerEnabled(t *testing.T) {
	// Use port 0 to let the system assign an available port
	StartDebugHTTPServer(0, true)

	// Give the server a moment to start
	time.Sleep(10 * time.Millisecond)

	// We can't easily test the actual server startup without more complex setup,
	// but we can at least verify the function doesn't panic when debugMode is true.
	// A more complete test would require capturing the log output or
	// using a more sophisticated server testing approach.
}

func TestStartDebugHTTPServerWithNewFlag(t *testing.T) {
	// Test that the function works when called with explicit true flag
	// This simulates the behavior when --enable_debug_http_server=true is used
	StartDebugHTTPServer(0, true)

	// Give the server a moment to start
	time.Sleep(10 * time.Millisecond)

	// The function should not panic and should start the server
	// Since we can't easily verify the server is actually running without
	// complex setup, this mainly tests that the function executes correctly
}
