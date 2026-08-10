package safehttp

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
)

func fixedProxyFunc(raw string, bypass func(*http.Request) bool) func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		if bypass != nil && bypass(req) {
			return nil, nil
		}
		return url.Parse(raw)
	}
}

func directProxyFunc(*http.Request) (*url.URL, error) { return nil, nil }

func directPolicyForTest(policy Policy) Policy {
	policy.proxyFunc = directProxyFunc
	return policy
}

func TestControlledProxyRoutesHTTPAndHonorsDirectSelection(t *testing.T) {
	var proxyRequests atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyRequests.Add(1)
		if r.URL.String() != "http://external.example.test/v1" {
			t.Errorf("proxy request URL=%q", r.URL.String())
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer proxy.Close()

	proxyPolicy := DefaultPolicy()
	proxyPolicy.AllowedSchemes["http"] = true
	proxyPolicy.AllowedPorts[80] = true
	proxyPolicy.Resolver = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.10")}, nil
	}
	proxyPolicy.proxyFunc = fixedProxyFunc(proxy.URL, nil)
	var directDials atomic.Int32
	proxyPolicy.DialContext = func(context.Context, string, string) (net.Conn, error) {
		directDials.Add(1)
		return nil, fmt.Errorf("target dial must not be used through proxy")
	}
	request, err := http.NewRequest(http.MethodGet, "http://external.example.test/v1", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := New(proxyPolicy).Do(request)
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	_ = response.Body.Close()
	if proxyRequests.Load() != 1 || directDials.Load() != 0 {
		t.Fatalf("proxyRequests=%d directDials=%d", proxyRequests.Load(), directDials.Load())
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"direct":true}`)
	}))
	defer upstream.Close()
	host, port, err := net.SplitHostPort(strings.TrimPrefix(upstream.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	portNumber := mustPort(t, port)
	directPolicy := DefaultPolicy()
	directPolicy.AllowedSchemes["http"] = true
	directPolicy.AllowedPorts[portNumber] = true
	directPolicy.TrustedHosts = map[string]bool{"direct.example.test": true}
	directPolicy.Resolver = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP(host)}, nil
	}
	directPolicy.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(host, port))
	}
	var proxyDials atomic.Int32
	directPolicy.ProxyDialContext = func(context.Context, string, string) (net.Conn, error) {
		proxyDials.Add(1)
		return nil, fmt.Errorf("proxy dial must not be used for direct request")
	}
	directPolicy.proxyFunc = fixedProxyFunc(proxy.URL, func(*http.Request) bool { return true })
	directRequest, err := http.NewRequest(http.MethodGet, "http://direct.example.test:"+port+"/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}
	directResponse, err := New(directPolicy).Do(directRequest)
	if err != nil {
		t.Fatalf("direct request failed: %v", err)
	}
	_ = directResponse.Body.Close()
	if proxyDials.Load() != 0 {
		t.Fatalf("direct request used proxy: %d dials", proxyDials.Load())
	}
}

func TestControlledProxyRoutesHTTPSAndPreservesDestinationValidation(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"secure":true}`)
	}))
	defer upstream.Close()
	upstreamHost, upstreamPort, err := net.SplitHostPort(upstream.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	var connectRequests atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
			return
		}
		connectRequests.Add(1)
		clientConn, buffered, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer clientConn.Close()
		_, _ = io.WriteString(buffered, "HTTP/1.1 200 Connection Established\r\n\r\n")
		_ = buffered.Flush()
		upstreamConn, err := net.Dial("tcp", net.JoinHostPort(upstreamHost, upstreamPort))
		if err != nil {
			return
		}
		defer upstreamConn.Close()
		go func() { _, _ = io.Copy(upstreamConn, clientConn) }()
		_, _ = io.Copy(clientConn, upstreamConn)
	}))
	defer proxy.Close()

	policy := CredentialPolicy("production")
	policy.AllowedPorts[mustPort(t, upstreamPort)] = true
	policy.Resolver = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.11")}, nil
	}
	policy.DialContext = func(context.Context, string, string) (net.Conn, error) {
		return nil, fmt.Errorf("target dial must not be used through HTTPS proxy")
	}
	policy.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}
	policy.proxyFunc = fixedProxyFunc(proxy.URL, nil)
	request, err := http.NewRequest(http.MethodGet, "https://secure.example.test:"+upstreamPort+"/v1", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer test-credential-not-real")
	response, err := New(policy).Do(request)
	if err != nil {
		t.Fatalf("HTTPS proxy request failed: %v", err)
	}
	_ = response.Body.Close()
	if connectRequests.Load() != 1 {
		t.Fatalf("CONNECT requests=%d", connectRequests.Load())
	}

	policy.Resolver = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("10.0.0.8")}, nil
	}
	privateRequest, err := http.NewRequest(http.MethodGet, "https://private.example.test/v1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = New(policy).Do(privateRequest); ErrorCode(err) != CodeCredentialDestinationForbidden {
		t.Fatalf("private destination error=%v", err)
	}
	if connectRequests.Load() != 1 {
		t.Fatalf("private destination reached proxy: CONNECT requests=%d", connectRequests.Load())
	}
}

