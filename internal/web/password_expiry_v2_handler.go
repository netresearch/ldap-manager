package web

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// defaultExpiryWindowDays is the "expiring soon" horizon when no ?days= is
// given. Thirty days is the common first reminder threshold.
const defaultExpiryWindowDays = 30

// maxExpiryWindowDays caps ?days= so a hostile or fat-fingered value cannot ask
// the directory for an unbounded window. A year is well beyond any reminder
// cadence.
const maxExpiryWindowDays = 366

// handlePasswordExpiryV2 renders the admin-only roster of accounts whose LDAP
// password is expiring. It is gated by RequireAdmin and needs the service
// account: expiry is resolved live from the directory, not the cache, because
// the cache cannot compute it (it never holds the domain max-age) and because a
// security roster wants current truth rather than a 30-second-stale snapshot.
//
// Query parameters:
//   - days=N   the "expiring soon" window (default 30, capped at 366)
//   - show=all include every account with its status, not only the ones due
//   - sort/dir column sort, matching the users table
func (a *App) handlePasswordExpiryV2(c *fiber.Ctx) error {
	viewerDN, handled, res := a.resolveViewerDN(c)
	if handled {
		return res
	}

	resolver := a.effectiveExpiryResolver()
	if resolver == nil {
		// No service account => no enumeration path. RequireAdmin already
		// returns false here, so this is defence in depth.
		return c.Status(fiber.StatusServiceUnavailable).
			SendString("Password-expiry roster requires a configured service account")
	}

	days := parseWindowDays(c.Query("days"))
	showAll := c.Query("show") == "all"

	ctx := c.UserContext()
	window := time.Duration(days) * 24 * time.Hour

	rows, err := collectExpiryRows(ctx, resolver, window, showAll)
	if err != nil {
		return handle500(c, err)
	}

	sortKey := c.Query("sort", "expires")
	sortDir := normaliseSortDir(c.Query("dir", "asc"))
	sortExpiryRows(rows, sortKey, sortDir)

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	page := templates.PasswordExpiryV2(rows, days, showAll, sortKey, sortDir,
		a.takeFlash(c), a.paletteContextFor(viewerDN))

	return page.Render(c.UserContext(), c.Response().BodyWriter())
}

// effectiveExpiryResolver returns the roster's data source: the injected test
// resolver, else the service-account client, else nil when neither is set.
// Returning the concrete *ldap.LDAP only when non-nil avoids the typed-nil
// interface trap — a nil *ldap.LDAP boxed into the interface would not compare
// equal to nil and would panic on first use.
func (a *App) effectiveExpiryResolver() expiryResolver {
	if a.expiryResolver != nil {
		return a.expiryResolver
	}
	if a.ldapReadonly != nil {
		return a.ldapReadonly
	}

	return nil
}

// expiryResolver is the slice of the LDAP client the roster needs. Depending
// on an interface rather than *ldap.LDAP lets the row-collection logic — the
// due/show-all split, the disabled-skip, error propagation — be tested with a
// fake instead of a live directory. *ldap.LDAP satisfies it.
type expiryResolver interface {
	UsersWithExpiringPasswords(ctx context.Context, within time.Duration) ([]ldap.ExpiringUser, error)
	FindUsersContext(ctx context.Context) ([]ldap.User, error)
	PasswordExpiryFor(ctx context.Context, user *ldap.User) (ldap.PasswordExpiry, error)
}

