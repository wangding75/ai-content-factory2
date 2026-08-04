package safehttp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCredentialPolicySchemeBounds(t *testing.T) {
	dev := CredentialPolicy("development")
	prod := CredentialPolicy("production")

	if _, err := NormalizeURL("https://api.example.test/v1", prod); err != nil {
		t.Fatalf("external https rejected in production: %v", err)
	}
	if _, err := NormalizeURL("http://api.example.test/v1", prod); ErrorCode(err) != CodeCredentialTransportRequiresHTTPS {
		t.Fatalf("external http production err=%v", err)
	}
	if _, err := NormalizeURL("http://n8n:5678", prod); ErrorCode(err) != CodeCredentialTransportRequiresHTTPS {
		t.Fatalf("n8n http production err=%v", err)
	}
	if _, err := NormalizeURL("http://n8n:5678", dev); err != nil {
		t.Fatalf("n8n http development rejected: %v", err)
	}
	if _, err := NormalizeURL("http://n8n:80", dev); ErrorCode(err) != CodeCredentialTransportRequiresHTTPS {
		t.Fatalf("n8n wrong port err=%v", err)
	}
	if _, err := NormalizeURL("http://evil.example.test:5678", dev); ErrorCode(err) != CodeCredentialTransportRequiresHTTPS {
		t.Fatalf("non-trusted http err=%v", err)
	}
	if _, err := NormalizeURL("https://user:pass@api.example.test/v1", prod); ErrorCode(err) != CodeUnsafeBaseURL {
		t.Fatalf("userinfo err=%v", err)
	}
	if _, err := NormalizeURL("https://localhost/v1", prod); err == nil {
		t.Fatal("localhost must remain rejected")
	}
}

func TestCredentialHTTPRequiresPrivateResolution(t *testing.T) {
	policy := CredentialPolicy("development")
	policy.Resolver = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.50")}, nil
	}
	u, err := NormalizeURL("http://n8n:5678/api", policy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateDestination(context.Background(), u, policy); ErrorCode(err) != CodeCredentialDestinationForbidden {
		t.Fatalf("public DNS for cleartext n8n accepted: %v", err)
	}
	policy.Resolver = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("172.20.0.3")}, nil
	}
	if _, err = ValidateDestination(context.Background(), u, policy); err != nil {
		t.Fatalf("private docker n8n rejected: %v", err)
	}
}

func TestCredentialRedirectNeverFollows(t *testing.T) {
	statuses := []int{
		http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusSeeOther,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect,
	}
	for _, status := range statuses {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			var secondHop atomic.Int32
			var secondSensitive atomic.Int32
			final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				secondHop.Add(1)
				for name := range r.Header {
					if IsSensitiveHeader(name) && r.Header.Get(name) != "" {
						secondSensitive.Add(1)
					}
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer final.Close()

			first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, final.URL+"/second", status)
			}))
			defer first.Close()

			host, port, _ := net.SplitHostPort(strings.TrimPrefix(first.URL, "http://"))
			policy := CredentialPolicy("development")
			// Test servers bind loopback; treat host as trusted only for dial via custom resolver+trusted.
			// Use explicit policy that allows the test host over HTTP on its port for redirect proof.
			policy.AllowedSchemes = map[string]bool{"http": true, "https": true}
			policy.AllowedPorts = map[int]bool{mustPort(t, port): true, 80: true, 443: true, 5678: true}
			policy.TrustedHosts = map[string]bool{host: true}
			policy.TrustedLocalHTTP = map[string]map[int]bool{host: {mustPort(t, port): true}}
			policy.Resolver = func(context.Context, string) ([]net.IP, error) {
				return []net.IP{net.ParseIP(host)}, nil
			}

			client := New(policy)
			req, _ := http.NewRequest(http.MethodGet, first.URL+"/start", nil)
			req.Header.Set("Authorization", "Bearer test-token-not-real")
			req.Header.Set("X-N8N-API-KEY", "n8n-test-key-not-real")
			req.Header.Set("X-API-Key", "provider-test-key-not-real")
			_, err := client.Do(req)
			if ErrorCode(err) != CodeCredentialRedirectForbidden {
				t.Fatalf("status=%d err=%v", status, err)
			}
			if secondHop.Load() != 0 {
				t.Fatalf("second hop visited %d times", secondHop.Load())
			}
			if secondSensitive.Load() != 0 {
				t.Fatalf("sensitive headers reached second hop: %d", secondSensitive.Load())
			}
			if err != nil && (strings.Contains(err.Error(), "Bearer") || strings.Contains(err.Error(), "test-token") || strings.Contains(err.Error(), "n8n-test-key")) {
				t.Fatalf("error leaked credential material: %v", err)
			}
		})
	}
}

