package system_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PoeAudits/overlord/internal/system"
)

func TestHTTPHealthCountsAnyAnswerBelow500AsHealthy(t *testing.T) {
	for status, want := range map[int]bool{200: true, 401: true, 404: true, 500: false, 503: false} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))
		if got := (system.HTTPHealth{}).Healthy(context.Background(), server.URL); got != want {
			t.Errorf("status %d: healthy = %v, want %v", status, got, want)
		}
		server.Close()
	}
}

func TestHTTPHealthReportsAServerThatDoesNotAnswerAsUnhealthy(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()

	if (system.HTTPHealth{}).Healthy(context.Background(), url) {
		t.Fatal("a closed server is healthy")
	}
}
