// Package safehttp provides the single outbound HTTP security boundary used by
// integrations. It validates the destination again at every network hop and
// returns only stable, non-sensitive errors.
package safehttp

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	CodeUnsafeBaseURL       = "unsafe_base_url"
	CodeDNSResolutionFailed = "dns_resolution_failed"
	CodeTLSValidationFailed = "tls_validation_failed"
	CodeRedirectNotAllowed  = "redirect_not_allowed"
	CodeUpstreamTimeout     = "upstream_timeout"
	CodeUpstreamUnavailable = "upstream_unavailable"
	CodeResponseTooLarge    = "upstream_response_too_large"
)

type Error struct {
	Code      string
	Message   string
	Retryable bool
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

type Resolver func(context.Context, string) ([]net.IP, error)
type Dialer func(context.Context, string, string) (net.Conn, error)

type Policy struct {
	AllowedSchemes   map[string]bool
	AllowedPorts     map[int]bool
	TrustedHosts     map[string]bool
	Resolver         Resolver
	DialContext      Dialer
	ConnectTimeout   time.Duration
	ResponseTimeout  time.Duration
	TotalTimeout     time.Duration
	MaxRedirects     int
	MaxResponseBytes int64
	TLSConfig        *tls.Config
}

func DefaultPolicy() Policy {
	return Policy{
		AllowedSchemes:   map[string]bool{"https": true},
		AllowedPorts:     map[int]bool{443: true},
		ConnectTimeout:   5 * time.Second,
		ResponseTimeout:  15 * time.Second,
		TotalTimeout:     30 * time.Second,
		MaxRedirects:     3,
		MaxResponseBytes: 1 << 20,
	}
}

func (p Policy) withDefaults() Policy {
	d := DefaultPolicy()
	if len(p.AllowedSchemes) == 0 {
		p.AllowedSchemes = d.AllowedSchemes
	}
	if len(p.AllowedPorts) == 0 {
		p.AllowedPorts = d.AllowedPorts
	}
	if p.ConnectTimeout <= 0 {
		p.ConnectTimeout = d.ConnectTimeout
	}
	if p.ResponseTimeout <= 0 {
		p.ResponseTimeout = d.ResponseTimeout
	}
	if p.TotalTimeout <= 0 {
		p.TotalTimeout = d.TotalTimeout
	}
	if p.MaxResponseBytes <= 0 {
		p.MaxResponseBytes = d.MaxResponseBytes
	}
	if p.Resolver == nil {
		p.Resolver = func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		}
	}
	if p.DialContext == nil {
		p.DialContext = (&net.Dialer{Timeout: p.ConnectTimeout}).DialContext
	}
	if p.TLSConfig == nil {
		p.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return p
}

func NormalizeURL(raw string, policy Policy) (*url.URL, error) {
	policy = policy.withDefaults()
	if strings.TrimSpace(raw) != raw || raw == "" || strings.ContainsAny(raw, "\r\n\t#") {
		return nil, safeError(CodeUnsafeBaseURL, "The integration URL is not allowed.", false)
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || !u.IsAbs() || u.Opaque != "" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return nil, safeError(CodeUnsafeBaseURL, "The integration URL is not allowed.", false)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if !policy.AllowedSchemes[u.Scheme] {
		return nil, safeError(CodeUnsafeBaseURL, "The integration URL scheme is not allowed.", false)
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" || host == "localhost" || strings.Contains(host, "%") {
		return nil, safeError(CodeUnsafeBaseURL, "The integration URL host is not allowed.", false)
	}
	port := defaultPort(u.Scheme)
	if u.Port() != "" {
		port, err = strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return nil, safeError(CodeUnsafeBaseURL, "The integration URL port is invalid.", false)
		}
	}
	if !policy.AllowedPorts[port] {
		return nil, safeError(CodeUnsafeBaseURL, "The integration URL port is not allowed.", false)
	}
	u.Host = host
	if port != defaultPort(u.Scheme) {
		u.Host = net.JoinHostPort(host, strconv.Itoa(port))
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u, nil
}

func ValidateDestination(ctx context.Context, u *url.URL, policy Policy) ([]net.IP, error) {
	policy = policy.withDefaults()
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if policy.TrustedHosts[host] {
		return policy.Resolver(ctx, host)
	}
	if literal, err := netip.ParseAddr(host); err == nil {
		literal = literal.Unmap()
		if restricted(literal) {
			return nil, safeError(CodeUnsafeBaseURL, "The integration URL resolves to a restricted address.", false)
		}
		return []net.IP{net.ParseIP(literal.String())}, nil
	}
	ips, err := policy.Resolver(ctx, host)
	if err != nil || len(ips) == 0 {
		return nil, safeError(CodeDNSResolutionFailed, "The integration host could not be resolved.", true)
	}
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok || restricted(addr.Unmap()) {
			return nil, safeError(CodeUnsafeBaseURL, "The integration host resolves to a restricted address.", false)
		}
	}
	return ips, nil
}

func restricted(addr netip.Addr) bool {
	return !addr.IsValid() || addr.IsUnspecified() || addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast()
}

type Client struct {
	httpClient *http.Client
	policy     Policy
}

// HTTPClient exposes the guarded standard client for adapters whose interface
// is fixed to *http.Client. Destination checks remain installed in the
// transport and redirect hook.
func (c *Client) HTTPClient() *http.Client { return c.httpClient }

func New(policy Policy) *Client {
	policy = policy.withDefaults()
	transport := &http.Transport{
		Proxy:                 nil,
		TLSClientConfig:       policy.TLSConfig.Clone(),
		TLSHandshakeTimeout:   policy.ConnectTimeout,
		ResponseHeaderTimeout: policy.ResponseTimeout,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, safeError(CodeUnsafeBaseURL, "The integration destination is invalid.", false)
			}
			u := &url.URL{Scheme: "https", Host: net.JoinHostPort(host, port)}
			ips, err := ValidateDestination(ctx, u, policy)
			if err != nil {
				return nil, err
			}
			var last error
			for _, ip := range ips {
				conn, dialErr := policy.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if dialErr == nil {
					return conn, nil
				}
				last = dialErr
			}
			if last != nil {
				return nil, safeError(CodeUpstreamUnavailable, "The integration service is unavailable.", true)
			}
			return nil, safeError(CodeUpstreamUnavailable, "The integration service is unavailable.", true)
		},
	}
	client := &Client{policy: policy}
	client.httpClient = &http.Client{Transport: transport, Timeout: policy.TotalTimeout}
	client.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > policy.MaxRedirects {
			return safeError(CodeRedirectNotAllowed, "The integration redirected too many times.", false)
		}
		normalized, err := NormalizeURL(req.URL.String(), policy)
		if err != nil {
			return safeError(CodeRedirectNotAllowed, "The integration redirect is not allowed.", false)
		}
		if _, err = ValidateDestination(req.Context(), normalized, policy); err != nil {
			return safeError(CodeRedirectNotAllowed, "The integration redirect is not allowed.", false)
		}
		req.URL = normalized
		return nil
	}
	return client
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	normalized, err := NormalizeURL(req.URL.String(), c.policy)
	if err != nil {
		return nil, err
	}
	if _, err = ValidateDestination(req.Context(), normalized, c.policy); err != nil {
		return nil, err
	}
	req.URL = normalized
	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, classify(err)
	}
	return response, nil
}