func TestCredentialRedirectCrossHostPortAndDowngrade(t *testing.T) {
	cases := []struct {
		name     string
		location string
	}{
		{"cross_host", "http://other.example.test/x"},
		{"same_host_diff_port", "http://127.0.0.1:9/x"},
		{"https_to_http", "http://api.example.test/x"},
		{"http_to_https", "https://api.example.test/x"},
		{"to_private", "http://10.0.0.8/x"},
		{"to_loopback", "http://127.0.0.1/x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var hops atomic.Int32
			policy := CredentialPolicy("development")
			policy.AllowedSchemes = map[string]bool{"http": true, "https": true}
			policy.AllowedPorts = map[int]bool{80: true, 443: true, 5678: true}
			policy.TrustedHosts = map[string]bool{"public.example.test": true}
			policy.TrustedLocalHTTP = map[string]map[int]bool{"public.example.test": {80: true}}
			policy.Resolver = func(_ context.Context, host string) ([]net.IP, error) {
				// Cleartext first hop must resolve private under credential HTTP rules.
				if host == "public.example.test" {
					return []net.IP{net.ParseIP("172.20.0.10")}, nil
				}
				if host == "other.example.test" || host == "api.example.test" {
					return []net.IP{net.ParseIP("203.0.113.21")}, nil
				}
				return []net.IP{net.ParseIP("10.0.0.8")}, nil
			}
			policy.DialContext = func(_ context.Context, _, address string) (net.Conn, error) {
				hops.Add(1)
				client, server := net.Pipe()
				go func() {
					defer server.Close()
					req, _ := http.ReadRequest(bufio.NewReader(server))
					if req != nil {
						_ = req.Body.Close()
					}
					_, _ = io.WriteString(server, "HTTP/1.1 302 Found\r\nLocation: "+tc.location+"\r\nContent-Length: 0\r\n\r\n")
				}()
				return client, nil
			}
			client := New(policy)
			req, _ := http.NewRequest(http.MethodGet, "http://public.example.test/start", nil)
			req.Header.Set("Authorization", "Bearer redirect-probe")
			_, err := client.Do(req)
			if ErrorCode(err) != CodeCredentialRedirectForbidden {
				t.Fatalf("err=%v", err)
			}
			if hops.Load() != 1 {
				t.Fatalf("expected single hop dial, got %d", hops.Load())
			}
		})
	}
}

func TestCredentialRedirectChainStillSingleHop(t *testing.T) {
	var dials atomic.Int32
	policy := CredentialPolicy("development")
	policy.AllowedSchemes["http"] = true
	policy.AllowedPorts[80] = true
	policy.TrustedHosts = map[string]bool{"chain.example.test": true}
	policy.TrustedLocalHTTP = map[string]map[int]bool{"chain.example.test": {80: true}}
	policy.Resolver = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("172.20.0.9")}, nil
	}
	policy.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
		dials.Add(1)
		client, server := net.Pipe()
		go func() {
			defer server.Close()
			req, _ := http.ReadRequest(bufio.NewReader(server))
			if req != nil {
				_ = req.Body.Close()
			}
			_, _ = io.WriteString(server, "HTTP/1.1 301 Moved\r\nLocation: http://chain.example.test/b\r\nContent-Length: 0\r\n\r\n")
		}()
		return client, nil
	}
	req, _ := http.NewRequest(http.MethodGet, "http://chain.example.test/a", nil)
	req.Header.Set("X-N8N-API-KEY", "chain-key")
	if _, err := New(policy).Do(req); ErrorCode(err) != CodeCredentialRedirectForbidden {
		t.Fatalf("err=%v", err)
	}
	if dials.Load() != 1 {
		t.Fatalf("redirect chain dialed %d times", dials.Load())
	}
}

