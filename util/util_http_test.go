package util

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	srv, err := StartDebugHTTPServer(0, false) // Port 0 will auto-assign if it actually starts
	if err != nil {
		t.Fatalf("StartDebugHTTPServer returned an error: %v", err)
	}
	if srv != nil {
		t.Fatal("StartDebugHTTPServer returned a non-nil server when debugMode is false")
	}
}

func TestStartDebugHTTPServerEnabled(t *testing.T) {
	// Use port 0 to let the system assign an available port
	srv, err := StartDebugHTTPServer(0, true)
	if err != nil {
		t.Fatalf("StartDebugHTTPServer returned an error: %v", err)
	}
	t.Cleanup(func() {
		if err := srv.Shutdown(context.Background()); err != nil {
			t.Errorf("failed to shut down debug HTTP server: %v", err)
		}
	})

	// The listener is bound before StartDebugHTTPServer returns, so the
	// server is already accepting connections here.
	resp, err := http.Get("http://" + srv.Addr)
	if err != nil {
		t.Fatalf("failed to reach debug HTTP server: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
}

func TestStartDebugHTTPServerWithNewFlag(t *testing.T) {
	// Test that the function works when called with explicit true flag
	// This simulates the behavior when --enable_debug_http_server=true is used
	srv, err := StartDebugHTTPServer(0, true)
	if err != nil {
		t.Fatalf("StartDebugHTTPServer returned an error: %v", err)
	}
	t.Cleanup(func() {
		if err := srv.Shutdown(context.Background()); err != nil {
			t.Errorf("failed to shut down debug HTTP server: %v", err)
		}
	})
}
