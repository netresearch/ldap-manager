package web

// HTTP handlers and middleware for authentication and session management.

import (
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
		user, authErr := a.authenticateUser(username, password)
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
				return c.Status(fiber.StatusTooManyRequests).
					SendString("Too many failed login attempts. Please try again later.")
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

		sess.Set("dn", user.DN())
		sess.Set("password", password)
		sess.Set("username", username)
		if err := sess.Save(); err != nil {
			return handle500(c, err)
		}

		log.Info().
			Str("username", username).
			Str("dn", user.DN()).
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

// authenticateUser verifies credentials using either the service account
// (when configured) or direct UPN bind (for AD without service account).
func (a *App) authenticateUser(username, password string) (*ldap.User, error) {
	if a.ldapReadonly != nil {
		// Service account available: use it to look up user and verify password
		return a.ldapReadonly.CheckPasswordForSAMAccountName(username, password)
	}

	// No service account: authenticate via UPN bind (Active Directory)
	return a.authenticateViaUPNBind(username, password)
}

// authenticateViaUPNBind authenticates by binding as user@domain directly.
// Used when no service account (readonly user) is configured.
func (a *App) authenticateViaUPNBind(username, password string) (*ldap.User, error) {
	domain := domainFromBaseDN(a.ldapConfig.BaseDN)
	upn := username + "@" + domain

	// Bind as the user via UPN
	userClient, err := ldap.New(a.ldapConfig, upn, password, a.ldapOpts...)
	if err != nil {
		return nil, err
	}
	defer userClient.Close()

	// Look up user details using the user's own connection
	user, err := userClient.FindUserBySAMAccountName(username)
	if err != nil {
		return nil, err
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