// collectExpiryRows resolves the rows to display.
//
// The default view asks the library for exactly the due set —
// UsersWithExpiringPasswords already filters to enabled accounts expiring
// within the window, must-change included, never/unknown excluded. The
// show-all view instead enumerates every account and resolves each status, so
// never-expires and unknown rows can be shown muted.
func collectExpiryRows(ctx context.Context, resolver expiryResolver, window time.Duration, showAll bool) ([]templates.ExpiryRow, error) {
	if !showAll {
		expiring, err := resolver.UsersWithExpiringPasswords(ctx, window)
		if err != nil {
			return nil, err
		}

		return toExpiryRows(expiring), nil
	}

	users, err := resolver.FindUsersContext(ctx)
	if err != nil {
		return nil, err
	}

	rows := make([]templates.ExpiryRow, 0, len(users))
	for i := range users {
		user := &users[i]
		if !user.Enabled {
			continue
		}

		expiry, err := resolver.PasswordExpiryFor(ctx, user)
		if err != nil {
			return nil, err
		}

		rows = append(rows, newExpiryRow(user, expiry))
	}

	return rows, nil
}

// toExpiryRows adapts the library's ExpiringUser slice to the view model.
func toExpiryRows(expiring []ldap.ExpiringUser) []templates.ExpiryRow {
	rows := make([]templates.ExpiryRow, 0, len(expiring))
	for i := range expiring {
		rows = append(rows, newExpiryRow(expiring[i].User, expiring[i].Expiry))
	}

	return rows
}

// newExpiryRow builds a single view-model row from a user and its resolved
// expiry. The status string comes from the library enum so the template and
// the API cannot disagree on the label.
func newExpiryRow(user *ldap.User, expiry ldap.PasswordExpiry) templates.ExpiryRow {
	row := templates.ExpiryRow{
		CN:             user.CN(),
		SAMAccountName: user.SAMAccountName,
		DN:             user.DN(),
		Status:         expiry.Status.String(),
		Expired:        expiry.Expired(time.Now()),
	}
	if expiry.Status == ldap.PasswordExpires {
		row.ExpiresAt = expiry.At.Unix()
		row.HasDeadline = true
	}

	return row
}

// parseWindowDays reads the ?days= window, falling back to the default and
// clamping to a sane range. A non-numeric or non-positive value is treated as
// unset rather than an error — this is a display filter, not an operation.
func parseWindowDays(raw string) int {
	if raw == "" {
		return defaultExpiryWindowDays
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultExpiryWindowDays
	}
	if n > maxExpiryWindowDays {
		return maxExpiryWindowDays
	}

	return n
}

// sortExpiryRows orders the rows in place by the requested column. The default
// is by deadline ascending, so the most urgent accounts lead.
//
// On the deadline column, rows without a concrete date (must-change, never,
// unknown) always sort to the bottom regardless of direction — reversing the
// direction reverses only the dated rows, keeping the undated ones out of the
// way rather than letting them jump to the top under desc.
func sortExpiryRows(rows []templates.ExpiryRow, key, dir string) {
	if key != "name" && key != "status" {
		sortByDeadline(rows, dir)

		return
	}

	less := expiryLess(key)
	sort.SliceStable(rows, func(i, j int) bool {
		if dir == "desc" {
			return less(rows[j], rows[i])
		}

		return less(rows[i], rows[j])
	})
}

// sortByDeadline sorts dated rows by their deadline in the given direction and
// pins undated rows to the bottom in both directions.
func sortByDeadline(rows []templates.ExpiryRow, dir string) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.HasDeadline != b.HasDeadline {
			// The dated row always precedes the undated one.
			return a.HasDeadline
		}
		if !a.HasDeadline {
			return false // both undated: keep stable order
		}
		if dir == "desc" {
			return a.ExpiresAt > b.ExpiresAt
		}

		return a.ExpiresAt < b.ExpiresAt
	})
}

// expiryLess returns the ascending comparator for the non-deadline columns.
// The deadline column is handled by sortByDeadline, which needs direction- and
// undated-aware ordering the plain comparator cannot express.
func expiryLess(key string) func(a, b templates.ExpiryRow) bool {
	if key == "status" {
		return func(a, b templates.ExpiryRow) bool {
			return a.Status < b.Status
		}
	}

	// "name"
	return func(a, b templates.ExpiryRow) bool {
		return strings.ToLower(a.CN) < strings.ToLower(b.CN)
	}
}
