package util

import (
	"expvar"
	"fmt"
	"net/http"
	"net/http/pprof"
)

// StartDebugHTTPServer starts the debug HTTP server on the specified port.
// This server provides access to pprof and expvar endpoints for debugging.
// It should only be called when debug mode is enabled.
func StartDebugHTTPServer(port int, debugMode bool) {
	if !debugMode {
		return
	}

	// Create a new ServeMux for this server instance to avoid conflicts
	mux := http.NewServeMux()

	// Set up the root handler that shows links to debug pages
	mux.HandleFunc("/", debugIndexHandler)

	// Register pprof handlers
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// Register expvar handler
	mux.Handle("/debug/vars", expvar.Handler())

	addr := fmt.Sprintf(":%d", port)
	Logger.Info().Msgf("Starting debug HTTP server on %s", addr)

	// Start the server in a goroutine so it doesn't block the main game loop
	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			Logger.Error().Err(err).Msg("Debug HTTP server error")
		}
	}()
}

// debugIndexHandler handles the root "/" path and displays links to available debug endpoints.
func debugIndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	html := `<!DOCTYPE html>
<html>
<head>
    <title>Vantage Debug Server</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        h1 { color: #333; }
        ul { list-style-type: none; padding: 0; }
        li { margin: 10px 0; }
        a { text-decoration: none; color: #0066cc; font-size: 16px; }
        a:hover { text-decoration: underline; }
        .description { color: #666; font-size: 14px; margin-left: 20px; }
    </style>
</head>
<body>
    <h1>Vantage Debug Server</h1>
    <p>Welcome to the Vantage debug server. Use the links below to access debugging information:</p>
    
    <h2>Performance Profiling (pprof)</h2>
    <ul>
        <li><a href="/debug/pprof/">pprof index</a><span class="description">- Overview of all available profiles</span></li>
        <li><a href="/debug/pprof/goroutine">goroutines</a><span class="description">- Currently running goroutines</span></li>
        <li><a href="/debug/pprof/heap">heap</a><span class="description">- Memory heap profile</span></li>
        <li><a href="/debug/pprof/profile">CPU profile</a><span class="description">- 30-second CPU profile</span></li>
        <li><a href="/debug/pprof/block">blocking</a><span class="description">- Goroutine blocking profile</span></li>
        <li><a href="/debug/pprof/mutex">mutex</a><span class="description">- Mutex contention profile</span></li>
        <li><a href="/debug/pprof/threadcreate">thread creation</a><span class="description">- Thread creation profile</span></li>
    </ul>
    
    <h2>Runtime Variables (expvar)</h2>
    <ul>
        <li><a href="/debug/vars">runtime variables</a><span class="description">- Exported variables in JSON format</span></li>
    </ul>
</body>
</html>`

	_, _ = fmt.Fprint(w, html)
}
