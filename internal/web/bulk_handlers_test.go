package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// TestBulkHandler_UnknownAction verifies unknown bulk actions return 400
// without requiring an LDAP connection.
func TestBulkHandler_UnknownAction(t *testing.T) {
	app, store := setupFullTestApp(t)

	cookies := createAuthSession(t, app, store)

	req := httptest.NewRequest(http.MethodPost, "/users/bulk?action=wat",
		strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := app.fiber.Test(req)
	if err != nil {
		t.Fatalf("bulk POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestBulkHandler_MissingGroup verifies add-to-group without a group_dn is
// rejected as 400.
func TestBulkHandler_MissingGroup(t *testing.T) {
	app, store := setupFullTestApp(t)

	cookies := createAuthSession(t, app, store)

	form := url.Values{"target_dn": {"cn=a,dc=test", "cn=b,dc=test"}}
	req := httptest.NewRequest(http.MethodPost,
		"/users/bulk?action=add-to-group",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := app.fiber.Test(req)
	if err != nil {
		t.Fatalf("bulk POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestBulkHandler_EmptyTargets verifies a well-formed request with no
// target_dn values redirects back to /users without errors.
func TestBulkHandler_EmptyTargets(t *testing.T) {
	app, store := setupFullTestApp(t)

	cookies := createAuthSession(t, app, store)

	form := url.Values{"group_dn": {"cn=staff,ou=groups,dc=test"}}
	req := httptest.NewRequest(http.MethodPost,
		"/users/bulk?action=add-to-group",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := app.fiber.Test(req)
	if err != nil {
		t.Fatalf("bulk POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected 303, got %d", resp.StatusCode)
	}

	if loc := resp.Header.Get("Location"); loc != "/users" {
		t.Fatalf("expected redirect to /users, got %q", loc)
	}
}

// TestCollectTargetDNs_URLEncoded sanity-checks the PeekMulti path which
// is not covered by the other handler tests (they always supply a full
// URL-encoded body).
func TestCollectTargetDNs_URLEncoded(t *testing.T) {
	f := fiber.New()
	var got []string

	f.Post("/x", func(c *fiber.Ctx) error {
		got = collectTargetDNs(c)

		return c.SendStatus(fiber.StatusOK)
	})

	form := url.Values{}
	form.Add("target_dn", "cn=alice,dc=test")
	form.Add("target_dn", "cn=bob,dc=test")
	form.Add("target_dn", "cn=carol,dc=test")

	req := httptest.NewRequest(http.MethodPost, "/x",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := f.Test(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if len(got) != 3 {
		t.Fatalf("expected 3 DNs, got %d: %#v", len(got), got)
	}
}
