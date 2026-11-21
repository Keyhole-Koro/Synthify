package httpserver

import (
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// A client that opens a connection and never finishes its request headers
// (Slowloris) must be cut off, so a few such clients cannot hold the server's
// connections and goroutines open indefinitely.
func TestNew_ClosesConnectionsThatNeverFinishTheirHeaders(t *testing.T) {
	saved := readHeaderTimeout
	readHeaderTimeout = 200 * time.Millisecond
	t.Cleanup(func() { readHeaderTimeout = saved })

	srv := New("127.0.0.1:0", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: example\r\n")); err != nil {
		t.Fatal(err)
	}
	// Wait well past the header timeout; the server must have hung up.
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(conn)
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Fatal("the server kept a connection with unfinished headers open")
	}

	// A normal request is still served.
	resp, err := http.Get("http://" + ln.Addr().String() + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status %d, want 204", resp.StatusCode)
	}
}