func TestProxyEnvironmentSelection(t *testing.T) {
	caseName := os.Getenv("SAFEHTTP_PROXY_CASE")
	if caseName != "" {
		var rawURL string
		switch caseName {
		case "https-proxy":
			rawURL = "https://external.example.test/v1"
		case "http-proxy":
			rawURL = "http://external.example.test/v1"
		case "no-proxy", "no-proxy-match":
			rawURL = "https://external.example.test/v1"
		default:
			t.Fatalf("unknown proxy test case %q", caseName)
		}
		request, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		policy := DefaultPolicy().withDefaults()
		proxyURL, err := proxyForRequest(request, policy)
		if err != nil {
			t.Fatal(err)
		}
		switch caseName {
		case "https-proxy", "http-proxy", "no-proxy-match":
			if proxyURL == nil || proxyURL.Host != "proxy.example.test:8080" {
				t.Fatalf("proxy=%v", proxyURL)
			}
		case "no-proxy":
			if proxyURL != nil {
				t.Fatalf("proxy=%v, want direct", proxyURL)
			}
		}
		return
	}

	cases := []struct {
		name string
		env  map[string]string
	}{
		{name: "https-proxy", env: map[string]string{"HTTPS_PROXY": "http://proxy.example.test:8080"}},
		{name: "http-proxy", env: map[string]string{"HTTP_PROXY": "http://proxy.example.test:8080"}},
		{name: "no-proxy", env: map[string]string{"HTTPS_PROXY": "http://proxy.example.test:8080", "NO_PROXY": "external.example.test"}},
		{name: "no-proxy-match", env: map[string]string{"HTTPS_PROXY": "http://proxy.example.test:8080", "NO_PROXY": "internal.example.test"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestProxyEnvironmentSelection$")
			cmd.Env = proxyTestEnvironment(test.name, test.env)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("proxy child failed: %v\n%s", err, output)
			}
		})
	}
}

func proxyTestEnvironment(caseName string, values map[string]string) []string {
	proxyNames := map[string]bool{
		"HTTP_PROXY": true, "HTTPS_PROXY": true, "ALL_PROXY": true, "NO_PROXY": true,
		"http_proxy": true, "https_proxy": true, "all_proxy": true, "no_proxy": true,
		"SAFEHTTP_PROXY_CASE": true,
	}
	environment := make([]string, 0, len(os.Environ())+len(values)+1)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !proxyNames[key] {
			environment = append(environment, entry)
		}
	}
	for key, value := range values {
		environment = append(environment, key+"="+value)
	}
	environment = append(environment, "SAFEHTTP_PROXY_CASE="+caseName)
	return environment
}

func TestControlledProxyRejectsMalformedConfiguration(t *testing.T) {
	for _, rawProxy := range []string{"://malformed", "http://proxy.example.test:not-a-port", "socks5://proxy.example.test:1080"} {
		t.Run(rawProxy, func(t *testing.T) {
			policy := CredentialPolicy("production")
			policy.Resolver = func(context.Context, string) ([]net.IP, error) {
				return []net.IP{net.ParseIP("203.0.113.12")}, nil
			}
			policy.proxyFunc = fixedProxyFunc(rawProxy, nil)
			request, err := http.NewRequest(http.MethodGet, "https://external.example.test/v1", nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = New(policy).Do(request); ErrorCode(err) != CodeProxyConfigurationInvalid {
				t.Fatalf("malformed proxy error=%v", err)
			}
		})
	}
}

func TestProxyAddressMatchingUsesDefaultPort(t *testing.T) {
	proxyURL, err := url.Parse("http://proxy.example.test")
	if err != nil {
		t.Fatal(err)
	}
	if !isProxyAddress("proxy.example.test:80", proxyURL) {
		t.Fatal("default HTTP proxy port did not match")
	}
}
