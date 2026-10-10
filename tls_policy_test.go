package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tlsPolicyFixture(t *testing.T, expires time.Time) (*httptest.Server, *http.Transport) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: expires, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, DNSNames: []string{"localhost"}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"ok":true}`)) }))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	server.StartTLS()
	t.Cleanup(server.Close)
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: roots}
	t.Cleanup(transport.CloseIdleConnections)
	return server, transport
}

func TestCertificatePolicyBoundaries(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	target := httpTarget("https://example.com")
	target.TLSMinDays = 30
	if err := checkCertificateValidity(target, now.Add(30*24*time.Hour).Format(time.RFC3339), now); err != nil {
		t.Fatal("exact minimum should pass", err)
	}
	if err := checkCertificateValidity(target, now.Add(30*24*time.Hour-time.Second).Format(time.RFC3339), now); err == nil {
		t.Fatal("below minimum accepted")
	}
	if checkCertificateValidity(target, "", now) == nil {
		t.Fatal("missing expiry accepted")
	}
	target.TLSMinDays = 0
	if err := checkCertificateValidity(target, "", now); err != nil {
		t.Fatal("disabled policy failed", err)
	}
	for _, days := range []int{-1, 366} {
		target.TLSMinDays = days
		if validate(target) == nil {
			t.Fatal("invalid minimum accepted", days)
		}
	}
	target.TLSMinDays = 30
	target.Host = "http://example.com"
	if validate(target) == nil {
		t.Fatal("plaintext expiry policy accepted")
	}
}

func TestTrustedHTTPSCertificateMetadataAndPolicy(t *testing.T) {
	expires := time.Now().Add(3 * 24 * time.Hour).UTC().Truncate(time.Second)
	server, transport := tlsPolicyFixture(t, expires)
	target := httpTarget(server.URL)
	result, err := probeHTTPWithTransport(context.Background(), target, transport.Clone())
	if err != nil || !result.Success || result.CertificateExpires != expires.Format(time.RFC3339) {
		t.Fatalf("trusted fixture failed: %+v %v", result, err)
	}
	target.TLSMinDays = 30
	result, err = probeHTTPWithTransport(context.Background(), target, transport.Clone())
	if err == nil || result.Success || result.StatusCode != 200 || result.CertificateExpires == "" || !strings.Contains(result.Message, "Minimum validity policy failed") {
		t.Fatalf("expiry policy ignored: %+v %v", result, err)
	}
	target.TLSMinDays = 1
	target.BodyMode = "json"
	target.JSONPointer = "/ok"
	target.JSONType = "boolean"
	target.BodyExpected = "true"
	result, err = probeHTTPWithTransport(context.Background(), target, transport.Clone())
	if err != nil || !result.Success || !strings.Contains(result.Message, "body matched") {
		t.Fatalf("TLS and body policies did not compose: %+v %v", result, err)
	}
	result, err = probeHTTP(context.Background(), target)
	if err == nil || result.Success || result.CertificateExpires != "" {
		t.Fatal("production trust verification bypassed", result, err)
	}
}

func TestExpiredCertificateStillFailsWithPolicyDisabled(t *testing.T) {
	server, transport := tlsPolicyFixture(t, time.Now().Add(-time.Minute))
	target := httpTarget(server.URL)
	result, err := probeHTTPWithTransport(context.Background(), target, transport.Clone())
	if err == nil || result.Success || result.CertificateExpires != "" || !strings.Contains(result.Message, "expired or invalid") {
		t.Fatalf("expired certificate accepted: %+v %v", result, err)
	}
}

func TestCertificatePolicyPersistenceAndBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "targets.json")
	a := storedApp(t, path)
	target := httpTarget("https://service.example/health")
	target.TLSMinDays = 30
	target.CertificateExpires = "2099-01-01T00:00:00Z"
	if err := a.SaveTarget(target); err != nil {
		t.Fatal(err)
	}
	id := a.GetTargets()[0].ID
	a.mu.Lock()
	a.targets[id].target.CertificateExpires = "2099-01-01T00:00:00Z"
	err := a.persistLocked()
	a.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	b := storedApp(t, path)
	restored := b.GetTargets()[0]
	if restored.TLSMinDays != 30 || restored.CertificateExpires != "" {
		t.Fatalf("bad restore: %+v", restored)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "certificateExpires") {
		t.Fatal("runtime expiry saved as configuration")
	}
	a.mu.Lock()
	backup := a.monitorBackupLocked()
	a.mu.Unlock()
	data, err := json.Marshal(backup)
	if err != nil {
		t.Fatal(err)
	}
	if backup.Version != 4 || strings.Contains(string(data), "certificateExpires") {
		t.Fatal("runtime expiry leaked to backup")
	}
	c := portabilityApp(t)
	preview, err := c.prepareMonitorImport(data, "tls.json")
	if err != nil || preview.Ready != 1 {
		t.Fatal(preview, err)
	}
	if _, err = c.CommitMonitorImport(preview.Token, false); err != nil {
		t.Fatal(err)
	}
	imported := c.GetTargets()[0]
	if imported.TLSMinDays != 30 || imported.CertificateExpires != "" || imported.State != "paused" {
		t.Fatalf("bad policy import: %+v", imported)
	}
	backup.Version = 3
	data, _ = json.Marshal(backup)
	preview, err = c.prepareMonitorImport(data, "old.json")
	if err != nil || preview.Invalid != 1 {
		t.Fatal("legacy backup accepted expiry policy", preview, err)
	}
}
