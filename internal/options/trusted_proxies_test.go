package options

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseTrustedProxies_Accepts covers the forms an operator may write: a
// CIDR range, a bare address, several of them, and stray whitespace around
// the separators.
func TestParseTrustedProxies_Accepts(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"single CIDR", "10.0.0.0/8", []string{"10.0.0.0/8"}},
		{"bare IPv4 address", "192.0.2.7", []string{"192.0.2.7"}},
		{"bare IPv6 address", "2001:db8::1", []string{"2001:db8::1"}},
		{"IPv6 CIDR", "fc00::/7", []string{"fc00::/7"}},
		{
			"several entries with whitespace",
			" 127.0.0.0/8 , ::1/128 ,10.42.0.0/16 ",
			[]string{"127.0.0.0/8", "::1/128", "10.42.0.0/16"},
		},
		{"trailing separator", "10.0.0.0/8,", []string{"10.0.0.0/8"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTrustedProxies(tt.raw)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseTrustedProxies_Rejects covers what must not reach Fiber. A bad
// entry silently dropped would leave the app trusting less than the operator
// wrote, which surfaces later as a CSRF origin mismatch with no clue attached.
func TestParseTrustedProxies_Rejects(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"a hostname", "proxy.example.com"},
		{"a CIDR with a bad prefix length", "10.0.0.0/64"},
		{"an address with a port", "192.0.2.7:8080"},
		{"plain text", "trust-everything"},
		{"one bad entry among good ones", "127.0.0.0/8,not-an-ip,::1/128"},
		{"empty", ""},
		{"only separators and spaces", " , , "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTrustedProxies(tt.raw)
			require.Error(t, err)
			assert.Nil(t, got)

			var validationErr ValidationError
			require.True(t, errors.As(err, &validationErr),
				"the error must name the offending option")
			assert.Equal(t, "trusted-proxies", validationErr.Field)
		})
	}
}

// TestDefaultTrustedProxies pins the shipped default. It is the list the app
// carried before the option existed, so an upgrade changes nothing, and it is
// narrow on purpose: the same list decides whose X-Forwarded-For the login
// rate limiter believes.
func TestDefaultTrustedProxies(t *testing.T) {
	assert.Equal(t, []string{"127.0.0.0/8", "::1/128", "172.16.0.0/12"}, DefaultTrustedProxies)

	parsed, err := parseTrustedProxies("127.0.0.0/8,::1/128,172.16.0.0/12")
	require.NoError(t, err)
	assert.Equal(t, DefaultTrustedProxies, parsed,
		"the default must survive its own parser")
}
