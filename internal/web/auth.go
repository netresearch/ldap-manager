package web

// HTTP handlers and middleware for authentication and session management.

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"

	"github.com/netresearch/ldap-manager/internal/version"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

func (a *App) logoutHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	if err := sess.Destroy(); err != nil {
		return handle500(c, err)
	}

	return c.Redirect("/login")
}

func (a *App) loginHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	username := c.FormValue("username")
	password := c.FormValue("password")

	if username != "" && password != "" {
		dn, authErr := a.authenticateUser(username, password)
		if authErr != nil {
			// Record failed attempt for rate limiting
			ip := c.IP()
			blocked := a.rateLimiter.RecordAttempt(ip)

			// Log username for security audit trail - intentional per OWASP logging guidelines
			log.Warn().
				Err(authErr).
				Str("username", username).
				Str("ip", ip).
				Int("remaining_attempts", a.rateLimiter.GetRemainingAttempts(ip)).
				Msg("failed login attempt")

			// If blocked after this attempt, return rate limit error
			if blocked {
				c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

				return templates.LoginWithStyles(
					templates.Flashes(templates.ErrorFlash("Too many failed login attempts. Please try again later.")),
					"",
					a.GetCSRFToken(c),
					a.GetStylesPath(),
				).Render(c.UserContext(), c.Response().BodyWriter())
			}

			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

			return templates.LoginWithStyles(
				templates.Flashes(templates.ErrorFlash("Invalid username or password")),
				"",
				a.GetCSRFToken(c),
				a.GetStylesPath(),
			).Render(c.UserContext(), c.Response().BodyWriter())
		}

		// Successful login - reset rate limit counter
		a.rateLimiter.ResetAttempts(c.IP())

		// Regenerate session ID to prevent session fixation attacks
		if err := sess.Regenerate(); err != nil {
			return handle500(c, err)
		}

		sess.Set("dn", dn)
		// Password stored in session for per-user LDAP binding.
		// Mitigated by: session-only cookies (HttpOnly, SameSite=Strict),
		// configurable session TTL, and server-side session storage.
		sess.Set("password", password)
		sess.Set("username", username)
		if err := sess.Save(); err != nil {
			return handle500(c, err)
		}

		log.Info().
			Str("username", username).
			Str("dn", dn).
			Msg("successful login")

		return c.Redirect("/")
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.LoginWithStyles(
		templates.Flashes(),
		version.FormatVersion(),
		a.GetCSRFToken(c),
		a.GetStylesPath(),
	).Render(c.UserContext(), c.Response().BodyWriter())
}

// authenticateUser verifies credentials and returns the user's DN.
// Tries service account lookup first, then UPN bind (AD), then direct bind
// as fallback for non-person accounts (e.g. OpenLDAP admin).
func (a *App) authenticateUser(username, password string) (string, error) {
	if a.ldapReadonly != nil {
		user, err := a.ldapReadonly.CheckPasswordForSAMAccountName(username, password)
		if err == nil {
			return user.DN(), nil
		}

		// SAMAccountName lookup failed — try direct bind as fallback.
		// This handles accounts like cn=admin that aren't person entries.
		dn, bindErr := a.authenticateViaDirectBind(username, password)
		if bindErr == nil {
			return dn, nil
		}

		// Return the original error (more informative than direct bind error)
		return "", err
	}

	// No service account: authenticate via UPN bind (Active Directory)
	user, err := a.authenticateViaUPNBind(username, password)
	if err == nil {
		return user.DN(), nil
	}

	// UPN bind failed — try direct bind as fallback
	dn, bindErr := a.authenticateViaDirectBind(username, password)
	if bindErr == nil {
		return dn, nil
	}

	return "", err
}

// authenticateViaDirectBind tries binding directly with common DN patterns.
// This handles non-person accounts like the OpenLDAP root admin (cn=admin,dc=...).
func (a *App) authenticateViaDirectBind(username, password string) (string, error) {
	// Validate username to prevent LDAP injection
	if strings.ContainsAny(username, `\@,=+"<>#;*()`) || strings.ContainsRune(username, 0) {
		return "", fmt.Errorf("invalid characters in username")
	}

	// Try common DN patterns
	candidates := []string{
		fmt.Sprintf("cn=%s,%s", username, a.ldapConfig.BaseDN),
		fmt.Sprintf("uid=%s,%s", username, a.ldapConfig.BaseDN),
	}

	for _, dn := range candidates {
		client, err := ldap.New(a.ldapConfig, dn, password, a.ldapOpts...)
		if err != nil {
			continue
		}
		_ = client.Close()

		log.Info().
			Str("username", username).
			Str("dn", dn).
			Msg("authenticated via direct bind")

		return dn, nil
	}

	return "", fmt.Errorf("direct bind failed for %s", username)
}

// authenticateViaUPNBind authenticates by binding as user@domain directly.
// Used when no service account is configured.
func (a *App) authenticateViaUPNBind(username, password string) (*ldap.User, error) {
	// Validate username to prevent LDAP injection in UPN construction.
	// Blocks LDAP DN special chars and filter metacharacters.
	if strings.ContainsAny(username, `\@,=+"<>#;*()`) || strings.ContainsRune(username, 0) {
		return nil, fmt.Errorf("invalid characters in username")
	}

	domain := domainFromBaseDN(a.ldapConfig.BaseDN)
	if domain == "" {
		return nil, fmt.Errorf("cannot derive domain from BaseDN %q: no DC components found", a.ldapConfig.BaseDN)
	}

	upn := username + "@" + domain

	// Bind as the user via UPN
	userClient, err := ldap.New(a.ldapConfig, upn, password, a.ldapOpts...)
	if err != nil {
		return nil, fmt.Errorf("UPN bind failed: %w", err)
	}
	defer func() { _ = userClient.Close() }()

	// Look up user details using the user's own connection
	user, err := userClient.FindUserBySAMAccountName(username)
	if err != nil {
		return nil, fmt.Errorf("user lookup after UPN bind: %w", err)
	}

	return user, nil
}

// domainFromBaseDN derives a DNS domain from an LDAP BaseDN.
// Example: "DC=example,DC=com" → "example.com"
func domainFromBaseDN(baseDN string) string {
	parts := strings.Split(baseDN, ",")
	domains := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		upper := strings.ToUpper(part)

		if strings.HasPrefix(upper, "DC=") {
			domains = append(domains, part[3:])
		}
	}

	return strings.Join(domains, ".")
}
