//go:build integration

package integration

// Ports the HTTP helpers of test/integration/integration_test.rb at Kamal 2.12.0. The
// load balancer sends each request to vm1 or vm2, where kamal-proxy routes it by
// host name: 127.0.0.1 for app, and localhost for the other apps.

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// response is the status code and the body without surrounding whitespace.
type response struct {
	code string
	body string
}

func appHost(app string) string {
	if app == "app" {
		return "127.0.0.1"
	}
	return "localhost"
}

// appResponse gets /version of app through the load balancer, or over HTTPS on vm1
// with the certificate at certPath.
func (s *stack) appResponse(app, certPath string) response {
	s.t.Helper()
	if certPath == "" {
		return s.get(&http.Client{Timeout: 15 * time.Second}, "http://"+appHost(app)+":"+s.httpPort+"/version", "")
	}
	return s.get(s.clientTrusting(certPath), "https://"+appHost(app)+":"+s.httpsPort+"/version", "")
}

func (s *stack) clientTrusting(certPath string) *http.Client {
	s.t.Helper()
	pem, err := os.ReadFile(certPath)
	if err != nil {
		s.t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(pem)
	// Kamal's test certificate names the host only in its Common Name. Ruby's OpenSSL
	// accepts that, and Go's verifier wants a SAN, so the harness checks the chain
	// against the certificate and the host against the Common Name itself.
	config := &tls.Config{InsecureSkipVerify: true, VerifyConnection: func(state tls.ConnectionState) error {
		return verifyCommonName(state, pool)
	}}
	return &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{TLSClientConfig: config}}
}

func verifyCommonName(state tls.ConnectionState, pool *x509.CertPool) error {
	leaf := state.PeerCertificates[0]
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: pool}); err != nil {
		return err
	}
	if leaf.Subject.CommonName != state.ServerName {
		return fmt.Errorf("certificate is for %q, want %q", leaf.Subject.CommonName, state.ServerName)
	}
	return nil
}

// get sends a GET with an optional Host header and fails the test when the
// request fails.
func (s *stack) get(client *http.Client, url, host string) response {
	s.t.Helper()
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		s.t.Fatal(err)
	}
	if host != "" {
		request.Host = host
	}
	resp, err := client.Do(request)
	if err != nil {
		s.t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return response{code: resp.Status[:3], body: strings.TrimSpace(string(body))}
}

func (s *stack) assertAppIsUp(version string) { s.t.Helper(); s.assertAppIsUpAs(s.app, version, "") }

// assertAppIsUpAs checks for 200 and, when version is set, the deployed version.
func (s *stack) assertAppIsUpAs(app, version, certPath string) {
	s.t.Helper()
	resp := s.appResponse(app, certPath)
	s.debugResponseCode(resp, "200")
	if resp.code != "200" || (version != "" && resp.body != version) {
		s.t.Fatalf("app %s: got %s %q, want 200 %q", app, resp.code, resp.body, version)
	}
}

func (s *stack) assertAppIsDown()   { s.t.Helper(); s.assertAppErrorCode("502", "") }
func (s *stack) assertAppNotFound() { s.t.Helper(); s.assertAppErrorCode("404", "") }
func (s *stack) assertAppInMaintenance(message string) {
	s.t.Helper()
	s.assertAppErrorCode("503", message)
}

func (s *stack) assertAppErrorCode(code, message string) {
	s.t.Helper()
	resp := s.appResponse(s.app, "")
	s.debugResponseCode(resp, code)
	if resp.code != code || (message != "" && !regexp.MustCompile(message).MatchString(resp.body)) {
		s.t.Fatalf("app %s: got %s %q, want %s %q", s.app, resp.code, resp.body, code, message)
	}
}

// waitForAppToBeUp waits until upCount checks return 200, as `app start` does not
// wait for the containers.
func (s *stack) waitForAppToBeUp() {
	s.t.Helper()
	deadline, ups := time.Now().Add(20*time.Second), 0
	for ups < 3 && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		if s.appResponse(s.app, "").code == "200" {
			ups++
		}
	}
	if ups < 3 {
		s.t.Fatalf("app %s answered 200 %d times in 20 seconds, want 3", s.app, ups)
	}
}

// debugResponseCode logs the proxy and load balancer logs for an unexpected code.
func (s *stack) debugResponseCode(resp response, want string) {
	if resp.code == want {
		return
	}
	proxyLogs, _ := s.wunjoTry("proxy", "logs")
	balancerLogs, _ := compose(s.testID, "logs", "--no-color", "load_balancer")
	s.t.Logf("got response code %s, want %s\nproxy logs:\n%s\nload balancer logs:\n%s", resp.code, want, proxyLogs, balancerLogs)
}
