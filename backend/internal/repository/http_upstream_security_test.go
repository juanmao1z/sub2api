package repository

import (
	"context"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestHTTPUpstreamPublicHostsOnlyRejectsProxy(t *testing.T) {
	req := httptest.NewRequest("GET", "https://public.example/asset", nil)
	req = req.WithContext(service.WithHTTPUpstreamPublicHostsOnly(context.Background()))
	if err := rejectPublicHostsOnlyProxy(req, "socks5h://proxy.example:1080"); err == nil {
		t.Fatal("expected public-host-only request to reject remote-DNS proxy")
	}
	if err := rejectPublicHostsOnlyProxy(req, ""); err != nil {
		t.Fatalf("direct public-host-only request rejected: %v", err)
	}
}

func TestHTTPUpstreamShouldValidateResolvedIPWhenPrivateHostsDisabled(t *testing.T) {
	cases := []struct {
		name           string
		allowlist      bool
		allowPrivate   bool
		wantValidation bool
	}{
		{name: "allowlist disabled still enforces public hosts", allowlist: false, allowPrivate: false, wantValidation: true},
		{name: "explicit private host opt in", allowlist: false, allowPrivate: true, wantValidation: false},
		{name: "allowlist enabled with public hosts", allowlist: true, allowPrivate: false, wantValidation: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &httpUpstreamService{cfg: &config.Config{}}
			svc.cfg.Security.URLAllowlist.Enabled = tc.allowlist
			svc.cfg.Security.URLAllowlist.AllowPrivateHosts = tc.allowPrivate
			if got := svc.shouldValidateResolvedIP(); got != tc.wantValidation {
				t.Fatalf("shouldValidateResolvedIP() = %v, want %v", got, tc.wantValidation)
			}
		})
	}
}

func TestHTTPUpstreamNilConfigFailsClosed(t *testing.T) {
	if !(&httpUpstreamService{}).shouldValidateResolvedIP() {
		t.Fatal("nil config must validate resolved IPs")
	}
}

func TestHTTPUpstreamDirectTransportsUseSafeDialers(t *testing.T) {
	settings := poolSettings{}
	transport, err := buildUpstreamTransportSecurity(settings, nil, upstreamProtocolModeDefault, true)
	if err != nil {
		t.Fatalf("buildUpstreamTransportSecurity() error = %v", err)
	}
	if transport.DialContext == nil {
		t.Fatal("validated direct transport must configure DialContext")
	}

	tlsTransport, err := buildUpstreamTransportWithTLSFingerprintSecurity(settings, nil, nil, true)
	if err != nil {
		t.Fatalf("buildUpstreamTransportWithTLSFingerprintSecurity() error = %v", err)
	}
	if tlsTransport.DialTLSContext == nil {
		t.Fatal("validated TLS fingerprint transport must configure DialTLSContext")
	}
}

func TestHTTPUpstreamRejectsProxyWhenResolvedIPValidationEnabled(t *testing.T) {
	proxyURL, err := url.Parse("socks5h://127.0.0.1:1080")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildUpstreamTransportSecurity(poolSettings{}, proxyURL, upstreamProtocolModeDefault, true); err == nil {
		t.Fatal("expected SOCKS proxy to fail closed")
	}
	if _, err := buildUpstreamTransportWithTLSFingerprintSecurity(poolSettings{}, proxyURL, nil, true); err == nil {
		t.Fatal("expected TLS SOCKS proxy to fail closed")
	}
}
