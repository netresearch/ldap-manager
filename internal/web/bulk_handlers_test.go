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

// TestBulkHandler_Groups_UnknownAction verifies unknown actions on the
// groups bulk endpoint return 400 before any LDAP work.
func TestBulkHandler_Groups_UnknownAction(t *testing.T) {
	app, store := setupFullTestApp(t)

	cookies := createAuthSession(t, app, store)

	req := httptest.NewRequest(http.MethodPost, "/groups/bulk?action=nope",
		strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := app.fiber.Test(req)
	if err != nil {
		t.Fatalf("groups/bulk POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestBulkHandler_Groups_DeleteStubbed verifies the delete action on
// /groups/bulk is explicitly not yet implemented (simple-ldap-go has no
// DeleteGroup). Stubbed with 501 rather than a half-baked DIY.
func TestBulkHandler_Groups_DeleteStubbed(t *testing.T) {
	app, store := setupFullTestApp(t)

	cookies := createAuthSession(t, app, store)

	form := url.Values{"target_dn": {"cn=a,dc=test", "cn=b,dc=test"}}
	req := httptest.NewRequest(http.MethodPost,
		"/groups/bulk?action=delete",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := app.fiber.Test(req)
	if err != nil {
		t.Fatalf("groups/bulk POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", resp.StatusCode)
	}
}

// TestBulkHandler_Computers_DisableStubbed verifies the disable action on
// /computers/bulk is explicitly not yet implemented.
func TestBulkHandler_Computers_DisableStubbed(t *testing.T) {
	app, store := setupFullTestApp(t)

	cookies := createAuthSession(t, app, store)

	form := url.Values{"target_dn": {"cn=pc1,dc=test"}}
	req := httptest.NewRequest(http.MethodPost,
		"/computers/bulk?action=disable",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := app.fiber.Test(req)
	if err != nil {
		t.Fatalf("computers/bulk POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", resp.StatusCode)
	}
}

// TestBulkHandler_Users_DisableStubbed verifies the disable action on
// /users/bulk is explicitly not yet implemented for inetOrgPerson.
func TestBulkHandler_Users_DisableStubbed(t *testing.T) {
	app, store := setupFullTestApp(t)

	cookies := createAuthSession(t, app, store)

	form := url.Values{"target_dn": {"cn=u1,dc=test"}}
	req := httptest.NewRequest(http.MethodPost,
		"/users/bulk?action=disable",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := app.fiber.Test(req)
	if err != nil {
		t.Fatalf("users/bulk POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", resp.StatusCode)
	}
}

// TestBulkHandler_Groups_AddMembersMissingUser verifies the add-members
// action requires user_dn.
func TestBulkHandler_Groups_AddMembersMissingUser(t *testing.T) {
	app, store := setupFullTestApp(t)

	cookies := createAuthSession(t, app, store)

	form := url.Values{"target_dn": {"cn=g1,dc=test"}}
	req := httptest.NewRequest(http.MethodPost,
		"/groups/bulk?action=add-members",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := app.fiber.Test(req)
	if err != nil {
		t.Fatalf("groups/bulk POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestBulkHandler_Users_RemoveFromGroupMissingGroup verifies the
// remove-from-group action requires group_dn.
func TestBulkHandler_Users_RemoveFromGroupMissingGroup(t *testing.T) {
	app, store := setupFullTestApp(t)

	cookies := createAuthSession(t, app, store)

	form := url.Values{"target_dn": {"cn=u1,dc=test"}}
	req := httptest.NewRequest(http.MethodPost,
		"/users/bulk?action=remove-from-group",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := app.fiber.Test(req)
	if err != nil {
		t.Fatalf("users/bulk POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
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
