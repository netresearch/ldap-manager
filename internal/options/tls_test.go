package options

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParseTLSVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in      string
		want    uint16
		wantErr bool
	}{
		{"", 0, false},
		{"  ", 0, false},
		{"1.0", tls.VersionTLS10, false},
		{"1.1", tls.VersionTLS11, false},
		{"1.2", tls.VersionTLS12, false},
		{" 1.3 ", tls.VersionTLS13, false},
		{"1.4", 0, true},
		{"TLSv1.2", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got, err := parseTLSVersion("tls-min-version", tt.in)
			if tt.wantErr {
				var ve ValidationError
				if !errors.As(err, &ve) || ve.Field != "tls-min-version" {
					t.Fatalf("expected ValidationError for tls-min-version, got %v", err)
				}

				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %#x, want %#x", got, tt.want)
			}
		})
	}
}

func TestParseTLSCipherSuites(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    []uint16
		wantErr string
	}{
		{"empty", "", nil, ""},
		{"secure suite", "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384", []uint16{tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384}, ""},
		{
			"insecure static RSA suite is accepted",
			"TLS_RSA_WITH_AES_256_GCM_SHA384",
			[]uint16{tls.TLS_RSA_WITH_AES_256_GCM_SHA384},
			"",
		},
		{
			"list with spaces and empty items",
			" TLS_RSA_WITH_AES_256_GCM_SHA384 , ,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			[]uint16{tls.TLS_RSA_WITH_AES_256_GCM_SHA384, tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
			"",
		},
		{"unknown name", "TLS_RSA_WITH_AES_256_GCM_SHA384,AES256-GCM-SHA384", nil, "unknown cipher suite \"AES256-GCM-SHA384\""},
		{"TLS 1.3 suite", "TLS_AES_128_GCM_SHA256", nil, "TLS 1.3 suite"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseTLSCipherSuites("tls-cipher-suites", tt.in)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}

				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateTLSVersionRange(t *testing.T) {
	t.Parallel()

	suites := []uint16{tls.TLS_RSA_WITH_AES_256_GCM_SHA384}
	tests := []struct {
		name      string
		min, max  uint16
		suites    []uint16
		wantField string
	}{
		{"unset", 0, 0, nil, ""},
		{"equal", tls.VersionTLS12, tls.VersionTLS12, suites, ""},
		{"min only", tls.VersionTLS13, 0, nil, ""},
		{"max only", 0, tls.VersionTLS10, suites, ""},
		{"min above max", tls.VersionTLS13, tls.VersionTLS12, nil, "tls-min-version"},
		{"suites with min 1.3", tls.VersionTLS13, 0, suites, "tls-cipher-suites"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateTLSVersionRange(tt.min, tt.max, tt.suites)
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				return
			}
			var ve ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.wantField {
				t.Fatalf("expected ValidationError for %s, got %v", tt.wantField, err)
			}
		})
	}
}

func TestHasLDAPTLSSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts Opts
		want bool
	}{
		{"defaults", Opts{}, false},
		{"skip verify", Opts{TLSSkipVerify: true}, true},
		{"min version", Opts{TLSMinVersion: tls.VersionTLS12}, true},
		{"max version", Opts{TLSMaxVersion: tls.VersionTLS12}, true},
		{"cipher suites", Opts{TLSCipherSuites: []uint16{tls.TLS_RSA_WITH_AES_256_GCM_SHA384}}, true},
		{"CA file", Opts{TLSCAFile: "/ca.pem"}, true},
		{"server name", Opts{TLSServerName: "ldap.example.com"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.opts.HasLDAPTLSSettings(); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLDAPTLSConfig_Fields(t *testing.T) {
	t.Parallel()

	opts := Opts{
		TLSMaxVersion:   tls.VersionTLS12,
		TLSCipherSuites: []uint16{tls.TLS_RSA_WITH_AES_256_GCM_SHA384},
		TLSServerName:   "ldap.example.com",
		TLSSkipVerify:   true,
	}
	cfg, err := opts.LDAPTLSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion: got %#x, want TLS 1.2 when not configured", cfg.MinVersion)
	}
	if cfg.MaxVersion != tls.VersionTLS12 {
		t.Errorf("MaxVersion: got %#x", cfg.MaxVersion)
	}
	if !slices.Equal(cfg.CipherSuites, opts.TLSCipherSuites) {
		t.Errorf("CipherSuites: got %v", cfg.CipherSuites)
	}
	if cfg.ServerName != "ldap.example.com" {
		t.Errorf("ServerName: got %q", cfg.ServerName)
	}
	if !cfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify: expected true")
	}
	if cfg.RootCAs != nil {
		t.Error("RootCAs: expected nil (system pool) without a CA file")
	}

	opts.TLSMinVersion = tls.VersionTLS10
	cfg, err = opts.LDAPTLSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MinVersion != tls.VersionTLS10 {
		t.Errorf("MinVersion: got %#x, want the configured TLS 1.0", cfg.MinVersion)
	}
}

func TestLDAPTLSConfig_CAFileErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	notPEM := filepath.Join(dir, "not.pem")
	if err := os.WriteFile(notPEM, []byte("not a certificate\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for name, path := range map[string]string{
		"missing file": filepath.Join(dir, "missing.pem"),
		"no PEM":       notPEM,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts := Opts{TLSCAFile: path}
			_, err := opts.LDAPTLSConfig()
			var ve ValidationError
			if !errors.As(err, &ve) || ve.Field != "tls-ca-file" {
				t.Fatalf("expected ValidationError for tls-ca-file, got %v", err)
			}
		})
	}
}

