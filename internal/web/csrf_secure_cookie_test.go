package web

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ldap-manager/internal/options"
)

// hintFragment is the part of the warning that names the setting. Matching on
// it (rather than on the whole sentence) keeps the assertion stable against
// rewording while still failing if the hint stops naming COOKIE_SECURE.
const hintFragment = "COOKIE_SECURE=true"

// csrfHintOpts builds the minimal options the CSRF middleware reads.
func csrfHintOpts(cookieSecure bool) *options.Opts {
	return &options.Opts{
		LDAP: ldap.Config{
			Server: "ldap://127.0.0.1:389",
			BaseDN: "dc=test,dc=local",
		},
		SessionDuration: 30 * time.Minute,
		CookieSecure:    cookieSecure,
	}
}

// postRejectedByCSRF drives one POST through the real CSRF handler built by
// createCSRFConfig and returns everything the operator would see in the log.
// The request always fails validation, so the ErrorHandler always runs; the
// arms differ only in the three inputs the hint predicate reads.
//
// The app trusts every peer so that X-Forwarded-Proto decides Scheme(); the
// production trust list lives in createFiberApp and is not under test here.
func postRejectedByCSRF(t *testing.T, cookieSecure bool, header, value string) string {
	t.Helper()

	opts := csrfHintOpts(cookieSecure)
	f := fiber.New(fiber.Config{
		TrustProxy:       true,
		TrustProxyConfig: fiber.TrustProxyConfig{Proxies: []string{"0.0.0.0/0"}},
	})
	f.Post("/guarded", createCSRFConfig(opts, createSessionStore(opts)), func(c fiber.Ctx) error {
		return c.SendString("reached the handler")
	})

	var buf bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = previous })

	req := httptest.NewRequest("POST", "/guarded", strings.NewReader("csrf_token=not-a-valid-token"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if header != "" {
		req.Header.Set(header, value)
	}

	resp, err := f.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode,
		"the request must be rejected, otherwise the ErrorHandler never runs")
	require.NoError(t, resp.Body.Close())

	logged := buf.String()
	require.Contains(t, logged, "CSRF validation failed",
		"the baseline rejection must be logged in every arm")

	return logged
}

// TestCSRFHintNamesCookieSecure pins the diagnostic added for issue #678: a
// browser on plain HTTP discards the Secure cookies, so the form token can
// never match and the bare "csrf: token invalid" names the wrong cause.
func TestCSRFHintNamesCookieSecure(t *testing.T) {
	logged := postRejectedByCSRF(t, true, "", "")

	assert.Contains(t, logged, hintFragment,
		"a rejected plain-HTTP request without a CSRF cookie must name COOKIE_SECURE")
}

// TestCSRFHintSilentWhenCookiesAreNotSecure covers the deployment that already
// runs on plain HTTP by choice. Nothing is misconfigured, so nothing is said.
func TestCSRFHintSilentWhenCookiesAreNotSecure(t *testing.T) {
	logged := postRejectedByCSRF(t, false, "", "")

	assert.NotContains(t, logged, hintFragment,
		"COOKIE_SECURE=false is the documented plain-HTTP setup and needs no hint")
}

// TestCSRFHintSilentOverHTTPS covers a request that reached us as HTTPS. The
// Secure attribute is satisfied, so a missing cookie has some other cause.
func TestCSRFHintSilentOverHTTPS(t *testing.T) {
	logged := postRejectedByCSRF(t, true, "X-Forwarded-Proto", "https")

	assert.NotContains(t, logged, hintFragment,
		"an HTTPS request must not be blamed on the Secure attribute")
}

// TestCSRFHintSilentWhenCookieArrives covers the loopback browser: the scheme
// is http, and the cookie is sent all the same because loopback is a secure
// context. This arm is why the predicate tests the cookie and not the scheme.
func TestCSRFHintSilentWhenCookieArrives(t *testing.T) {
	logged := postRejectedByCSRF(t, true, "Cookie", csrfCookieName+"=a-cookie-that-was-sent")

	assert.NotContains(t, logged, hintFragment,
		"the browser did send the cookie, so the Secure attribute is not the cause")
}
