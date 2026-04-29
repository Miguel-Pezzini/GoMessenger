package observability

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestMiddlewareRecordsRouteTemplateAndStatus(t *testing.T) {
	observer := New("test-service")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	rec := httptest.NewRecorder()
	observer.Handler(mux).ServeHTTP(rec, req)

	metrics := gatherMetrics(t, observer)
	if !strings.Contains(metrics, `gomessenger_http_requests_total{method="GET",route="GET /users/{id}",service="test-service",status="201"} 1`) {
		t.Fatalf("expected route template and status in metrics, got:\n%s", metrics)
	}
	if strings.Contains(metrics, "/users/123") {
		t.Fatalf("metrics should not contain raw request paths, got:\n%s", metrics)
	}
}

func TestMiddlewareTracksInFlightRequests(t *testing.T) {
	observer := New("test-service")
	started := make(chan struct{})
	release := make(chan struct{})

	handler := observer.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/slow", nil))
		close(done)
	}()

	<-started
	metrics := gatherMetrics(t, observer)
	close(release)
	<-done

	if !strings.Contains(metrics, `gomessenger_http_in_flight_requests{method="GET",route="unknown",service="test-service"} 1`) {
		t.Fatalf("expected in-flight gauge to be 1 while request is blocked, got:\n%s", metrics)
	}
}

func TestMountReadinessAndMetrics(t *testing.T) {
	observer := New("test-service")
	mux := http.NewServeMux()
	observer.Mount(mux, FunctionCheck("dependency", func(context.Context) error {
		return errors.New("down")
	}))

	healthReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRec := httptest.NewRecorder()
	mux.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want %d", healthRec.Code, http.StatusOK)
	}

	readyReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	readyRec := httptest.NewRecorder()
	mux.ServeHTTP(readyRec, readyReq)
	if readyRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz status = %d, want %d", readyRec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(readyRec.Body.String(), "dependency") {
		t.Fatalf("expected readiness response to include check name, got %s", readyRec.Body.String())
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	mux.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d", metricsRec.Code, http.StatusOK)
	}
	if !strings.Contains(metricsRec.Body.String(), "go_goroutines") {
		t.Fatalf("expected runtime metrics, got:\n%s", metricsRec.Body.String())
	}
}

func TestPprofRoutesRequireOptIn(t *testing.T) {
	t.Setenv("OBSERVABILITY_PPROF_ENABLED", "false")
	disabledMux := http.NewServeMux()
	New("disabled").Mount(disabledMux)
	disabledRec := httptest.NewRecorder()
	disabledMux.ServeHTTP(disabledRec, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))
	if disabledRec.Code != http.StatusNotFound {
		t.Fatalf("disabled pprof status = %d, want %d", disabledRec.Code, http.StatusNotFound)
	}

	t.Setenv("OBSERVABILITY_PPROF_ENABLED", "true")
	enabledMux := http.NewServeMux()
	New("enabled").Mount(enabledMux)
	enabledRec := httptest.NewRecorder()
	enabledMux.ServeHTTP(enabledRec, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))
	if enabledRec.Code != http.StatusOK {
		t.Fatalf("enabled pprof status = %d, want %d", enabledRec.Code, http.StatusOK)
	}
}

func TestStreamMessageAge(t *testing.T) {
	age := StreamMessageAge("1712000000123-0", timeFromUnixMillis(1712000001123))
	if age.String() != "1s" {
		t.Fatalf("age = %s, want 1s", age)
	}
	if got := StreamMessageAge("invalid", timeFromUnixMillis(1712000001123)); got != -1 {
		t.Fatalf("invalid age = %s, want -1ns", got)
	}
}

func gatherMetrics(t *testing.T, observer *Observer) string {
	t.Helper()
	rec := httptest.NewRecorder()
	promhttp.HandlerFor(observer.Registry(), promhttp.HandlerOpts{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatalf("read metrics body: %v", err)
	}
	return string(body)
}

func timeFromUnixMillis(ms int64) time.Time {
	return time.UnixMilli(ms)
}
