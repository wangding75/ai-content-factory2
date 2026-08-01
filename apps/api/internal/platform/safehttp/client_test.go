package safehttp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNormalizeURLPolicy(t *testing.T) {
	policy := DefaultPolicy()
	for _, raw := range []string{
		"http://api.example.test/v1", "https://user:pass@api.example.test/v1",
		"https://localhost/v1", "https://api.example.test:444/v1", " https://api.example.test/v1",
		"https://api.example.test/v1#fragment", "https://[fe80::1%25eth0]/v1",
	} {
		if _, err := NormalizeURL(raw, policy); ErrorCode(err) != CodeUnsafeBaseURL {
			t.Errorf("NormalizeURL(%q) error=%v", raw, err)
		}
	}
	u, err := NormalizeURL("https://API.Example.Test/v1?b=2", policy)
	if err != nil || u.String() != "https://api.example.test/v1?b=2" {
		t.Fatalf("normalized=%v err=%v", u, err)
	}
}

func TestValidateDestinationRejectsEveryRestrictedAddress(t *testing.T) {
	for _, rawIP := range []string{
		"0.0.0.0", "127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.0.1",
		"169.254.169.254", "224.0.0.1", "::", "::1", "fc00::1", "fe80::1", "ff02::1", "::ffff:127.0.0.1",
	} {
		t.Run(rawIP, func(t *testing.T) {
			policy := DefaultPolicy()
			policy.Resolver = func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP(rawIP)}, nil }
			u, _ := NormalizeURL("https://api.example.test/v1", policy)
			if _, err := ValidateDestination(context.Background(), u, policy); ErrorCode(err) != CodeUnsafeBaseURL {
				t.Fatalf("address %s accepted: %v", rawIP, err)
			}
		})
	}
	policy := DefaultPolicy()
	policy.Resolver = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.10"), net.ParseIP("10.0.0.2")}, nil
	}
	u, _ := NormalizeURL("https://mixed.example.test", policy)
	if _, err := ValidateDestination(context.Background(), u, policy); ErrorCode(err) != CodeUnsafeBaseURL {
		t.Fatalf("mixed DNS answer accepted: %v", err)
	}
}

func TestClientRevalidatesRedirectDestination(t *testing.T) {
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
	client := New(policy)
	req, _ := http.NewRequest(http.MethodGet, "http://public.example.test/start", nil)
	if _, err := client.Do(req); ErrorCode(err) != CodeRedirectNotAllowed {
		t.Fatalf("redirect error=%v", err)
	}
}

func TestClientTLSFailureTimeoutAndBodyLimitAreSafe(t *testing.T) {
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer tlsServer.Close()
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(tlsServer.URL, "https://"))
	policy := DefaultPolicy()
	policy.AllowedPorts = map[int]bool{mustPort(t, port): true}
	policy.TrustedHosts = map[string]bool{host: true}
	policy.Resolver = func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP(host)}, nil }
	req, _ := http.NewRequest(http.MethodGet, tlsServer.URL, nil)
	if _, err := New(policy).Do(req); ErrorCode(err) != CodeTLSValidationFailed {
		t.Fatalf("TLS error=%v", err)
	}

	timeoutPolicy := DefaultPolicy()
	timeoutPolicy.TotalTimeout = 10 * time.Millisecond
	timeoutPolicy.Resolver = func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("203.0.113.10")}, nil }
	timeoutPolicy.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	timeoutReq, _ := http.NewRequest(http.MethodGet, "https://timeout.example.test", nil)
	if _, err := New(timeoutPolicy).Do(timeoutReq); ErrorCode(err) != CodeUpstreamTimeout {
		t.Fatalf("timeout error=%v", err)
	}

	limitPolicy := DefaultPolicy()
	limitPolicy.MaxResponseBytes = 4
	response := &http.Response{Body: io.NopCloser(bytes.NewBufferString("12345"))}
	if _, err := New(limitPolicy).ReadBody(response); ErrorCode(err) != CodeResponseTooLarge {
		t.Fatalf("body limit error=%v", err)
	}
}

func mustPort(t *testing.T, value string) int {
	t.Helper()
	var port int
	if _, err := fmt.Sscanf(value, "%d", &port); err != nil {
		t.Fatal(err)
	}
	return port
}
