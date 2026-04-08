package main

import (
	"net/url"
	"strings"
	"testing"
)

var (
	gatewayBaseURL  = httpBaseURL(envOrDefault("GATEWAY_ADDR", ":8080"))
	loggingBaseURL  = gatewayBaseURL
	presenceBaseURL = gatewayBaseURL
)

func httpBaseURL(addr string) string {
	switch {
	case strings.HasPrefix(addr, "http://"), strings.HasPrefix(addr, "https://"):
		return strings.TrimRight(addr, "/")
	case strings.HasPrefix(addr, ":"):
		return "http://localhost" + addr
	default:
		return "http://" + strings.TrimRight(addr, "/")
	}
}

func websocketURL(baseURL, path string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return baseURL + path
	}

	switch parsed.Scheme {
	case "https":
		parsed.Scheme = "wss"
	default:
		parsed.Scheme = "ws"
	}
	parsed.Path = path
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return parsed.String()
}

func TestHTTPBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		addr string
		want string
	}{
		{name: "host port", addr: "localhost:18080", want: "http://localhost:18080"},
		{name: "leading colon", addr: ":18080", want: "http://localhost:18080"},
		{name: "http URL", addr: "http://127.0.0.1:18080", want: "http://127.0.0.1:18080"},
		{name: "https URL", addr: "https://example.test", want: "https://example.test"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := httpBaseURL(tc.addr); got != tc.want {
				t.Fatalf("httpBaseURL(%q) = %q, want %q", tc.addr, got, tc.want)
			}
		})
	}
}

func TestWebsocketURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base string
		path string
		want string
	}{
		{name: "http to ws", base: "http://localhost:18080", path: "/ws", want: "ws://localhost:18080/ws"},
		{name: "https to wss", base: "https://example.test", path: "/logs/ws", want: "wss://example.test/logs/ws"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := websocketURL(tc.base, tc.path); got != tc.want {
				t.Fatalf("websocketURL(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.want)
			}
		})
	}
}