func TestSensitiveHeaderDetectionCaseInsensitive(t *testing.T) {
	for _, name := range []string{"Authorization", "authorization", "AUTHORIZATION", "X-N8N-API-KEY", "x-api-key", "Api-Key", "Cookie", "Proxy-Authorization"} {
		if !IsSensitiveHeader(name) {
			t.Fatalf("%s should be sensitive", name)
		}
	}
	h := make(http.Header)
	h.Set("X-N8N-API-KEY", "a")
	h.Set("Authorization", "Bearer b")
	if !HeaderHasSensitiveValues(h) {
		t.Fatal("expected sensitive values")
	}
}

func TestCredentialDNSRebindingAndProxyDisabled(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://proxy.example.test:8080")
	t.Setenv("HTTPS_PROXY", "http://proxy.example.test:8080")
	t.Setenv("ALL_PROXY", "http://proxy.example.test:8080")

	var seen atomic.Int32
	policy := CredentialPolicy("production")
	calls := 0
	policy.Resolver = func(context.Context, string) ([]net.IP, error) {
		calls++
		if calls == 1 {
			return []net.IP{net.ParseIP("203.0.113.30")}, nil
		}
		// Dial-time rebinding to private.
		return []net.IP{net.ParseIP("10.0.0.9")}, nil
	}
	policy.DialContext = func(context.Context, string, string) (net.Conn, error) {
		seen.Add(1)
		return nil, fmt.Errorf("should not dial after rebind reject")
	}
	req, _ := http.NewRequest(http.MethodGet, "https://rebind.example.test/v1", nil)
	req.Header.Set("Authorization", "Bearer rebind-token")
	_, err := New(policy).Do(req)
	if ErrorCode(err) != CodeCredentialDestinationForbidden && ErrorCode(err) != CodeUnsafeBaseURL {
		t.Fatalf("rebinding err=%v code=%s", err, ErrorCode(err))
	}
	if seen.Load() != 0 {
		t.Fatal("dial must not proceed after rebind")
	}

	// Mixed multi-address answer rejects entirely.
	policy.Resolver = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.40"), net.ParseIP("10.0.0.10")}, nil
	}
	req, _ = http.NewRequest(http.MethodGet, "https://mixed.example.test/v1", nil)
	if _, err = New(policy).Do(req); ErrorCode(err) != CodeCredentialDestinationForbidden && ErrorCode(err) != CodeUnsafeBaseURL {
		t.Fatalf("mixed DNS err=%v", err)
	}
}

func TestCredentialIPv6LoopbackRejected(t *testing.T) {
	policy := CredentialPolicy("development")
	policy.Resolver = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("::1")}, nil
	}
	u, _ := NormalizeURL("https://loop.example.test/v1", policy)
	if _, err := ValidateDestination(context.Background(), u, policy); err == nil {
		t.Fatal("IPv6 loopback accepted")
	}
}

func TestNonCredentialClientStillRevalidatesRedirect(t *testing.T) {
	// Preserve frozen non-credential redirect validation behavior.
	policy := DefaultPolicy()
	policy.AllowedSchemes = map[string]bool{"http": true}
	policy.AllowedPorts = map[int]bool{80: true}
	policy.Resolver = func(_ context.Context, host string) ([]net.IP, error) {
		if host == "private.example.test" {
			return []net.IP{net.ParseIP("10.0.0.8")}, nil
		}
		return []net.IP{net.ParseIP("203.0.113.10")}, nil
	}
	policy.DialContext = func(context.Context, string, string) (net.Conn, error) {
		client, server := net.Pipe()
		go func() {
			defer server.Close()
			request, _ := http.ReadRequest(bufio.NewReader(server))
			if request != nil {
				_ = request.Body.Close()
			}
			_, _ = io.WriteString(server, "HTTP/1.1 302 Found\r\nLocation: http://private.example.test/secret\r\nContent-Length: 0\r\n\r\n")
		}()
		return client, nil
	}
	req, _ := http.NewRequest(http.MethodGet, "http://public.example.test/start", nil)
	if _, err := New(policy).Do(req); ErrorCode(err) != CodeRedirectNotAllowed {
		t.Fatalf("redirect error=%v", err)
	}
}
