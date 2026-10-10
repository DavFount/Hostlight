package main

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// This first HTTP slice deliberately sends GET only: no credentials, cookies,
// request bodies, arbitrary headers, response body storage, or redirect following.
type HTTPProbeResult struct {
	Success    bool    `json:"success"`
	StatusCode int     `json:"statusCode"`
	Latency    float64 `json:"latency"`
	Message    string  `json:"message"`
}

type httpCheckError struct {
	cause   error
	message string
}

func (e *httpCheckError) Error() string { return e.message }
func (e *httpCheckError) Unwrap() error { return e.cause }
func expectedHTTPStatus(t Target) int {
	if t.ExpectedStatus == 0 {
		return 200
	}
	return t.ExpectedStatus
}
func canonicalHTTPURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	if (u.Scheme == "http" && u.Port() == "80") || (u.Scheme == "https" && u.Port() == "443") {
		u.Host = strings.TrimSuffix(u.Host, ":"+u.Port())
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}
func validateHTTPDefinition(t Target) error {
	if len(t.Host) > 2048 || strings.ContainsAny(t.Host, " \t\r\n") {
		return fmt.Errorf("Enter an HTTP/HTTPS URL without spaces, up to 2,048 characters")
	}
	u, err := url.Parse(t.Host)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Opaque != "" {
		return fmt.Errorf("Enter a complete http:// or https:// URL")
	}
	if u.User != nil || u.Fragment != "" {
		return fmt.Errorf("URL credentials and fragments are not supported; do not include secrets in URLs")
	}
	if strings.HasSuffix(u.Host, ":") {
		return fmt.Errorf("Enter a port after the URL colon, or remove the colon")
	}
	if port := u.Port(); port != "" {
		n, e := strconv.Atoi(port)
		if e != nil || n < 1 || n > 65535 {
			return fmt.Errorf("URL port must be 1–65535")
		}
	}
	if status := expectedHTTPStatus(t); status < 100 || status > 599 {
		return fmt.Errorf("Expected HTTP status must be 100–599")
	}
	return nil
}

func probeHTTP(ctx context.Context, t Target) (HTTPProbeResult, error) {
	result := HTTPProbeResult{}
	if err := validateHTTPDefinition(t); err != nil {
		return result, &probeSetupError{err.Error()}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(t.Timeout)*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, t.Host, nil)
	if err != nil {
		return result, &probeSetupError{"Could not create HTTP request"}
	}
	request.Header.Set("User-Agent", "Hostlight/1")
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Avoid accumulating a pool per monitor, and keep each check a fresh connection.
	transport.DisableKeepAlives = true
	transport.MaxResponseHeaderBytes = 64 << 10
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	started := time.Now()
	response, err := client.Do(request)
	result.Latency = float64(time.Since(started).Microseconds()) / 1000
	if err != nil {
		result.Message = httpFailureMessage(err)
		// Preserve wrapped DNS errors for the engine's local-check-error classification,
		// but do not return a URL-bearing error through the preview API.
		return result, &httpCheckError{err, result.Message}
	}
	defer response.Body.Close()
	result.StatusCode = response.StatusCode
	result.Success = response.StatusCode == expectedHTTPStatus(t)
	result.Message = fmt.Sprintf("HTTP %d · Expected status received", response.StatusCode)
	if !result.Success {
		result.Message = fmt.Sprintf("HTTP %d · Expected %d", response.StatusCode, expectedHTTPStatus(t))
		if response.StatusCode >= 300 && response.StatusCode < 400 {
			result.Message += " (redirects are not followed)"
		}
		return result, fmt.Errorf("%s", result.Message)
	}
	return result, nil
}

func httpFailureMessage(err error) string {
	var dns *net.DNSError
	var network net.Error
	var unknown x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	switch {
	case errors.Is(err, context.Canceled):
		return "HTTP request cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "HTTP request timed out before response headers"
	case errors.As(err, &dns):
		return "HTTP DNS lookup failed. Check the URL hostname and local DNS connection."
	case errors.As(err, &unknown):
		return "HTTPS certificate is not trusted"
	case errors.As(err, &hostname):
		return "HTTPS certificate does not match the URL hostname"
	case errors.As(err, &invalid):
		return "HTTPS certificate is expired or invalid"
	case errors.As(err, &network) && network.Timeout():
		return "HTTP request timed out before response headers"
	default:
		return "HTTP connection or protocol failed. No expected response received."
	}
}

// TestHTTPRequest performs one bounded request, never creating a monitor,
// incident or notification. It uses the same policy as scheduled checks.
func (a *App) TestHTTPRequest(t Target) (HTTPProbeResult, error) {
	t.Type = "http"
	t.Port = 0
	t.Host = strings.TrimSpace(t.Host)
	if t.Name == "" {
		t.Name = "Request preview"
	}
	if err := validate(t); err != nil {
		return HTTPProbeResult{}, err
	}
	a.mu.Lock()
	ctx := a.ctx
	closing := a.closing
	a.mu.Unlock()
	if closing {
		return HTTPProbeResult{}, fmt.Errorf("Hostlight is closing")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(t.Timeout)*time.Second)
	defer cancel()
	select {
	case a.slots <- struct{}{}:
		defer func() { <-a.slots }()
	case <-ctx.Done():
		return HTTPProbeResult{}, fmt.Errorf("Request preview timed out waiting for a probe slot")
	}
	result, _ := probeHTTP(ctx, t)
	return result, nil
}
