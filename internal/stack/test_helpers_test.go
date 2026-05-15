package stack

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strconv"
	"testing"
)

func commandWithOutput(output string) *exec.Cmd {
	return exec.Command("sh", "-c", "cat <<'EOF'\n"+output+"\nEOF")
}

func commandWithOutputAndExit(output string, code int) *exec.Cmd {
	return exec.Command("sh", "-c", "cat <<'EOF'\n"+output+"\nEOF\nexit "+strconv.Itoa(code))
}

func newIPv4Server(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	server := httptest.NewUnstartedServer(handler)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on loopback: %v", err)
	}
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)

	return server
}

func mustServerHostPort(t *testing.T, rawURL string) (string, int) {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("failed to parse server URL %q: %v", rawURL, err)
	}

	host := parsed.Hostname()
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatalf("failed to parse server port from %q: %v", rawURL, err)
	}

	return host, port
}
