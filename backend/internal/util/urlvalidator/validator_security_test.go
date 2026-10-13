package urlvalidator

import (
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestValidateHTTPURLBlocksPrivateLiteralWhenConfigured(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1:8080",
		"https://10.0.0.7/metadata",
		"http://[::1]/health",
		"https://localhost/internal",
	} {
		if _, err := ValidateHTTPURL(raw, true, ValidationOptions{AllowPrivate: false}); err == nil {
			t.Fatalf("expected private host to be rejected: %s", raw)
		}
	}
}

func TestValidateHTTPURLPreservesPrivateHostOptIn(t *testing.T) {
	got, err := ValidateHTTPURL("http://127.0.0.1:8080/", true, ValidationOptions{AllowPrivate: true})
	if err != nil {
		t.Fatalf("expected explicitly allowed private host, got %v", err)
	}
	if got != "http://127.0.0.1:8080" {
		t.Fatalf("normalized URL = %q", got)
	}
}

func TestValidateHTTPURLStillRequiresAllowlistWhenRequested(t *testing.T) {
	if _, err := ValidateHTTPURL("https://example.com", false, ValidationOptions{RequireAllowlist: true}); err == nil {
		t.Fatal("expected an empty required allowlist to be rejected")
	}
}

func TestResolvePublicIPsRejectsBlockedAddressFamilies(t *testing.T) {
	blocked := []string{
		"127.0.0.1",
		"10.1.2.3",
		"169.254.1.1",
		"100.64.1.1",
		"0.1.2.3",
		"224.0.0.1",
		"fc00::1",
		"fe80::1",
		"::",
	}
	for _, raw := range blocked {
		t.Run(raw, func(t *testing.T) {
			_, err := ResolvePublicIPsWithLookup(context.Background(), "example.test", func(context.Context, string) ([]net.IP, error) {
				return []net.IP{net.ParseIP(raw)}, nil
			})
			if err == nil {
				t.Fatalf("expected %s to be rejected", raw)
			}
		})
	}
}

func TestResolvePublicIPsRejectsMixedPublicAndPrivateRecords(t *testing.T) {
	_, err := ResolvePublicIPsWithLookup(context.Background(), "example.test", func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.10"), net.ParseIP("10.0.0.8")}, nil
	})
	if err == nil {
		t.Fatal("expected any private record to reject the complete DNS result")
	}
}

func TestSafeDialContextUsesVerifiedIPsAndTriesAllRecords(t *testing.T) {
	var dialed []string
	lookup := func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.10"), net.ParseIP("203.0.113.11")}, nil
	}
	dial := func(_ context.Context, network, address string) (net.Conn, error) {
		dialed = append(dialed, network+":"+address)
		if len(dialed) == 1 {
			return nil, errors.New("first address unavailable")
		}
		return &fakeConn{}, nil
	}
	conn, err := SafeDialContextWithLookup(context.Background(), "tcp", "example.test:443", lookup, dial)
	if err != nil {
		t.Fatalf("SafeDialContextWithLookup() error = %v", err)
	}
	_ = conn.Close()
	want := []string{"tcp:203.0.113.10:443", "tcp:203.0.113.11:443"}
	if !reflect.DeepEqual(dialed, want) {
		t.Fatalf("dialed = %v, want %v", dialed, want)
	}
}

func TestSafeDialContextFailsClosedOnDNSFailure(t *testing.T) {
	_, err := SafeDialContextWithLookup(context.Background(), "tcp", "example.test:443", func(context.Context, string) ([]net.IP, error) {
		return nil, errors.New("resolver unavailable")
	}, func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("dial must not run after DNS failure")
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected DNS failure")
	}
}

type fakeConn struct{}

func (f *fakeConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (f *fakeConn) Write(p []byte) (int, error)      { return len(p), nil }
func (f *fakeConn) Close() error                     { return nil }
func (f *fakeConn) LocalAddr() net.Addr              { return fakeAddr("local") }
func (f *fakeConn) RemoteAddr() net.Addr             { return fakeAddr("remote") }
func (f *fakeConn) SetDeadline(time.Time) error      { return nil }
func (f *fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (f *fakeConn) SetWriteDeadline(time.Time) error { return nil }

type fakeAddr string

func (a fakeAddr) Network() string { return "tcp" }
func (a fakeAddr) String() string  { return string(a) }
