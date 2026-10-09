package node

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
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

// coreCAFixture builds a private CA and a leaf it signed, and writes the CA
// where the node can retain it. A TLS server started with the leaf alone
// reproduces the reported deployment: no chain and no AIA are sent.
func coreCAFixture(t *testing.T) (string, tls.Certificate) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "OAC test CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caFile := filepath.Join(t.TempDir(), "core-ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600); err != nil {
		t.Fatal(err)
	}
	return caFile, tls.Certificate{Certificate: [][]byte{leafDER}, PrivateKey: leafKey}
}

func leafOnlyServer(t *testing.T, leaf tls.Certificate, handler http.Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{leaf}}
	server.StartTLS()
	return server
}

func TestRootPoolAppendsOperatorCAPreservingSystemRoots(t *testing.T) {
	caFile, _ := coreCAFixture(t)
	system, err := x509.SystemCertPool()
	if err != nil {
		t.Skipf("no system certificate pool: %v", err)
	}
	base := len(system.Subjects())
	pool, err := rootPool(caFile)
	if err != nil {
		t.Fatal(err)
	}
	// The operator CA is appended, so the system anchors must still be present.
	if got := len(pool.Subjects()); got != base+1 {
		t.Fatalf("pool has %d anchors, want %d system anchors plus the operator CA", got, base)
	}
	plain, err := rootPool("")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(plain.Subjects()); got != base {
		t.Fatalf("empty path changed the system pool: %d anchors, want %d", got, base)
	}
}

func TestRootPoolRefusesMissingAndUnparseableCA(t *testing.T) {
	dir := t.TempDir()
	if _, err := rootPool(filepath.Join(dir, "missing.pem")); err == nil {
		t.Fatal("missing CA file was accepted")
	}
	garbage := filepath.Join(dir, "garbage.pem")
	if err := os.WriteFile(garbage, []byte("not a certificate\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := rootPool(garbage); err == nil {
		t.Fatal("unparseable CA file was accepted")
	}
	link := filepath.Join(dir, "link.pem")
	if err := os.Symlink(garbage, link); err != nil {
		t.Fatal(err)
	}
	if _, err := rootPool(link); err == nil {
		t.Fatal("symlinked CA file was accepted")
	}
}

func TestCoreHTTPClientVerifiesWithOperatorCA(t *testing.T) {
	caFile, leaf := coreCAFixture(t)
	server := leafOnlyServer(t, leaf, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	trusted, err := coreHTTPClient(caFile)
	if err != nil {
		t.Fatal(err)
	}
	response, err := trusted.Get(server.URL)
	if err != nil {
		t.Fatalf("operator CA did not trust the leaf-only server: %v", err)
	}
	_ = response.Body.Close()

	// Without the CA the same leaf is an unknown authority: verification is never
	// skipped, it is only ever widened by the operator's explicit anchor.
	untrusted, err := coreHTTPClient("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := untrusted.Get(server.URL); err == nil || !strings.Contains(err.Error(), "certificate signed by unknown authority") {
		t.Fatalf("untrusted server = %v, want an unknown-authority failure", err)
	}
}

func TestEnrollmentRetainsOperatorCAForLaterReconnect(t *testing.T) {
	caFile, leaf := coreCAFixture(t)
	id := identity()
	server := leafOnlyServer(t, leaf, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sandbox-node/enroll" || r.Header.Get("Authorization") != "Bearer enrollment" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(EnrollmentResponse{SpecificationDigest: id.SpecificationDigest,
			DeploymentGeneration: id.DeploymentGeneration, NodeID: id.NodeID, InstallationID: id.InstallationID,
			Provider: id.Provider, MaxActive: 2, MaxRetained: 8})
	}))
	defer server.Close()

	dir := stateDir(t)
	stored, err := InitIdentityWithCoreCA(dir, server.URL, id, false, caFile)
	if err != nil {
		t.Fatal(err)
	}
	if stored.CoreCA != caFile || stored.CoreCASHA256 == "" {
		t.Fatalf("retained identity did not record the CA: %+v", stored)
	}
	if _, err := Enroll(t.Context(), server.URL, dir, "enrollment", EnrollmentRequest{Name: "test"}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadIdentity(dir)
	if err != nil || reloaded.CoreCA != caFile || reloaded.CoreCASHA256 != stored.CoreCASHA256 {
		t.Fatalf("retained CA did not survive enrollment: %+v %v", reloaded, err)
	}
	// A rerun with a different (or absent) CA is refused rather than silently
	// widening or narrowing the node's trust.
	if _, err := InitIdentityWithCoreCA(dir, server.URL, id, false, ""); err == nil {
		t.Fatal("rerun without the retained CA was accepted")
	}
	other := filepath.Join(t.TempDir(), "other.pem")
	if err := os.WriteFile(other, mustReadFile(t, caFile), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := InitIdentityWithCoreCA(dir, server.URL, id, false, other); err == nil {
		t.Fatal("rerun with a different CA path was accepted")
	}
}

func TestRunReconnectsWithRetainedOperatorCA(t *testing.T) {
	caFile, leaf := coreCAFixture(t)
	id := identity()
	var credential string
	hub := NewHub(HubOptions{
		Authenticate: func(_ context.Context, node, token string) (Identity, error) {
			if node != id.NodeID || token != credential {
				return Identity{}, ErrAuthentication
			}
			return id, nil
		},
		OwnerEpoch: func(context.Context) (uint64, error) { return 7, nil },
	})
	server := leafOnlyServer(t, leaf, hub)
	defer server.Close()
	defer hub.Close()

	dir := stateDir(t)
	stored, err := InitIdentityWithCoreCA(dir, server.URL, id, false, caFile)
	if err != nil {
		t.Fatal(err)
	}
	credential = stored.Credential
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, AgentConfig{CoreURL: server.URL, StateDirectory: dir, Identity: id,
			Credential: credential, Provider: &fakeProvider{}, Probe: probe})
	}()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("agent did not stop")
		}
	}()
	wait(t, func() bool { return hub.Online(id.NodeID) })
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
