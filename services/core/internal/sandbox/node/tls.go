package node

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// rootPool is the Core trust anchor set: the system store with the operator's
// CA file appended. An empty path leaves the system store untouched; a missing,
// non-regular or unparseable file is a hard error, never a fallback that skips
// verification. The CA is appended, never substituted, so a Core response that
// redirects to a public release host is still verified against the system store.
func rootPool(caPath string) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, errors.New("cannot read the system certificate store")
	}
	if caPath == "" {
		return pool, nil
	}
	data, err := readCoreCAFile(caPath)
	if err != nil {
		return nil, err
	}
	if !pool.AppendCertsFromPEM(data) {
		return nil, errors.New("core CA file holds no certificate")
	}
	return pool, nil
}

// readCoreCAFile reads one operator CA file, refusing anything but a canonical,
// absolute, regular file so a symlink cannot redirect the trust anchor.
func readCoreCAFile(caPath string) ([]byte, error) {
	if !filepath.IsAbs(caPath) || filepath.Clean(caPath) != caPath {
		return nil, errors.New("core CA path must be canonical and absolute")
	}
	info, err := os.Lstat(caPath)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("core CA must be a regular file")
	}
	data, err := os.ReadFile(caPath)
	if err != nil {
		return nil, errors.New("cannot read the core CA file")
	}
	return data, nil
}

// coreCAIdentity validates an operator CA file and returns its canonical path
// and SHA-256. No CA yields empty values; the retained identity records both so
// a rerun with a different CA is refused.
func coreCAIdentity(caPath string) (string, string, error) {
	if caPath == "" {
		return "", "", nil
	}
	data, err := readCoreCAFile(caPath)
	if err != nil {
		return "", "", err
	}
	probe := x509.NewCertPool()
	if !probe.AppendCertsFromPEM(data) {
		return "", "", errors.New("core CA file holds no certificate")
	}
	sum := sha256.Sum256(data)
	return caPath, hex.EncodeToString(sum[:]), nil
}

// tlsConfig is the one TLS policy for Core connections: real chain and hostname
// verification against the system store plus the node's retained CA.
func tlsConfig(caPath string) (*tls.Config, error) {
	pool, err := rootPool(caPath)
	if err != nil {
		return nil, err
	}
	return &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}, nil
}

// coreHTTPClient is the enrollment and capacity HTTP client for the retained CA.
func coreHTTPClient(caPath string) (*http.Client, error) {
	config, err := tlsConfig(caPath)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = config
	return &http.Client{Timeout: 15 * time.Second, Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}
