package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// fakeExpiryResolver stands in for the LDAP client so the collection logic can
// be tested without a directory.
type fakeExpiryResolver struct {
	expiring    []ldap.ExpiringUser
	expiringErr error
	users       []ldap.User
	usersErr    error
	perUser     map[string]ldap.PasswordExpiry
	perUserErr  error
}

func (f *fakeExpiryResolver) UsersWithExpiringPasswords(_ context.Context, _ time.Duration) ([]ldap.ExpiringUser, error) {
	return f.expiring, f.expiringErr
}

func (f *fakeExpiryResolver) FindUsersContext(_ context.Context) ([]ldap.User, error) {
	return f.users, f.usersErr
}

func (f *fakeExpiryResolver) PasswordExpiryFor(_ context.Context, user *ldap.User) (ldap.PasswordExpiry, error) {
	if f.perUserErr != nil {
		return ldap.PasswordExpiry{}, f.perUserErr
	}

	return f.perUser[user.SAMAccountName], nil
}

func userWith(sam string, enabled bool) ldap.User {
	u := ldap.User{SAMAccountName: sam, Enabled: enabled}

	return u
}

func TestCollectExpiryRows_DueOnlyUsesTheLibraryFilter(t *testing.T) {
	soon := ldap.PasswordExpiry{Status: ldap.PasswordExpires, At: time.Now().Add(48 * time.Hour)}
	must := ldap.PasswordExpiry{Status: ldap.PasswordMustChange}

	resolver := &fakeExpiryResolver{
		expiring: []ldap.ExpiringUser{
			{User: ptr(userWith("alice", true)), Expiry: soon},
			{User: ptr(userWith("bob", true)), Expiry: must},
		},
	}

	rows, err := collectExpiryRows(context.Background(), resolver, 30*24*time.Hour, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if !rows[0].HasDeadline || rows[0].Status != "expires" {
		t.Errorf("row0 = %+v, want a dated expires row", rows[0])
	}
	if rows[1].HasDeadline || rows[1].Status != "must-change" {
		t.Errorf("row1 = %+v, want an undated must-change row", rows[1])
	}
}

func TestCollectExpiryRows_ShowAllEnumeratesAndSkipsDisabled(t *testing.T) {
	resolver := &fakeExpiryResolver{
		users: []ldap.User{
			userWith("alice", true),
			userWith("ghost", false), // disabled: must be skipped before resolving
			userWith("carol", true),
		},
		perUser: map[string]ldap.PasswordExpiry{
			"alice": {Status: ldap.PasswordNeverExpires},
			"carol": {Status: ldap.PasswordExpiryUnknown},
		},
	}

	rows, err := collectExpiryRows(context.Background(), resolver, 30*24*time.Hour, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (disabled skipped)", len(rows))
	}
	for _, r := range rows {
		if r.SAMAccountName == "ghost" {
			t.Error("a disabled account must not appear in the show-all roster")
		}
	}
}

func TestCollectExpiryRows_ErrorsPropagate(t *testing.T) {
	sentinel := errors.New("ldap down")

	t.Run("due-only", func(t *testing.T) {
		_, err := collectExpiryRows(context.Background(),
			&fakeExpiryResolver{expiringErr: sentinel}, time.Hour, false)
		if !errors.Is(err, sentinel) {
			t.Fatalf("err = %v, want the resolver error", err)
		}
	})

	t.Run("show-all enumeration", func(t *testing.T) {
		_, err := collectExpiryRows(context.Background(),
			&fakeExpiryResolver{usersErr: sentinel}, time.Hour, true)
		if !errors.Is(err, sentinel) {
			t.Fatalf("err = %v, want the resolver error", err)
		}
	})

	t.Run("show-all per-user", func(t *testing.T) {
		_, err := collectExpiryRows(context.Background(),
			&fakeExpiryResolver{users: []ldap.User{userWith("a", true)}, perUserErr: sentinel}, time.Hour, true)
		if !errors.Is(err, sentinel) {
			t.Fatalf("err = %v, want the resolver error", err)
		}
	})
}

func TestNewExpiryRow_FourStates(t *testing.T) {
	tests := []struct {
		name        string
		expiry      ldap.PasswordExpiry
		wantStatus  string
		wantDated   bool
		wantExpired bool
	}{
		{"expires future", ldap.PasswordExpiry{Status: ldap.PasswordExpires, At: time.Now().Add(time.Hour)}, "expires", true, false},
		{"expires past", ldap.PasswordExpiry{Status: ldap.PasswordExpires, At: time.Now().Add(-time.Hour)}, "expires", true, true},
		{"must change", ldap.PasswordExpiry{Status: ldap.PasswordMustChange}, "must-change", false, true},
		{"never", ldap.PasswordExpiry{Status: ldap.PasswordNeverExpires}, "never-expires", false, false},
		{"unknown", ldap.PasswordExpiry{Status: ldap.PasswordExpiryUnknown}, "unknown", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := newExpiryRow(ptr(userWith("sam", true)), tt.expiry)
			if row.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", row.Status, tt.wantStatus)
			}
			if row.HasDeadline != tt.wantDated {
				t.Errorf("HasDeadline = %v, want %v", row.HasDeadline, tt.wantDated)
			}
			if row.Expired != tt.wantExpired {
				t.Errorf("Expired = %v, want %v", row.Expired, tt.wantExpired)
			}
		})
	}
}

