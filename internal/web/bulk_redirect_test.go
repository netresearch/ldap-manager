// internal/web/bulk_redirect_test.go
package web

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// TestBulkRedirectAfter exercises the helper that preserves query
// filters (and optionally ?panel=) in the post-action 303 target.
// Covers the bug where POSTing from /users?ou=Eng&enabled=true&panel=1
// redirected to a bare /users, closing the drawer and clearing the
// filter chips on every disable/delete action.
func TestBulkRedirectAfter(t *testing.T) {
	cases := []struct {
		name         string
		referer      string
		fallbackList string
		dropPanel    bool
		want         string
	}{
		{
			name:         "no referer falls back to list",
			referer:      "",
			fallbackList: "/users",
			dropPanel:    false,
			want:         "/users",
		},
		{
			name:         "relative list referer preserved (disable)",
			referer:      "/users?ou=Eng&enabled=true&panel=1",
			fallbackList: "/users",
			dropPanel:    false,
			want:         "/users?enabled=true&ou=Eng&panel=1",
		},
		{
			name:         "list referer strips panel on delete",
			referer:      "/users?ou=Eng&enabled=true&panel=1",
			fallbackList: "/users",
			dropPanel:    true,
			want:         "/users?enabled=true&ou=Eng",
		},
		{
			name:         "detail page referer preserved on disable",
			referer:      "/users/cn%3Dbob%2Cdc%3Dx?panel=1&ou=Eng",
			fallbackList: "/users",
			dropPanel:    false,
			want:         "/users/cn%3Dbob%2Cdc%3Dx?ou=Eng&panel=1",
		},
		{
			name:         "detail page referer collapses to list on delete",
			referer:      "/users/cn%3Dbob%2Cdc%3Dx?panel=1&ou=Eng",
			fallbackList: "/users",
			dropPanel:    true,
			want:         "/users?ou=Eng",
		},
		{
			name:         "unrelated referer falls back",
			referer:      "/groups?ou=Ops",
			fallbackList: "/users",
			dropPanel:    false,
			want:         "/users",
		},
		{
			name:         "cross-origin referer rejected",
			referer:      "https://evil.example.com/users?ou=Eng",
			fallbackList: "/users",
			dropPanel:    false,
			want:         "/users",
		},
		{
			name:         "unparseable referer falls back",
			referer:      "://not-a-url",
			fallbackList: "/users",
			dropPanel:    false,
			want:         "/users",
		},
		{
			name:         "plain list no query preserved",
			referer:      "/groups",
			fallbackList: "/groups",
			dropPanel:    true,
			want:         "/groups",
		},
		{
			name:         "panel key dropped even when only param",
			referer:      "/users?panel=1",
			fallbackList: "/users",
			dropPanel:    true,
			want:         "/users",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/_probe", func(c *fiber.Ctx) error {
				return c.SendString(bulkRedirectAfter(c, tc.fallbackList, tc.dropPanel))
			})

			req := httptest.NewRequest("GET", "/_probe", nil)
			if tc.referer != "" {
				req.Header.Set(fiber.HeaderReferer, tc.referer)
			}

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			body := make([]byte, 256)
			n, _ := resp.Body.Read(body)
			got := string(body[:n])

			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