func (c *Client) ReadBody(response *http.Response) ([]byte, error) {
	defer response.Body.Close()
	reader := io.LimitReader(response.Body, c.policy.MaxResponseBytes+1)
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, safeError(CodeUpstreamUnavailable, "The integration response could not be read.", true)
	}
	if int64(len(body)) > c.policy.MaxResponseBytes {
		return nil, safeError(CodeResponseTooLarge, "The integration response is too large.", false)
	}
	return body, nil
}

func classify(err error) error {
	var safe *Error
	if errors.As(err, &safe) {
		return safe
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return safeError(CodeUpstreamTimeout, "The integration request timed out.", true)
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		var record tls.RecordHeaderError
		if errors.As(urlErr.Err, &record) || strings.Contains(strings.ToLower(urlErr.Err.Error()), "certificate") {
			return safeError(CodeTLSValidationFailed, "The integration TLS certificate could not be validated.", false)
		}
	}
	return safeError(CodeUpstreamUnavailable, "The integration service is unavailable.", true)
}

func safeError(code, message string, retryable bool) error {
	return &Error{Code: code, Message: message, Retryable: retryable}
}
func defaultPort(scheme string) int {
	if scheme == "https" {
		return 443
	}
	return 80
}
func ErrorCode(err error) string {
	var safe *Error
	if errors.As(err, &safe) {
		return safe.Code
	}
	return CodeUpstreamUnavailable
}
