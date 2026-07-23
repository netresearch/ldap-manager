package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	ldap "github.com/netresearch/simple-ldap-go"
)

func TestUserIsAdmin(t *testing.T) {
	const adminGroup = "cn=admins,ou=groups,dc=example,dc=com"

	tests := []struct {
		name       string
		adminGroup string
		user       ldap.User
		want       bool
	}{
		{
			name: "adminCount marks an admin regardless of group config",
			user: ldap.User{AdminCount: true},
			want: true,
		},
		{
			name:       "member of the configured admin group",
			adminGroup: adminGroup,
			user:       ldap.User{Groups: []string{adminGroup}},
			want:       true,
		},
		{
			name:       "membership match is case-insensitive",
			adminGroup: adminGroup,
			user:       ldap.User{Groups: []string{"CN=Admins,OU=Groups,DC=example,DC=com"}},
			want:       true,
		},
		{
			name:       "not a member and no adminCount",
			adminGroup: adminGroup,
			user:       ldap.User{Groups: []string{"cn=other,ou=groups,dc=example,dc=com"}},
			want:       false,
		},
		{
			name: "no admin group configured and no adminCount: nobody is admin (OpenLDAP lockout)",
			user: ldap.User{Groups: []string{adminGroup}},
			want: false,
		},
		{
			// The adminGroupDN != "" guard exists for exactly this: a directory
			// that returns an empty-string group entry must not match an unset
			// admin group, since IsMemberOf("") would otherwise be true.
			name: "empty group entry with no admin group configured is not admin",
			user: ldap.User{Groups: []string{""}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &App{adminGroupDN: tt.adminGroup}
			if got := a.userIsAdmin(&tt.user); got != tt.want {
				t.Errorf("userIsAdmin() = %v, want %v", got, tt.want)
			}
		})
	}
}

// isAdmin must be false when there is no cache — the roster needs the service
// account, and without it there is no way to resolve membership.
func TestIsAdmin_NilCacheIsNotAdmin(t *testing.T) {
	a := &App{adminGroupDN: "cn=admins,dc=example,dc=com"}
	if a.isAdmin("cn=someone,dc=example,dc=com") {
		t.Error("isAdmin must be false without a cache")
	}
}

// RequireAdmin returns 403 for an authenticated non-admin. The admit path is
// covered by TestUserIsAdmin: a DN-addressable admin cache entry cannot be
// constructed here because User.DN is not settable outside the LDAP package,
// so the full-app path can only exercise the deny side.
func TestRequireAdmin_ForbidsNonAdmin(t *testing.T) {
	app, _ := setupFullTestApp(t)

	cookies := simulatedSession(t, app)

	req := httptest.NewRequest(http.MethodGet, "/password-expiry", nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}

	resp, err := app.fiber.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for a non-admin viewer", resp.StatusCode)
	}
}
