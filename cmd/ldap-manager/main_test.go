package main

// Tests for main.go's CLI helpers.

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestIsValidPort(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"0", false},
		{"1", true},
		{"80", true},
		{"3000", true},
		{"65535", true},
		{"65536", false},
		{"99999", false}, // >65535
		{"123456", false},
		{"abc", false},
		{"12a", false},
		{"12/3", false},
		{"-1", false},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := isValidPort(tc.in); got != tc.want {
				t.Errorf("isValidPort(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestRunHealthCheck(t *testing.T) {
	t.Run("rejects invalid port before issuing request", func(t *testing.T) {
		if got := runHealthCheck("abc"); got != 1 {
			t.Errorf("expected 1 for invalid port, got %d", got)
		}

		if got := runHealthCheck(""); got != 1 {
			t.Errorf("expected 1 for empty port, got %d", got)
		}
	})

	t.Run("returns 1 when no server is listening", func(t *testing.T) {
		// Acquire a known-free ephemeral port from the OS, close it, and reuse
		// the number. This avoids the flaky assumption that a hard-coded port
		// (e.g. 65535) is available on shared CI runners.
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}

		port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
		_ = l.Close()

		got := runHealthCheck(port)
		if got != 1 {
			t.Errorf("expected 1 when no server listening, got %d", got)
		}
	})

	t.Run("returns 0 for healthy HTTP 200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/health/live") {
				w.WriteHeader(http.StatusNotFound)

				return
			}

			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		u, err := url.Parse(srv.URL)
		if err != nil {
			t.Fatalf("parse URL: %v", err)
		}

		port := u.Port()
		if got := runHealthCheck(port); got != 0 {
			t.Errorf("expected 0 for healthy server, got %d", got)
		}
	})

	t.Run("returns 1 for non-200 response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		u, err := url.Parse(srv.URL)
		if err != nil {
			t.Fatalf("parse URL: %v", err)
		}

		if got := runHealthCheck(u.Port()); got != 1 {
			t.Errorf("expected 1 for 503, got %d", got)
		}
	})
}
