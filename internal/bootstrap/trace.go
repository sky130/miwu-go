package bootstrap

import (
	"fmt"
	"io"
	"net/http"
	"sync"
)

type traceTransport struct {
	mu   sync.Mutex
	base http.RoundTripper
	out  io.Writer
}

func (t *traceTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(request)
	status := 0
	if response != nil {
		status = response.StatusCode
	}

	// Authentication URLs, headers and bodies can contain credentials.
	t.mu.Lock()
	defer t.mu.Unlock()
	fmt.Fprintf(t.out, "request method=%s host=%s status=%d failed=%t\n", request.Method, request.URL.Hostname(), status, err != nil)
	return response, err
}
