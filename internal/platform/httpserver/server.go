// Package httpserver builds the HTTP servers of the API and the worker.
package httpserver

import (
	"net/http"
	"time"
)

// readHeaderTimeout bounds how long a client may take to send its request
// headers. Without it, clients that open connections and never finish their
// headers (Slowloris) hold the server's connections and goroutines open for
// good.
var readHeaderTimeout = 10 * time.Second

// New returns the server for handler on addr.
//
// Only the time to read the request headers is bounded. Request bodies are
// small (uploads go straight to storage), and the worker answers a job request
// only when the job ends, minutes later, so read, write and idle timeouts are
// left unset as before.
func New(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}
}