func TestInsecureTLSCipherSuiteNames(t *testing.T) {
	t.Parallel()

	opts := Opts{TLSCipherSuites: []uint16{
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	}}
	got := opts.InsecureTLSCipherSuiteNames()
	if !slices.Equal(got, []string{"TLS_RSA_WITH_AES_256_GCM_SHA384"}) {
		t.Errorf("got %v", got)
	}
	if names := (&Opts{}).InsecureTLSCipherSuiteNames(); len(names) != 0 {
		t.Errorf("expected none, got %v", names)
	}
}

// staticRSAServer starts a TLS 1.2 server that offers only
// TLS_RSA_WITH_AES_256_GCM_SHA384 — the configuration of the eDirectory
// server in issue #737 — with a self-signed certificate for serverName. It
// returns the address and the path of a PEM file holding the certificate.
func staticRSAServer(t *testing.T, serverName string) (string, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: serverName},
		DNSNames:              []string{serverName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
		MinVersion:   tls.VersionTLS12,
		MaxVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{tls.TLS_RSA_WITH_AES_256_GCM_SHA384},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				_ = conn.(*tls.Conn).Handshake()
				_ = conn.Close()
			}()
		}
	}()

	return ln.Addr().String(), caFile
}

func dialTLS(t *testing.T, addr string, cfg *tls.Config) (*tls.Conn, error) {
	t.Helper()

	dialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: 5 * time.Second}, Config: cfg}
	conn, err := dialer.DialContext(t.Context(), "tcp", addr)
	if err != nil {
		return nil, err
	}

	return conn.(*tls.Conn), nil
}

// TestLDAPTLSConfig_StaticRSAServer reproduces issue #737: a server that
// offers only a static RSA key exchange suite fails the handshake with the
// Go defaults, and succeeds once the suite is configured. The CA file and the
// server name take part too: verification stays on, against a certificate
// that is not in the system trust store and is issued for another name than
// the dialled IP address.
func TestLDAPTLSConfig_StaticRSAServer(t *testing.T) {
	t.Parallel()

	const serverName = "ldap.example.test"
	addr, caFile := staticRSAServer(t, serverName)

	defaultOpts := Opts{TLSCAFile: caFile, TLSServerName: serverName}
	defaultCfg, err := defaultOpts.LDAPTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dialTLS(t, addr, defaultCfg); err == nil || !strings.Contains(err.Error(), "handshake failure") {
		t.Fatalf("default cipher suites: expected handshake failure, got %v", err)
	}

	opts := Opts{
		TLSMinVersion:   tls.VersionTLS12,
		TLSMaxVersion:   tls.VersionTLS12,
		TLSCipherSuites: []uint16{tls.TLS_RSA_WITH_AES_256_GCM_SHA384},
		TLSCAFile:       caFile,
		TLSServerName:   serverName,
	}
	cfg, err := opts.LDAPTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	conn, err := dialTLS(t, addr, cfg)
	if err != nil {
		t.Fatalf("configured cipher suite: handshake failed: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if got := conn.ConnectionState().CipherSuite; got != tls.TLS_RSA_WITH_AES_256_GCM_SHA384 {
		t.Errorf("negotiated %s", tls.CipherSuiteName(got))
	}

	// Without the CA file the self-signed certificate does not verify.
	opts.TLSCAFile = ""
	cfg, err = opts.LDAPTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dialTLS(t, addr, cfg); err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("without CA file: expected a certificate error, got %v", err)
	}

	// Without the server name, the IP address does not match the certificate.
	opts.TLSCAFile = caFile
	opts.TLSServerName = ""
	cfg, err = opts.LDAPTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dialTLS(t, addr, cfg); err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("without server name: expected a certificate error, got %v", err)
	}
}
