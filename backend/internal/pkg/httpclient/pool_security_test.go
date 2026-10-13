package httpclient

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestValidatedTransportRejectsResolvedPrivateHost(t *testing.T) {
	original := validateResolvedIP
	validateResolvedIP = func(host string) error {
		if host == "internal.example" {
			return errors.New("resolved private address")
		}
		return nil
	}
	t.Cleanup(func() { validateResolvedIP = original })

	transport := newValidatedTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(http.NoBody),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}))

	req := httptest.NewRequest(http.MethodGet, "https://internal.example/health", nil)
	if _, err := transport.RoundTrip(req); err == nil {
		t.Fatal("expected resolved private host to be rejected")
	}
}

func TestValidatedTransportChecksEveryRequest(t *testing.T) {
	original := validateResolvedIP
	calls := 0
	validateResolvedIP = func(string) error {
		calls++
		return nil
	}
	t.Cleanup(func() { validateResolvedIP = original })

	transport := newValidatedTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(http.NoBody), Header: make(http.Header), Request: req}, nil
	}))
	for range 2 {
		if _, err := transport.RoundTrip(httptest.NewRequest(http.MethodGet, "https://public.example/health", nil)); err != nil {
			t.Fatalf("RoundTrip() error = %v", err)
		}
	}
	if calls != 2 {
		t.Fatalf("validation calls = %d, want 2", calls)
	}
}

func TestBuildTransportUsesSafeDialerAndRejectsProxyWhenValidationEnabled(t *testing.T) {
	transport, err := buildTransport(Options{ValidateResolvedIP: true})
	if err != nil {
		t.Fatalf("buildTransport() error = %v", err)
	}
	if transport.DialContext == nil {
		t.Fatal("validated direct transport must use SafeDialContext")
	}
	if _, err := buildTransport(Options{ValidateResolvedIP: true, ProxyURL: "socks5h://127.0.0.1:1080"}); err == nil {
		t.Fatal("expected proxy route to fail closed")
	}
	if _, err := buildTransport(Options{ValidateResolvedIP: true, AllowPrivateHosts: true, ProxyURL: "socks5h://127.0.0.1:1080"}); err != nil {
		t.Fatalf("explicit private-host opt-in should preserve proxy support: %v", err)
	}
}
