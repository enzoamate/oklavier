// Package mtls provides shared mutual-TLS configuration for the agent ↔ core
// channel. mTLS is OPT-IN. When the relevant env vars are unset, all helpers
// return nil/zero values and callers fall back to plain HTTP.
//
// Required env (when enabling):
//   - MTLS_CERT_FILE  PEM-encoded server/client certificate
//   - MTLS_KEY_FILE   PEM-encoded private key matching MTLS_CERT_FILE
//   - MTLS_CA_FILE    PEM-encoded CA bundle used to verify the peer
//
// Typical operator workflow (cert-manager):
//
//	apiVersion: cert-manager.io/v1
//	kind: Certificate
//	metadata: { name: oklavier-core-mtls, namespace: oklavier }
//	spec:
//	  secretName: oklavier-core-mtls
//	  issuerRef: { name: oklavier-mtls-ca, kind: ClusterIssuer }
//	  commonName: core
//	  dnsNames: [oklavier-agent.oklavier.svc]
//	  usages: [server auth, client auth]
//
// Mount the resulting secret at /mtls/ on both the core and every agent and
// set MTLS_CERT_FILE=/mtls/tls.crt, MTLS_KEY_FILE=/mtls/tls.key,
// MTLS_CA_FILE=/mtls/ca.crt.
package mtls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
)

// Enabled reports whether the env vars to enable mTLS are all set.
func Enabled() bool {
	return os.Getenv("MTLS_CERT_FILE") != "" &&
		os.Getenv("MTLS_KEY_FILE") != "" &&
		os.Getenv("MTLS_CA_FILE") != ""
}

// ServerConfig builds a *tls.Config for an HTTP server that REQUIRES a
// valid client certificate signed by the configured CA. Returns (nil, nil)
// when mTLS is not configured — callers should fall back to plain HTTP.
func ServerConfig() (*tls.Config, error) {
	if !Enabled() {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(os.Getenv("MTLS_CERT_FILE"), os.Getenv("MTLS_KEY_FILE"))
	if err != nil {
		return nil, fmt.Errorf("mtls: load server keypair: %w", err)
	}
	caPEM, err := os.ReadFile(os.Getenv("MTLS_CA_FILE"))
	if err != nil {
		return nil, fmt.Errorf("mtls: read CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("mtls: CA bundle has no valid PEM certs")
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// ClientTransport builds an *http.Transport that presents the configured
// client certificate and verifies the peer against the configured CA.
// Returns (nil, nil) when mTLS is not configured.
func ClientTransport() (*http.Transport, error) {
	if !Enabled() {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(os.Getenv("MTLS_CERT_FILE"), os.Getenv("MTLS_KEY_FILE"))
	if err != nil {
		return nil, fmt.Errorf("mtls: load client keypair: %w", err)
	}
	caPEM, err := os.ReadFile(os.Getenv("MTLS_CA_FILE"))
	if err != nil {
		return nil, fmt.Errorf("mtls: read CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("mtls: CA bundle has no valid PEM certs")
	}
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      pool,
			MinVersion:   tls.VersionTLS12,
		},
	}, nil
}
