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
		// An IPv4-mapped prefix of /96 or longer folds into its IPv4 form
		// inside Fiber's net.ParseCIDR, so "::ffff:0:0/96" would trust every
		// IPv4 peer there is. Shorter than /96 it matches no IPv4 peer at all.
		{"IPv4-mapped prefix covering all of IPv4", "::ffff:0:0/96"},
		{"IPv4-mapped prefix of a subnet", "::ffff:192.0.2.0/120"},
		{"IPv4-mapped prefix cutting into the mapping", "::ffff:0:0/95"},
		{"IPv4-mapped bare address", "::ffff:192.0.2.5"},
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

// TestParseTrustedProxies_NamesThePlainFormOfAMappedEntry checks the part of
// the refusal that makes it actionable: the operator is told what to write.
//
// The values come from a probe of net.ParseCIDR, which is what Fiber v3.5.0
// uses: "::ffff:0:0/96" yields the network 0.0.0.0/0 and matches 8.8.8.8,
// while "::ffff:0:0/95" yields ::fffe:0:0/95 and matches no IPv4 peer.
func TestParseTrustedProxies_NamesThePlainFormOfAMappedEntry(t *testing.T) {
	tests := []struct {
		raw       string
		wantPlain string
	}{
		{"::ffff:0:0/96", "0.0.0.0/0"},
		{"::ffff:192.0.2.0/120", "192.0.2.0/24"},
		{"::ffff:0:0/95", "0.0.0.0/0"},
		{"::ffff:192.0.2.5", "192.0.2.5"},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			_, err := parseTrustedProxies(tt.raw)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantPlain,
				"the error must name the plain form to write instead")
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
