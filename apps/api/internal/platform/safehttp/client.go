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
	CodeUnsafeBaseURL                        = "unsafe_base_url"
	CodeDNSResolutionFailed                  = "dns_resolution_failed"
	CodeTLSValidationFailed                  = "tls_validation_failed"
	CodeRedirectNotAllowed                   = "redirect_not_allowed"
	CodeUpstreamTimeout                      = "upstream_timeout"
	CodeUpstreamUnavailable                  = "upstream_unavailable"
	CodeResponseTooLarge                     = "upstream_response_too_large"
	CodeCredentialRedirectForbidden          = "credential_redirect_forbidden"
	CodeCredentialTransportRequiresHTTPS     = "credential_transport_requires_https"
	CodeCredentialDestinationForbidden       = "credential_destination_forbidden"
	CodeCredentialTransportDowngradeForbidden = "credential_transport_downgrade_forbidden"
)

// sensitiveHeaderNames is compared with canonical lower-case header keys.
var sensitiveHeaderNames = map[string]struct{}{
	"authorization":       {},
	"proxy-authorization": {},
	"cookie":              {},
	"x-n8n-api-key":       {},
	"x-api-key":           {},
	"api-key":             {},
}

// IsSensitiveHeader reports whether name is a credential-bearing header (case-insensitive).
func IsSensitiveHeader(name string) bool {
	_, ok := sensitiveHeaderNames[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// HeaderHasSensitiveValues reports whether any sensitive header is present.
// Values are never inspected or returned.
func HeaderHasSensitiveValues(header http.Header) bool {
	if header == nil {
		return false
	}
	for name := range header {
		if IsSensitiveHeader(name) && strings.TrimSpace(header.Get(name)) != "" {
			return true
		}
	}
	return false
}

type Error struct {
	Code      string
	Message   string
	Retryable bool
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

type Resolver func(context.Context, string) ([]net.IP, error)
type Dialer func(context.Context, string, string) (net.Conn, error)

// Policy is the single outbound destination policy for integrations.
type Policy struct {
	AllowedSchemes   map[string]bool
	AllowedPorts     map[int]bool
	TrustedHosts     map[string]bool
	// TrustedLocalHTTP maps host -> allowed ports for cleartext HTTP only in
	// development-like environments. Public hosts never appear here.
	TrustedLocalHTTP map[string]map[int]bool
	// CredentialMode disables all redirects and enforces credential transport rules.
	CredentialMode  bool
	Environment     string
	Resolver        Resolver
	DialContext     Dialer
	ConnectTimeout  time.Duration
	ResponseTimeout time.Duration
	TotalTimeout    time.Duration
	MaxRedirects    int
	MaxResponseBytes int64
	TLSConfig       *tls.Config
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

// CredentialPolicy builds the unified policy for any request that may carry
// Authorization, API keys, or connection credentials.
//
// Rules:
//   - never follow redirects
//   - external destinations require HTTPS
//   - cleartext HTTP is limited to explicit local development targets (n8n:5678)
//   - production never grants the local HTTP exception
//   - proxy is always disabled on the resulting client
func CredentialPolicy(environment string) Policy {
	policy := DefaultPolicy()
	policy.CredentialMode = true
	policy.MaxRedirects = 0
	policy.Environment = strings.ToLower(strings.TrimSpace(environment))
	policy.AllowedSchemes = map[string]bool{"https": true}
	policy.AllowedPorts = map[int]bool{443: true}
	if isDevelopmentLike(policy.Environment) {
		// Docker-internal n8n remains reachable in development/test only.
		policy.AllowedSchemes["http"] = true
		policy.AllowedPorts[80] = true
		policy.AllowedPorts[5678] = true
		policy.TrustedHosts = map[string]bool{"n8n": true}
		policy.TrustedLocalHTTP = map[string]map[int]bool{
			"n8n": {5678: true},
		}
	}
	return policy
}

func isDevelopmentLike(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "development", "dev", "test", "testing", "local":
		return true
	default:
		return false
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
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" || host == "localhost" || strings.Contains(host, "%") {
		return nil, destinationForbidden(policy, "The integration URL host is not allowed.")
	}
	port := defaultPort(u.Scheme)
	if u.Port() != "" {
		port, err = strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return nil, safeError(CodeUnsafeBaseURL, "The integration URL port is invalid.", false)
		}
	}
	if err := validateSchemeAndPort(u.Scheme, host, port, policy); err != nil {
		return nil, err
	}
	u.Host = host
	if port != defaultPort(u.Scheme) {
		u.Host = net.JoinHostPort(host, strconv.Itoa(port))
	}
	if u.Path == "" {
		u.Path = "/"
	}
	u.Fragment = ""
	return u, nil
}

func validateSchemeAndPort(scheme, host string, port int, policy Policy) error {
	if scheme == "https" {
		if !policy.AllowedSchemes["https"] {
			return transportRequiresHTTPS(policy)
		}
		if policy.AllowedPorts[port] {
			return nil
		}
		// Development docker n8n may use https://n8n:5678.
		if policy.CredentialMode && isDevelopmentLike(policy.Environment) && policy.TrustedHosts[host] && port == 5678 {
			return nil
		}
		return safeError(CodeUnsafeBaseURL, "The integration URL port is not allowed.", false)
	}
	if scheme != "http" {
		return safeError(CodeUnsafeBaseURL, "The integration URL scheme is not allowed.", false)
	}
	if policy.CredentialMode {
		if !isDevelopmentLike(policy.Environment) {
			return transportRequiresHTTPS(policy)
		}
		ports := policy.TrustedLocalHTTP[host]
		if ports == nil || !ports[port] {
			return transportRequiresHTTPS(policy)
		}
		return nil
	}
	if !policy.AllowedSchemes[scheme] {
		return safeError(CodeUnsafeBaseURL, "The integration URL scheme is not allowed.", false)
	}
	if !policy.AllowedPorts[port] {
		return safeError(CodeUnsafeBaseURL, "The integration URL port is not allowed.", false)
	}
	return nil
}

func transportRequiresHTTPS(policy Policy) error {
	if policy.CredentialMode {
		return safeError(CodeCredentialTransportRequiresHTTPS, "Credentialed integration requests require HTTPS.", false)
	}
	return safeError(CodeUnsafeBaseURL, "The integration URL scheme is not allowed.", false)
}

func destinationForbidden(policy Policy, message string) error {
	if policy.CredentialMode {
		return safeError(CodeCredentialDestinationForbidden, message, false)
	}
	return safeError(CodeUnsafeBaseURL, message, false)
}

// ValidateDestination resolves hostnames and rejects restricted addresses.
// Trusted development hosts may resolve to private docker networks only when
// the host is explicitly listed; a public resolution of those names is still
// accepted for HTTPS, but cleartext HTTP additionally requires private IPs.
func ValidateDestination(ctx context.Context, u *url.URL, policy Policy) ([]net.IP, error) {
	policy = policy.withDefaults()
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	port := defaultPort(u.Scheme)
	if u.Port() != "" {
		if p, err := strconv.Atoi(u.Port()); err == nil {
			port = p
		}
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme == "" {
		scheme = "https"
	}

	var ips []net.IP
	if literal, err := netip.ParseAddr(host); err == nil {
		literal = literal.Unmap()
		if restricted(literal) && !policy.TrustedHosts[host] {
			return nil, destinationForbidden(policy, "The integration URL resolves to a restricted address.")
		}
		ips = []net.IP{net.ParseIP(literal.String())}
	} else {
		resolved, err := policy.Resolver(ctx, host)
		if err != nil || len(resolved) == 0 {
			return nil, safeError(CodeDNSResolutionFailed, "The integration host could not be resolved.", true)
		}
		for _, ip := range resolved {
			addr, ok := netip.AddrFromSlice(ip)
			if !ok {
				return nil, safeError(CodeDNSResolutionFailed, "The integration host could not be resolved.", true)
			}
			addr = addr.Unmap()
			if restricted(addr) && !policy.TrustedHosts[host] {
				return nil, destinationForbidden(policy, "The integration host resolves to a restricted address.")
			}
			ips = append(ips, ip)
		}
	}

	if scheme == "http" && policy.CredentialMode {
		if err := validateSchemeAndPort(scheme, host, port, policy); err != nil {
			return nil, err
		}
		// HTTP exceptions require every resolved address to be private/loopback
		// so a public DNS answer for "n8n" cannot receive cleartext credentials.
		for _, ip := range ips {
			addr, ok := netip.AddrFromSlice(ip)
			if !ok {
				return nil, destinationForbidden(policy, "The integration host resolves to a restricted address.")
			}
			addr = addr.Unmap()
			if !addr.IsPrivate() && !addr.IsLoopback() {
				return nil, destinationForbidden(policy, "Cleartext credential destinations must resolve to a local network address.")
			}
		}
	}
	return ips, nil
}

// ValidateCredentialDestination is the shared entry point for URL + DNS checks
// on credential-bearing outbound calls.
func ValidateCredentialDestination(ctx context.Context, raw string, policy Policy) (*url.URL, []net.IP, error) {
	if !policy.CredentialMode {
		policy.CredentialMode = true
	}
	normalized, err := NormalizeURL(raw, policy)
	if err != nil {
		return nil, nil, err
	}
	ips, err := ValidateDestination(ctx, normalized, policy)
	if err != nil {
		return nil, nil, err
	}
	return normalized, ips, nil
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
	baseTransport := &http.Transport{
		Proxy:                 nil,
		TLSClientConfig:       policy.TLSConfig.Clone(),
		TLSHandshakeTimeout:   policy.ConnectTimeout,
		ResponseHeaderTimeout: policy.ResponseTimeout,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, safeError(CodeUnsafeBaseURL, "The integration destination is invalid.", false)
			}
			// Scheme is re-validated in RoundTrip; Dial rechecks every resolved
			// address immediately before connect to defeat DNS rebinding.
			checkURL := &url.URL{Scheme: "https", Host: host}
			ips, err := ValidateDestination(ctx, checkURL, policy)
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
	transport := &guardedTransport{base: baseTransport, policy: policy}
	client := &Client{policy: policy}
	client.httpClient = &http.Client{Transport: transport, Timeout: policy.TotalTimeout}
	client.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if policy.CredentialMode || policy.MaxRedirects <= 0 {
			// Never follow. Callers receive the 3xx via ErrUseLastResponse semantics
			// and guardedTransport converts it to a controlled error.
			return http.ErrUseLastResponse
		}
		if len(via) > policy.MaxRedirects {
			return safeError(CodeRedirectNotAllowed, "The integration redirected too many times.", false)
		}
		if HeaderHasSensitiveValues(req.Header) {
			return safeError(CodeCredentialRedirectForbidden, "Credentialed requests may not follow redirects.", false)
		}
		// Reject scheme downgrades even for non-credential clients.
		if len(via) > 0 && via[0].URL != nil && strings.EqualFold(via[0].URL.Scheme, "https") && strings.EqualFold(req.URL.Scheme, "http") {
			return safeError(CodeCredentialTransportDowngradeForbidden, "HTTPS to HTTP redirects are not allowed.", false)
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

// guardedTransport enforces destination policy before dial and converts
// credential-mode redirect responses into controlled errors without a second hop.
type guardedTransport struct {
	base   http.RoundTripper
	policy Policy
}

func (t *guardedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, safeError(CodeUnsafeBaseURL, "The integration destination is invalid.", false)
	}
	normalized, err := NormalizeURL(req.URL.String(), t.policy)
	if err != nil {
		return nil, err
	}
	if _, err = ValidateDestination(req.Context(), normalized, t.policy); err != nil {
		return nil, err
	}
	// Clone so we never mutate the caller's URL while still applying normalization.
	cloned := req.Clone(req.Context())
	cloned.URL = normalized
	// Strip Userinfo again as defense in depth (NormalizeURL already rejects it).
	cloned.URL.User = nil
	cloned.URL.Fragment = ""

	response, err := t.base.RoundTrip(cloned)
	if err != nil {
		return nil, classify(err)
	}
	if t.policy.CredentialMode && isRedirectStatus(response.StatusCode) {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
		_ = response.Body.Close()
		// Never surface Location or response headers that may carry secrets.
		return nil, safeError(CodeCredentialRedirectForbidden, "Credentialed requests may not follow redirects.", false)
	}
	return response, nil
}

func isRedirectStatus(code int) bool {
	switch code {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	normalized, err := NormalizeURL(req.URL.String(), c.policy)
	if err != nil {
		return nil, err
	}
	if _, err = ValidateDestination(req.Context(), normalized, c.policy); err != nil {
		return nil, err
	}
	req = req.Clone(req.Context())
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
	if errors.Is(err, http.ErrUseLastResponse) {
		return safeError(CodeCredentialRedirectForbidden, "Credentialed requests may not follow redirects.", false)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return safeError(CodeUpstreamTimeout, "The integration request timed out.", true)
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if errors.As(urlErr.Err, &safe) {
			return safe
		}
		if errors.Is(urlErr.Err, http.ErrUseLastResponse) {
			return safeError(CodeCredentialRedirectForbidden, "Credentialed requests may not follow redirects.", false)
		}
		err = urlErr.Err
	}
	var record tls.RecordHeaderError
	if errors.As(err, &record) || strings.Contains(strings.ToLower(err.Error()), "certificate") || strings.Contains(strings.ToLower(err.Error()), "tls:") {
		return safeError(CodeTLSValidationFailed, "The integration TLS certificate could not be validated.", false)
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
