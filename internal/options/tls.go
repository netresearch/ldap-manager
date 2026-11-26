package options

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"slices"
	"strings"
)

// tlsVersions maps the values LDAP_TLS_MIN_VERSION and LDAP_TLS_MAX_VERSION
// accept to crypto/tls protocol versions.
var tlsVersions = map[string]uint16{
	"1.0": tls.VersionTLS10,
	"1.1": tls.VersionTLS11,
	"1.2": tls.VersionTLS12,
	"1.3": tls.VersionTLS13,
}

// parseTLSVersion converts "1.0" … "1.3" to a crypto/tls version. An empty
// value returns 0, which leaves the crypto/tls default in place.
func parseTLSVersion(field, value string) (uint16, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}

	v, ok := tlsVersions[value]
	if !ok {
		return 0, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("unknown TLS version %q, valid values are 1.0, 1.1, 1.2, 1.3", value),
		}
	}

	return v, nil
}

// parseTLSCipherSuites converts a comma-separated list of IANA cipher suite
// names (as crypto/tls names them, e.g. TLS_RSA_WITH_AES_256_GCM_SHA384) to
// their IDs. Names from tls.InsecureCipherSuites are accepted: listing one is
// the operator's explicit opt-in, and NewApp logs a warning for it. TLS 1.3
// suites are rejected because crypto/tls does not make them configurable.
func parseTLSCipherSuites(field, value string) ([]uint16, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	known := make(map[string]*tls.CipherSuite)
	for _, s := range slices.Concat(tls.CipherSuites(), tls.InsecureCipherSuites()) {
		known[s.Name] = s
	}

	var ids []uint16
	for name := range strings.SplitSeq(value, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		s, ok := known[name]
		if !ok {
			return nil, ValidationError{Field: field, Message: fmt.Sprintf("unknown cipher suite %q", name)}
		}
		if !slices.ContainsFunc(s.SupportedVersions, func(v uint16) bool { return v < tls.VersionTLS13 }) {
			return nil, ValidationError{
				Field:   field,
				Message: fmt.Sprintf("cipher suite %q is a TLS 1.3 suite; TLS 1.3 suites are not configurable", name),
			}
		}
		ids = append(ids, s.ID)
	}

	return ids, nil
}

// validateTLSVersionRange rejects a minimum above the maximum, and a cipher
// suite list that cannot take effect because the minimum is TLS 1.3.
func validateTLSVersionRange(minVersion, maxVersion uint16, cipherSuites []uint16) error {
	if minVersion != 0 && maxVersion != 0 && minVersion > maxVersion {
		return ValidationError{
			Field:   "tls-min-version",
			Message: fmt.Sprintf("%s is higher than tls-max-version %s", tls.VersionName(minVersion), tls.VersionName(maxVersion)),
		}
	}
	if minVersion == tls.VersionTLS13 && len(cipherSuites) > 0 {
		return ValidationError{
			Field:   "tls-cipher-suites",
			Message: "cipher suites apply to TLS 1.2 and lower only, but tls-min-version is 1.3",
		}
	}

	return nil
}

// HasLDAPTLSSettings reports whether any LDAP TLS option differs from its
// default. When it returns false, the LDAP client keeps the crypto/tls
// defaults and NewApp passes no TLS configuration at all.
func (o *Opts) HasLDAPTLSSettings() bool {
	return o.TLSSkipVerify || o.TLSMinVersion != 0 || o.TLSMaxVersion != 0 ||
		len(o.TLSCipherSuites) > 0 || o.TLSCAFile != "" || o.TLSServerName != ""
}

// LDAPTLSConfig builds the TLS configuration for LDAPS connections from the
// LDAP_TLS_* options.
//
// A CA file extends the system trust store; it does not replace it. A
// minimum version that is not set stays at TLS 1.2, the crypto/tls client
// default.
func (o *Opts) LDAPTLSConfig() (*tls.Config, error) {
	minVersion := o.TLSMinVersion
	if minVersion == 0 {
		minVersion = tls.VersionTLS12
	}

	// InsecureSkipVerify is set only by the operator through
	// LDAP_TLS_SKIP_VERIFY / --tls-skip-verify and defaults to false; NewApp
	// logs a warning when it is on. MinVersion below TLS 1.2 and the cipher
	// suites are likewise explicit operator settings.
	cfg := &tls.Config{
		MinVersion:         minVersion,
		MaxVersion:         o.TLSMaxVersion,
		CipherSuites:       slices.Clone(o.TLSCipherSuites),
		ServerName:         o.TLSServerName,
		InsecureSkipVerify: o.TLSSkipVerify, // #nosec G402 -- opt-in via LDAP_TLS_SKIP_VERIFY, default false (see comment above)
	}

	if o.TLSCAFile != "" {
		pool, err := loadCAPool(o.TLSCAFile)
		if err != nil {
			return nil, err
		}
		cfg.RootCAs = pool
	}

	return cfg, nil
}

// loadCAPool returns the system trust store extended by the PEM
// certificates in path.
func loadCAPool(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path) // #nosec G304 -- path is operator configuration
	if err != nil {
		return nil, ValidationError{Field: "tls-ca-file", Message: fmt.Sprintf("cannot read CA file: %v", err)}
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, ValidationError{
			Field:   "tls-ca-file",
			Message: fmt.Sprintf("%s contains no PEM-encoded certificate", path),
		}
	}

	return pool, nil
}

// InsecureTLSCipherSuiteNames returns the names of the configured cipher
// suites that crypto/tls lists as insecure, such as the static RSA key
// exchange suites. NewApp logs them at startup.
func (o *Opts) InsecureTLSCipherSuiteNames() []string {
	var names []string
	for _, s := range tls.InsecureCipherSuites() {
		if slices.Contains(o.TLSCipherSuites, s.ID) {
			names = append(names, s.Name)
		}
	}

	return names
}