func TestParseWindowDays(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 30},
		{"7", 7},
		{"0", 30},
		{"-5", 30},
		{"notanumber", 30},
		{"999", 366},
		{"366", 366},
	}

	for _, tt := range tests {
		if got := parseWindowDays(tt.in); got != tt.want {
			t.Errorf("parseWindowDays(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestSortExpiryRows_ByDeadlineUndatedLast(t *testing.T) {
	rows := []templates.ExpiryRow{
		{CN: "z-never", Status: "never-expires"},
		{CN: "late", Status: "expires", HasDeadline: true, ExpiresAt: 2000},
		{CN: "soon", Status: "expires", HasDeadline: true, ExpiresAt: 1000},
	}

	sortExpiryRows(rows, "expires", "asc")

	want := []string{"soon", "late", "z-never"}
	for i, cn := range want {
		if rows[i].CN != cn {
			t.Errorf("position %d = %q, want %q", i, rows[i].CN, cn)
		}
	}
}

// Undated rows must stay at the bottom under BOTH directions — reversing the
// deadline sort reverses only the dated rows, it must not float must-change /
// never / unknown accounts to the top.
func TestSortExpiryRows_UndatedStayLastUnderDesc(t *testing.T) {
	rows := []templates.ExpiryRow{
		{CN: "must", Status: "must-change"},
		{CN: "late", Status: "expires", HasDeadline: true, ExpiresAt: 2000},
		{CN: "soon", Status: "expires", HasDeadline: true, ExpiresAt: 1000},
		{CN: "never", Status: "never-expires"},
	}

	sortExpiryRows(rows, "expires", "desc")

	// Dated rows first, newest deadline leading; undated rows keep their order
	// at the bottom.
	want := []string{"late", "soon", "must", "never"}
	for i, cn := range want {
		if rows[i].CN != cn {
			t.Errorf("position %d = %q, want %q", i, rows[i].CN, cn)
		}
	}
}

func TestSortExpiryRows_ByStatus(t *testing.T) {
	rows := []templates.ExpiryRow{
		{CN: "c", Status: "unknown"},
		{CN: "a", Status: "expires"},
		{CN: "b", Status: "must-change"},
	}

	sortExpiryRows(rows, "status", "asc")

	// Alphabetical by status string: expires < must-change < unknown.
	want := []string{"expires", "must-change", "unknown"}
	for i, s := range want {
		if rows[i].Status != s {
			t.Errorf("position %d status = %q, want %q", i, rows[i].Status, s)
		}
	}

	sortExpiryRows(rows, "status", "desc")
	if rows[0].Status != "unknown" {
		t.Errorf("desc first = %q, want unknown", rows[0].Status)
	}
}

func TestSortExpiryRows_NameDescending(t *testing.T) {
	rows := []templates.ExpiryRow{
		{CN: "Alice"}, {CN: "carol"}, {CN: "Bob"},
	}

	sortExpiryRows(rows, "name", "desc")

	want := []string{"carol", "Bob", "Alice"}
	for i, cn := range want {
		if rows[i].CN != cn {
			t.Errorf("position %d = %q, want %q", i, rows[i].CN, cn)
		}
	}
}

func ptr[T any](v T) *T { return &v }

// The injected admin check lets a request reach the handler behind
// RequireAdmin — otherwise unreachable, since a DN-addressable admin cache
// entry cannot be built outside the LDAP package.
func adminApp(t *testing.T) (*App, []*http.Cookie) {
	t.Helper()

	app, _ := setupFullTestApp(t)
	app.adminCheck = func(string) bool { return true }
	cookies := simulatedSession(t, app)

	return app, cookies
}

func getExpiry(t *testing.T, app *App, cookies []*http.Cookie, target string) *http.Response {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, target, nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	resp, err := app.fiber.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	return resp
}

// With no service account the roster has no enumeration path; the handler
// returns 503 rather than 500 or a blank page.
func TestHandlePasswordExpiry_NoServiceAccountIs503(t *testing.T) {
	app, cookies := adminApp(t)
	app.ldapReadonly = nil
	app.expiryResolver = nil

	resp := getExpiry(t, app, cookies, "/password-expiry")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}

// An admin reaches the handler and gets the rendered roster. This also
// exercises the RequireAdmin admit path and the days/show/sort wiring.
func TestHandlePasswordExpiry_AdminGetsRoster(t *testing.T) {
	app, cookies := adminApp(t)
	soon := ldap.PasswordExpiry{Status: ldap.PasswordExpires, At: time.Now().Add(48 * time.Hour)}
	app.expiryResolver = &fakeExpiryResolver{
		expiring: []ldap.ExpiringUser{{User: ptr(userWith("alice", true)), Expiry: soon}},
	}

	resp := getExpiry(t, app, cookies, "/password-expiry?days=14")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	if !strings.Contains(html, "alice") {
		t.Error("roster should render the expiring account")
	}
	if !strings.Contains(html, "expiring within 14 days") {
		t.Errorf("count label should reflect ?days=14, got body without it")
	}
}

// show=all takes the enumeration path and includes never/unknown rows.
func TestHandlePasswordExpiry_ShowAllIncludesEveryState(t *testing.T) {
	app, cookies := adminApp(t)
	app.expiryResolver = &fakeExpiryResolver{
		users: []ldap.User{userWith("never", true), userWith("quiet", true)},
		perUser: map[string]ldap.PasswordExpiry{
			"never": {Status: ldap.PasswordNeverExpires},
			"quiet": {Status: ldap.PasswordExpiryUnknown},
		},
	}

	resp := getExpiry(t, app, cookies, "/password-expiry?show=all")
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	if !strings.Contains(html, "Never expires") || !strings.Contains(html, "Unknown") {
		t.Error("show=all should render never-expires and unknown rows")
	}
}

// A resolver error surfaces as a server error, not a blank 200.
func TestHandlePasswordExpiry_ResolverErrorIsHandled(t *testing.T) {
	app, cookies := adminApp(t)
	app.expiryResolver = &fakeExpiryResolver{expiringErr: errors.New("ldap down")}

	resp := getExpiry(t, app, cookies, "/password-expiry")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		t.Errorf("status = %d, want a non-200 error for a resolver failure", resp.StatusCode)
	}
}
