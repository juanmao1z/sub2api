package urlvalidator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ValidationOptions struct {
	AllowedHosts     []string
	RequireAllowlist bool
	AllowPrivate     bool
}

// ValidateHTTPURL validates an outbound HTTP/HTTPS URL.
//
// It provides a single validation entry point that supports:
// - scheme 校验（https 或可选允许 http）
// - 可选 allowlist（支持 *.example.com 通配）
// - allow_private_hosts 策略（阻断 localhost/私网字面量 IP）
//
// 注意：DNS Rebinding 防护（解析后 IP 校验）应在实际发起请求时执行，避免 TOCTOU。
func ValidateHTTPURL(raw string, allowInsecureHTTP bool, opts ValidationOptions) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("url is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid url: %s", trimmed)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && (!allowInsecureHTTP || scheme != "http") {
		return "", fmt.Errorf("invalid url scheme: %s", parsed.Scheme)
	}

	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return "", errors.New("invalid host")
	}
	if !opts.AllowPrivate && isBlockedHost(host) {
		return "", fmt.Errorf("host is not allowed: %s", host)
	}

	if port := parsed.Port(); port != "" {
		num, err := strconv.Atoi(port)
		if err != nil || num <= 0 || num > 65535 {
			return "", fmt.Errorf("invalid port: %s", port)
		}
	}

	allowlist := normalizeAllowlist(opts.AllowedHosts)
	if opts.RequireAllowlist && len(allowlist) == 0 {
		return "", errors.New("allowlist is not configured")
	}
	if len(allowlist) > 0 && !isAllowedHost(host, allowlist) {
		return "", fmt.Errorf("host is not allowed: %s", host)
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func ValidateURLFormat(raw string, allowInsecureHTTP bool) (string, error) {
	// 最小格式校验：仅保证 URL 可解析且 scheme 合规，不做白名单/私网/SSRF 校验
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("url is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid url: %s", trimmed)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && (!allowInsecureHTTP || scheme != "http") {
		return "", fmt.Errorf("invalid url scheme: %s", parsed.Scheme)
	}

	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "", errors.New("invalid host")
	}

	if port := parsed.Port(); port != "" {
		num, err := strconv.Atoi(port)
		if err != nil || num <= 0 || num > 65535 {
			return "", fmt.Errorf("invalid port: %s", port)
		}
	}

	return strings.TrimRight(trimmed, "/"), nil
}

func ValidateHTTPSURL(raw string, opts ValidationOptions) (string, error) {
	return ValidateHTTPURL(raw, false, opts)
}

// LookupIPFunc resolves all A/AAAA records for a hostname.
type LookupIPFunc func(context.Context, string) ([]net.IP, error)

// ResolvePublicIPs resolves host and rejects the entire result set if any address
// is outside the public address space. DNS failures and empty results fail closed.
func ResolvePublicIPs(ctx context.Context, host string) ([]net.IP, error) {
	return ResolvePublicIPsWithLookup(ctx, host, defaultLookupIP)
}

// ResolvePublicIPsWithLookup is the deterministic/testable form of ResolvePublicIPs.
func ResolvePublicIPsWithLookup(ctx context.Context, host string, lookup LookupIPFunc) ([]net.IP, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, errors.New("host is empty")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isDisallowedIP(ip) {
			return nil, fmt.Errorf("resolved ip %s is not allowed", ip.String())
		}
		return []net.IP{ip}, nil
	}
	if lookup == nil {
		return nil, errors.New("dns resolver is nil")
	}
	ips, err := lookup(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("dns resolution failed: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("dns resolution returned no addresses for %s", host)
	}
	publicIPs := make([]net.IP, 0, len(ips))
	seen := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		if ip == nil || isDisallowedIP(ip) {
			if ip == nil {
				return nil, errors.New("resolved nil ip is not allowed")
			}
			return nil, fmt.Errorf("resolved ip %s is not allowed", ip.String())
		}
		key := ip.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		publicIPs = append(publicIPs, ip)
	}
	if len(publicIPs) == 0 {
		return nil, fmt.Errorf("dns resolution returned no usable addresses for %s", host)
	}
	return publicIPs, nil
}

// SafeDialContext resolves a hostname immediately before dialing and uses only
// the verified IP address for the socket. The caller retains the original host
// for HTTP Host headers and TLS SNI.
func SafeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return SafeDialContextWithLookup(ctx, network, address, defaultLookupIP, (&net.Dialer{Timeout: 10 * time.Second}).DialContext)
}

// SafeDialContextWithLookup is the deterministic/testable form of SafeDialContext.
func SafeDialContextWithLookup(ctx context.Context, network, address string, lookup LookupIPFunc, dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	if dial == nil {
		return nil, errors.New("dialer is nil")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := ResolvePublicIPsWithLookup(ctx, host, lookup)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, ip := range ips {
		conn, dialErr := dial(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no usable addresses for %s", host)
	}
	return nil, lastErr
}

func defaultLookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}

// ValidateResolvedIP 验证 DNS 解析后的全部 IP 是否安全。
// 用于保持原有调用方 API 兼容；实际建连应使用 SafeDialContext。
func ValidateResolvedIP(host string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := ResolvePublicIPs(ctx, host)
	return err
}

func isDisallowedIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 0 || ip4[0] == 10 || ip4[0] == 127 ||
			(ip4[0] == 169 && ip4[1] == 254) ||
			(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) ||
			(ip4[0] == 192 && ip4[1] == 168) ||
			(ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127)
	}
	return len(ip) == net.IPv6len && ip[0]&0xfe == 0xfc
}

func normalizeAllowlist(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	for _, v := range values {
		entry := strings.ToLower(strings.TrimSpace(v))
		if entry == "" {
			continue
		}
		if host, _, err := net.SplitHostPort(entry); err == nil {
			entry = host
		}
		normalized = append(normalized, entry)
	}
	return normalized
}

func isAllowedHost(host string, allowlist []string) bool {
	for _, entry := range allowlist {
		if entry == "" {
			continue
		}
		if strings.HasPrefix(entry, "*.") {
			suffix := strings.TrimPrefix(entry, "*.")
			if host == suffix || strings.HasSuffix(host, "."+suffix) {
				return true
			}
			continue
		}
		if host == entry {
			return true
		}
	}
	return false
}

// IsBlockedHost 报告 host 是否为 localhost、*.localhost，或回环、私网、链路本地、未指定地址的字面量 IP。
// 只判断字面量，域名的解析结果由 ValidateResolvedIP 校验。
func IsBlockedHost(host string) bool {
	return isBlockedHost(strings.ToLower(strings.TrimSpace(host)))
}

func isBlockedHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return isDisallowedIP(ip)
	}
	return false
}
