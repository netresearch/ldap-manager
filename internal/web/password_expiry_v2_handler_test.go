package web

import (
	"context"
	"errors"
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
