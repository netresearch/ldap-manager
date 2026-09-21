package web

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ldap-manager/internal/options"
)

// forwardedView is what the app believes about a request after the trust list
// has been applied: the scheme the CSRF middleware compares against the
// browser's Origin, and the client IP the login rate limiter counts against.
type forwardedView struct {
	scheme string
	ip     string
}

// askForwardedView drives one request through an app built by the real
// createFiberApp with the given trust list, and reports what the app made of
// the two forwarded headers.
//
// fiber.App.Test presents the request from 0.0.0.0, so a list containing
// 0.0.0.0/0 is the trusted arm and one containing only loopback is the
// untrusted arm.
func askForwardedView(t *testing.T, trusted []string) forwardedView {
	t.Helper()

	f := createFiberApp(&options.Opts{TrustedProxies: trusted})
	f.Get("/seen", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"scheme": c.Scheme(), "ip": c.IP()})
	})

	req := httptest.NewRequest("GET", "/seen", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-For", "203.0.113.9")

	resp, err := f.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	var body struct {
		Scheme string `json:"scheme"`
		IP     string `json:"ip"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	return forwardedView{scheme: body.Scheme, ip: body.IP}
}

// TestTrustedProxies_HeadersHonouredFromATrustedPeer pins the reason the
// option exists: a TLS terminator that is named in the list makes the app
// report https, so the CSRF origin check matches the browser.
func TestTrustedProxies_HeadersHonouredFromATrustedPeer(t *testing.T) {
	seen := askForwardedView(t, []string{"0.0.0.0/0"})

	require.Equal(t, "https", seen.scheme,
		"a trusted peer's X-Forwarded-Proto must decide the scheme")
	require.Equal(t, "203.0.113.9", seen.ip,
		"a trusted peer's X-Forwarded-For must decide the client IP")
}

// TestTrustedProxies_HeadersIgnoredFromAnUntrustedPeer is the other direction,
// and the one that matters for the rate limiter: an unnamed peer must not be
// able to choose the IP its login attempts are counted against.
func TestTrustedProxies_HeadersIgnoredFromAnUntrustedPeer(t *testing.T) {
	seen := askForwardedView(t, []string{"127.0.0.0/8"})

	require.Equal(t, "http", seen.scheme,
		"an untrusted peer's X-Forwarded-Proto must be ignored")
	require.NotEqual(t, "203.0.113.9", seen.ip,
		"an untrusted peer must not be able to name its own client IP")
}

// TestTrustedProxies_DefaultExcludesAnArbitraryPeer pins the shipped default.
// It is deliberately narrow: loopback and the Docker bridge range, nothing
// else, so an unconfigured deployment never believes a stranger's headers.
func TestTrustedProxies_DefaultExcludesAnArbitraryPeer(t *testing.T) {
	seen := askForwardedView(t, options.DefaultTrustedProxies)

	require.Equal(t, "http", seen.scheme,
		"the default list must not trust a peer outside loopback and the Docker bridge")
	require.NotEqual(t, "203.0.113.9", seen.ip,
		"the default list must not let an arbitrary peer name its client IP")
}
