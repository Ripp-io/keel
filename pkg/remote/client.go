package remote

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// HardenedClient returns an *http.Client with security protections:
//   - 120-second timeout
//   - Redirect validation (HTTPS enforcement, SSRF checks)
//   - DNS rebinding mitigation via a custom DialContext that validates
//     resolved IP addresses at connect time
//
// This is the single source of truth for secure HTTP clients in Keel.
// Both the remote layer fetcher and the deps downloader use it.
func HardenedClient() *http.Client {
	return &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			DialContext: ssrfSafeDialContext,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if err := validateRedirect(req.URL.String()); err != nil {
				return err
			}
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}

// ssrfSafeDialContext wraps the default dialer and validates that resolved
// IP addresses are not loopback, private, or link-local. This mitigates
// DNS rebinding attacks where a hostname resolves to a public IP at URL
// parse time but to a private/loopback IP when the actual TCP connection
// is established.
//
// The test override skipSSRFValidation is respected so that tests using
// httptest.NewServer (which binds to 127.0.0.1) can still function.
const defaultDialTimeout = 30 * time.Second

func ssrfSafeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	timeout := defaultDialTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
	}

	if skipSSRFValidation.Load() {
		return dialer.DialContext(ctx, network, addr)
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q: %w", addr, err)
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("DNS lookup for %q failed: %w", host, err)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses found for %q", host)
	}

	// Validate every resolved IP. If any address is private/loopback/link-local,
	// reject the connection. An attacker controlling DNS could return a mix of
	// public and private IPs; we reject if any resolved address is dangerous.
	for _, ipAddr := range ips {
		if err := validateIP(ipAddr.IP); err != nil {
			return nil, fmt.Errorf("DNS rebinding protection: %s resolved to %s: %w", host, ipAddr.IP, err)
		}
	}

	// Connect using the first resolved IP directly to prevent a TOCTOU race
	// where a second DNS lookup could return a different (malicious) address.
	resolvedAddr := net.JoinHostPort(ips[0].IP.String(), port)
	return dialer.DialContext(ctx, network, resolvedAddr)
}

// validateIP checks a single IP address against SSRF blocklists.
func validateIP(ip net.IP) error {
	if ip.IsUnspecified() {
		return fmt.Errorf("connection to unspecified address rejected: %s", ip)
	}
	if ip.IsLoopback() {
		return fmt.Errorf("connection to loopback address rejected: %s", ip)
	}
	if ip.IsPrivate() {
		return fmt.Errorf("connection to private address rejected: %s", ip)
	}
	if ip.IsLinkLocalUnicast() {
		return fmt.Errorf("connection to link-local address rejected: %s", ip)
	}
	if ip.IsLinkLocalMulticast() {
		return fmt.Errorf("connection to link-local multicast address rejected: %s", ip)
	}
	return nil
}

// ValidateURL performs pre-request SSRF validation on a URL. It checks
// that the host (if an IP literal) is not loopback, private, or link-local.
// DNS-name hosts are validated at connect time by the HardenedClient's
// DialContext.
//
// Use this before making requests to reject obviously bad URLs early,
// before any network call. The HardenedClient's DialContext provides the
// second layer of defense against DNS rebinding.
func ValidateURL(rawURL string) error {
	return validateHost(rawURL)
}
