package web

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"log/slog"
	"math/big"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/options"
)

// staticRSALDAPSServer starts a TLS 1.2 listener that offers only
// TLS_RSA_WITH_AES_256_GCM_SHA384, as the eDirectory server in issue #737
// does. It closes each connection after the handshake and counts the
// handshakes that succeeded.
func staticRSALDAPSServer(t *testing.T) (string, *atomic.Int32) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "ldap.example.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
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

	var handshakes atomic.Int32
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				if conn.(*tls.Conn).Handshake() == nil {
					handshakes.Add(1)
				}
				_ = conn.Close()
			}()
		}
	}()

	return "ldaps://" + ln.Addr().String(), &handshakes
}

// TestNewApp_LDAPTLSCipherSuitesReachDial checks that the LDAP_TLS_* options
// reach the service account's LDAP dial. With the Go default cipher suites
// the handshake against a static-RSA-only server fails, as in issue #737;
// with LDAP_TLS_CIPHER_SUITES naming the server's suite it succeeds. The
// server closes the connection after the handshake, so NewApp still returns
// an error in both cases; the handshake count tells them apart.
func TestNewApp_LDAPTLSCipherSuitesReachDial(t *testing.T) {
	chdirToRepoRoot(t)

	newOpts := func(server string) *options.Opts {
		return &options.Opts{
			LDAP:                  ldap.Config{Server: server, BaseDN: "dc=example,dc=com"},
			ReadonlyUser:          "cn=readonly,dc=example,dc=com",
			ReadonlyPassword:      "secret", // pragma: allowlist secret
			SessionDuration:       30 * time.Minute,
			PoolMaxConnections:    1,
			PoolMinConnections:    1,
			PoolMaxIdleTime:       time.Minute,
			PoolMaxLifetime:       options.DefaultPoolMaxLifetime,
			PoolConnectionTimeout: 2 * time.Second,
			PoolAcquireTimeout:    2 * time.Second,
			// The test certificate is self-signed; verification is covered by
			// the options package tests.
			TLSSkipVerify: true,
		}
	}

	t.Run("default cipher suites", func(t *testing.T) {
		server, handshakes := staticRSALDAPSServer(t)
		_, err := NewApp(newOpts(server))
		if err == nil || !strings.Contains(err.Error(), "handshake failure") {
			t.Fatalf("expected handshake failure, got %v", err)
		}
		if n := handshakes.Load(); n != 0 {
			t.Errorf("expected no successful handshake, got %d", n)
		}
	})

	t.Run("configured cipher suite", func(t *testing.T) {
		server, handshakes := staticRSALDAPSServer(t)
		opts := newOpts(server)
		opts.TLSMaxVersion = tls.VersionTLS12
		opts.TLSCipherSuites = []uint16{tls.TLS_RSA_WITH_AES_256_GCM_SHA384}
		_, err := NewApp(opts)
		if err != nil && strings.Contains(err.Error(), "handshake failure") {
			t.Fatalf("handshake failed with the configured suite: %v", err)
		}
		if handshakes.Load() == 0 {
			t.Errorf("expected a successful handshake, got none (NewApp error: %v)", err)
		}
	})
}

func TestNewApp_LDAPTLSCAFileError(t *testing.T) {
	chdirToRepoRoot(t)

	opts := &options.Opts{
		LDAP:      ldap.Config{Server: unreachableLDAPServer, BaseDN: "dc=example,dc=com"},
		TLSCAFile: t.TempDir() + "/missing.pem",
	}
	_, err := NewApp(opts)
	if err == nil || !strings.Contains(err.Error(), "tls-ca-file") {
		t.Fatalf("expected tls-ca-file error, got %v", err)
	}
}

func TestWarnLDAPTLSSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts options.Opts
		want []string
		not  []string
	}{
		{
			name: "secure settings on ldaps",
			opts: options.Opts{
				LDAP:            ldap.Config{Server: "ldaps://ldap.example.com"},
				TLSMinVersion:   tls.VersionTLS12,
				TLSCipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384},
				TLSServerName:   "ldap.example.com",
			},
			not: []string{"level=WARN"},
		},
		{
			name: "weak settings",
			opts: options.Opts{
				LDAP:            ldap.Config{Server: "LDAPS://ldap.example.com"},
				TLSSkipVerify:   true,
				TLSMinVersion:   tls.VersionTLS10,
				TLSCipherSuites: []uint16{tls.TLS_RSA_WITH_AES_256_GCM_SHA384},
			},
			want: []string{
				"TLS certificate verification is disabled",
				"TLS_RSA_WITH_AES_256_GCM_SHA384",
				"min_version=\"TLS 1.0\"",
			},
			not: []string{"no effect"},
		},
		{
			name: "plain ldap",
			opts: options.Opts{
				LDAP:          ldap.Config{Server: "ldap://ldap.example.com"},
				TLSServerName: "ldap.example.com",
			},
			want: []string{"LDAP TLS settings have no effect"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			warnLDAPTLSSettings(&tt.opts, slog.New(slog.NewTextHandler(&buf, nil)))
			out := buf.String()
			for _, s := range tt.want {
				if !strings.Contains(out, s) {
					t.Errorf("expected %q in log output:\n%s", s, out)
				}
			}
			for _, s := range tt.not {
				if strings.Contains(out, s) {
					t.Errorf("unexpected %q in log output:\n%s", s, out)
				}
			}
		})
	}
}
