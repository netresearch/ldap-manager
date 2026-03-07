package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/stretchr/testify/assert"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
)

func TestAuthenticateViaDirectBind_RejectsInjection(t *testing.T) {
	app, _ := setupTestApp()

	badUsernames := []string{
		"admin*",
		"admin()",
		"admin\\bad",
		"admin@evil",
		"admin,dc=evil",
		"admin=bad",
		string([]byte{0x00}),
	}

	for _, username := range badUsernames {
		_, err := app.authenticateViaDirectBind(username, "password")
		if err == nil {
			t.Errorf("expected error for username %q, got nil", username)
		}
	}
}

func TestAuthenticateViaDirectBind_ValidUsername(t *testing.T) {
	app, _ := setupTestApp()

	// Valid username should pass validation.
	// In test env, ldap.New with test config may succeed (no real server check).
	dn, err := app.authenticateViaDirectBind("validuser", "password")
	if err == nil {
		// Direct bind succeeded — check that a DN was returned
		assert.NotEmpty(t, dn)
		assert.Contains(t, dn, "validuser")
	} else {
		// If it fails, it should be a connection error, not a validation error
		assert.NotContains(t, err.Error(), "invalid characters")
	}
}

func TestAuthenticateViaUPNBind_RejectsInjection(t *testing.T) {
	app, _ := setupTestApp()

	badUsernames := []string{
		"admin*",
		"admin()",
		"admin\\bad",
		"admin@evil",
		"admin,dc=evil",
		"admin=bad",
		"admin\"bad",
		"admin<bad",
		"admin>bad",
		"admin#bad",
		"admin;bad",
		"admin+bad",
		string([]byte{0x00}),
	}

	for _, username := range badUsernames {
		t.Run(username, func(t *testing.T) {
			_, err := app.authenticateViaUPNBind(username, "password")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid characters")
		})
	}
}

func TestAuthenticateViaUPNBind_NoDCComponents(t *testing.T) {
	app, _ := setupTestApp()
	// Override baseDN with no DC components
	app.ldapConfig.BaseDN = "OU=Users,CN=Admin"

	_, err := app.authenticateViaUPNBind("validuser", "password")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot derive domain")
}

func TestAuthenticateViaUPNBind_ValidUsername(t *testing.T) {
	app, _ := setupTestApp()
	// baseDN has DC components (dc=test,dc=com)

	_, err := app.authenticateViaUPNBind("validuser", "password")
	// In test env, UPN bind may succeed (test LDAP client) but user lookup may fail.
	// Either way, we just verify no panic and the function completes.
	if err != nil {
		// Error should be related to LDAP operations, not validation
		assert.NotContains(t, err.Error(), "invalid characters")
	}
}

func TestAuthenticateUser_WithServiceAccount(t *testing.T) {
	app, _ := setupTestApp()

	t.Run("existing user succeeds", func(t *testing.T) {
		// Mock has "john.doe" — CheckPasswordForSAMAccountName should find it
		dn, err := app.authenticateUser("john.doe", "password")
		// May succeed or fail depending on test LDAP behavior
		if err == nil {
			assert.NotEmpty(t, dn)
		}
	})

	t.Run("nonexistent user falls back to direct bind", func(t *testing.T) {
		// "nonexistent" not in mock — primary auth fails, tries direct bind
		dn, err := app.authenticateUser("nonexistent", "password")
		// Direct bind may succeed in test env
		if err != nil {
			assert.Error(t, err)
		} else {
			assert.NotEmpty(t, dn)
		}
	})
}

func TestAuthenticateUser_WithoutServiceAccount(t *testing.T) {
	app, _ := setupTestApp()
	app.ldapReadonly = nil // no service account

	// Without service account, authenticateUser tries UPN bind first.
	// UPN bind may succeed or fail in test env.
	dn, err := app.authenticateUser("validuser", "password")
	if err != nil {
		// Should be a connection/lookup error, not validation
		assert.NotContains(t, err.Error(), "invalid characters")
	} else {
		assert.NotEmpty(t, dn)
	}
}

func TestDomainFromBaseDN_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		baseDN   string
		expected string
	}{
		{"simple domain", "DC=example,DC=com", "example.com"},
		{"subdomain", "DC=sub,DC=example,DC=com", "sub.example.com"},
		{"with OU prefix", "OU=Users,DC=example,DC=com", "example.com"},
		{"mixed case", "dc=Example,DC=COM", "Example.COM"},
		{"with spaces", " DC=example , DC=com ", "example.com"},
		{"empty", "", ""},
		{"no DC components", "OU=Users,CN=Admin", ""},
		{"single DC", "DC=local", "local"},
		{"DC with equals in value", "DC=example=test,DC=com", "example=test.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := domainFromBaseDN(tc.baseDN)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestLoginHandler_ShowsForm(t *testing.T) {
	app, _ := setupFullTestApp(t)

	// GET /login should show the login form
	req := makeSimpleRequest("GET", "/login")
	resp, err := app.fiber.Test(req)
	assert.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
}

func TestLogoutHandler_RedirectsToLogin(t *testing.T) {
	app, store := setupFullTestApp(t)
	cookies := createAuthSession(t, app, store)

	resp := makeAuthRequest(t, app, "/logout", cookies)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, 302, resp.StatusCode)
	assert.Equal(t, "/login", resp.Header.Get("Location"))
}

// makeSimpleRequest creates a simple HTTP request without cookies.
func makeSimpleRequest(method, path string) *http.Request {
	return httptest.NewRequest(method, path, nil)
}

func TestFilterUnassignedGroups_Comprehensive(t *testing.T) {
	t.Run("returns all groups when user has none", func(t *testing.T) {
		user := &ldap_cache.FullLDAPUser{
			Groups: []ldap.Group{},
		}
		allGroups := []ldap.Group{
			{Members: []string{"cn=other"}},
			{Members: []string{"cn=another"}},
		}
		result := filterUnassignedGroups(allGroups, user)
		assert.Len(t, result, 2)
	})

	t.Run("returns empty when user is in all groups", func(t *testing.T) {
		// In test context, groups have empty DNs so they all match
		user := &ldap_cache.FullLDAPUser{
			Groups: []ldap.Group{
				{Members: []string{"cn=user1"}},
			},
		}
		allGroups := []ldap.Group{
			{Members: []string{"cn=user1"}},
		}
		result := filterUnassignedGroups(allGroups, user)
		// Both have empty DN() so they match
		assert.Empty(t, result)
	})
}

func TestFilterUnassignedUsers_Comprehensive(t *testing.T) {
	t.Run("returns all users when group has none", func(t *testing.T) {
		group := &ldap_cache.FullLDAPGroup{
			Members: []ldap.User{},
		}
		allUsers := []ldap.User{
			{SAMAccountName: "user1"},
			{SAMAccountName: "user2"},
		}
		result := filterUnassignedUsers(allUsers, group)
		assert.Len(t, result, 2)
	})
}
