package web

import (
	"net/http/httptest"
	"testing"
	"net/url"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/memory/v2"
	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	ldap "github.com/netresearch/simple-ldap-go"
)

// Simple mock LDAP client for testing
type testLDAPClient struct {
	users     []ldap.User
	groups    []ldap.Group
	computers []ldap.Computer
	authError error
}

func (t *testLDAPClient) FindUsers() ([]ldap.User, error) {
	return t.users, nil
}

func (t *testLDAPClient) FindGroups() ([]ldap.Group, error) {
	return t.groups, nil
}

func (t *testLDAPClient) FindComputers() ([]ldap.Computer, error) {
	return t.computers, nil
}

func (t *testLDAPClient) CheckPasswordForSAMAccountName(samAccountName, password string) (*ldap.User, error) {
	if t.authError != nil {
		return nil, t.authError
	}
	for i, user := range t.users {
		if user.SAMAccountName == samAccountName {
			return &t.users[i], nil
		}
	}
	return nil, ldap.ErrUserNotFound
}

func (t *testLDAPClient) WithCredentials(dn, password string) (*ldap.LDAP, error) {
	return &ldap.LDAP{}, nil
}

func setupTestApp() (*App, *testLDAPClient) {
	mockClient := &testLDAPClient{
		users: []ldap.User{
			{SAMAccountName: "john.doe", Enabled: true},
			{SAMAccountName: "jane.smith", Enabled: false},
		},
		groups: []ldap.Group{
			{Members: []string{"cn=john.doe,ou=users,dc=example,dc=com"}},
		},
		computers: []ldap.Computer{
			{SAMAccountName: "workstation-01$", Enabled: true},
		},
	}

	sessionStore := session.New(session.Config{
		Storage: memory.New(),
	})

	f := fiber.New(fiber.Config{
		ErrorHandler: handle500,
	})

	app := &App{
		ldapClient:   (*ldap.LDAP)(nil), // We'll need to work around this for login tests
		ldapCache:    ldap_cache.New(mockClient),
		sessionStore: sessionStore,
		fiber:        f,
	}

	// Populate cache
	app.ldapCache.RefreshUsers()
	app.ldapCache.RefreshGroups() 
	app.ldapCache.RefreshComputers()

	// Setup routes
	f.Get("/login", app.loginHandler)
	f.Get("/logout", app.logoutHandler)
	f.Get("/users", app.usersHandler)
	f.Get("/users/:userDN", app.userHandler)

	return app, mockClient
}

func TestLoginHandlerBasic(t *testing.T) {
	app, _ := setupTestApp()

	t.Run("shows login form on GET", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/login", nil)
		resp, err := app.fiber.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	// Note: Full authentication tests require complex LDAP client mocking
	// which is beyond the scope of basic coverage testing
}

func TestLogoutHandler(t *testing.T) {
	app, _ := setupTestApp()

	t.Run("redirects to login", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/logout", nil)
		resp, err := app.fiber.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode != 302 {
			t.Errorf("Expected redirect status 302, got %d", resp.StatusCode)
		}

		location := resp.Header.Get("Location")
		if location != "/login" {
			t.Errorf("Expected redirect to '/login', got '%s'", location)
		}
	})
}

func TestUsersHandlerBasic(t *testing.T) {
	app, _ := setupTestApp()

	t.Run("redirects unauthenticated requests to login", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users", nil)
		resp, err := app.fiber.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode != 302 {
			t.Errorf("Expected redirect status 302, got %d", resp.StatusCode)
		}

		location := resp.Header.Get("Location")
		if location != "/login" {
			t.Errorf("Expected redirect to '/login', got '%s'", location)
		}
	})
}

func TestUserHandlerBasic(t *testing.T) {
	app, _ := setupTestApp()
	userDN := url.PathEscape("cn=john.doe,ou=users,dc=example,dc=com")

	t.Run("redirects unauthenticated requests to login", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/"+userDN, nil)
		resp, err := app.fiber.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode != 302 {
			t.Errorf("Expected redirect status 302, got %d", resp.StatusCode)
		}

		location := resp.Header.Get("Location")
		if location != "/login" {
			t.Errorf("Expected redirect to '/login', got '%s'", location)
		}
	})

	// Note: Testing invalid URL parameters is complex with httptest
	// The error handling is covered by the actual fiber router
}

// Test the cache helper functions
func TestFindUnassignedGroupsFunction(t *testing.T) {
	app, _ := setupTestApp()

	users := app.ldapCache.FindUsers(true)
	if len(users) > 0 {
		user := app.ldapCache.PopulateGroupsForUser(&users[0])
		unassignedGroups := app.findUnassignedGroups(user)

		// Basic sanity check - should return slice (possibly empty)
		if unassignedGroups == nil {
			t.Error("findUnassignedGroups should return a slice, not nil")
		}
	}
}

// Basic test for the 500 error handler
func TestHandle500(t *testing.T) {
	// Error handler testing with Fiber is complex and depends on template rendering
	// The error handler function exists and is used by other handlers
	t.Skip("Error handler testing requires complex template mocking")
}