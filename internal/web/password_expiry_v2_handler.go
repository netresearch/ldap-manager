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

	if a.ldapReadonly == nil {
		// No service account => no enumeration path. RequireAdmin already
		// returns false here, so this is defence in depth.
		return c.Status(fiber.StatusServiceUnavailable).
			SendString("Password-expiry roster requires a configured service account")
	}

	days := parseWindowDays(c.Query("days"))
	showAll := c.Query("show") == "all"

	ctx := c.UserContext()
	window := time.Duration(days) * 24 * time.Hour

	rows, err := collectExpiryRows(ctx, a.ldapReadonly, window, showAll)
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
// is by deadline ascending, so the most urgent accounts lead. Rows without a
// concrete deadline (must-change, never, unknown) sort after dated ones on the
// expires key, since they have no moment to compare.
func sortExpiryRows(rows []templates.ExpiryRow, key, dir string) {
	less := expiryLess(key)
	sort.SliceStable(rows, func(i, j int) bool {
		if dir == "desc" {
			return less(rows[j], rows[i])
		}

		return less(rows[i], rows[j])
	})
}

// expiryLess returns the ascending comparator for a sort column.
func expiryLess(key string) func(a, b templates.ExpiryRow) bool {
	switch key {
	case "name":
		return func(a, b templates.ExpiryRow) bool {
			return lowerCN(a) < lowerCN(b)
		}
	case "status":
		return func(a, b templates.ExpiryRow) bool {
			return a.Status < b.Status
		}
	default: // "expires"
		return func(a, b templates.ExpiryRow) bool {
			return expiryOrder(a) < expiryOrder(b)
		}
	}
}

// expiryOrder maps a row to a sortable deadline. Dated rows sort by their
// timestamp; undated rows (must-change, never, unknown) sort last, keeping the
// concrete deadlines — the ones an admin acts on — at the top.
func expiryOrder(r templates.ExpiryRow) int64 {
	if r.HasDeadline {
		return r.ExpiresAt
	}

	return 1<<62 - 1
}

func lowerCN(r templates.ExpiryRow) string {
	return strings.ToLower(r.CN)
}
