package templates

// Rendering coverage tests for generated *_templ.go files.
//
// These tests render every exported (and via a container, most unexported)
// template component to a bytes.Buffer. The goal is to exercise the code paths
// inside the generated templ code so coverage reflects the cost of the
// templates. The tests assert only that rendering succeeds and produces some
// output; they intentionally do NOT assert the exact HTML structure (which is
// already validated by higher-level integration tests and by templ itself).

import (
	"bytes"
	"context"
	"errors"
	"testing"

	templruntime "github.com/a-h/templ/runtime"
	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
)

func mustRender(t *testing.T, name string, fn func() error) {
	t.Helper()

	if err := fn(); err != nil {
		t.Fatalf("render %s: %v", name, err)
	}
}

func TestRender_Errors(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name   string
		render func(buf *bytes.Buffer) error
	}{
		{"FourOhFour", func(b *bytes.Buffer) error { return FourOhFour("/missing").Render(ctx, b) }},
		{"FourOhThree", func(b *bytes.Buffer) error { return FourOhThree("CSRF failed").Render(ctx, b) }},
		{"FiveHundred", func(b *bytes.Buffer) error { return FiveHundred(errors.New("boom")).Render(ctx, b) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			mustRender(t, tc.name, func() error { return tc.render(&buf) })

			if buf.Len() == 0 {
				t.Fatalf("%s produced empty output", tc.name)
			}
		})
	}
}

func TestRender_Login(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer
	mustRender(t, "Login", func() error {
		return Login(Flashes(ErrorFlash("bad creds")), "v1.2.3", "csrf-token").Render(ctx, &buf)
	})

	if buf.Len() == 0 {
		t.Fatal("Login produced empty output")
	}

	buf.Reset()
	mustRender(t, "LoginWithStyles", func() error {
		return LoginWithStyles(Flashes(SuccessFlash("ok")), "v1.2.3", "csrf-token", "/static/styles.abc.css").
			Render(ctx, &buf)
	})

	if buf.Len() == 0 {
		t.Fatal("LoginWithStyles produced empty output")
	}
}

func TestRender_Index(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name string
		user *ldap_cache.FullLDAPUser
	}{
		{
			name: "with description and mail and groups",
			user: &ldap_cache.FullLDAPUser{
				User: ldap.User{
					Enabled:        true,
					SAMAccountName: "john.doe",
					Description:    "Test user",
					Mail:           strPtr("john@example.com"),
					LastLogon:      133000000000000000,
				},
				Groups: []ldap.Group{{Members: []string{"cn=john.doe,ou=users,dc=example,dc=com"}}},
			},
		},
		{
			name: "empty description, no mail, no groups",
			user: &ldap_cache.FullLDAPUser{
				User: ldap.User{
					Enabled:        true,
					SAMAccountName: "empty.user",
					Description:    "",
					Mail:           nil,
				},
				Groups: nil,
			},
		},
		{
			name: "empty mail string",
			user: &ldap_cache.FullLDAPUser{
				User: ldap.User{
					Enabled:        true,
					SAMAccountName: "emptymail",
					Mail:           strPtr(""),
				},
				Groups: nil,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			mustRender(t, "Index/"+tc.name, func() error { return Index(tc.user).Render(ctx, &buf) })

			if buf.Len() == 0 {
				t.Fatalf("Index/%s produced empty output", tc.name)
			}
		})
	}

	var buf bytes.Buffer
	mustRender(t, "Code", func() error { return Code("some code").Render(ctx, &buf) })

	if buf.Len() == 0 {
		t.Fatal("Code produced empty output")
	}
}

func TestRender_Users(t *testing.T) {
	ctx := context.Background()

	users := []ldap.User{
		{Enabled: true, SAMAccountName: "user1", Description: "one", Mail: strPtr("user1@example.com")},
		{Enabled: false, SAMAccountName: "user2"},
	}

	for _, showDisabled := range []bool{false, true} {
		var buf bytes.Buffer
		mustRender(t, "Users", func() error {
			return Users(users, showDisabled, Flashes(InfoFlash("test"))).Render(ctx, &buf)
		})

		if buf.Len() == 0 {
			t.Fatalf("Users(showDisabled=%v) produced empty output", showDisabled)
		}
	}

	// Variant: empty users list (exercises "no users" branch)
	var emptyBuf bytes.Buffer
	mustRender(t, "Users-empty", func() error {
		return Users(nil, false, Flashes()).Render(ctx, &emptyBuf)
	})

	// Detail: User with assigned groups, mail, description, disabled
	userCases := []struct {
		name       string
		user       *ldap_cache.FullLDAPUser
		unassigned []ldap.Group
	}{
		{
			name: "enabled with mail/description/groups/lastlogon",
			user: &ldap_cache.FullLDAPUser{
				User: ldap.User{
					Enabled:        true,
					SAMAccountName: "user1",
					Description:    "desc",
					Mail:           strPtr("u1@example.com"),
					LastLogon:      133500000000000000,
				},
				Groups: []ldap.Group{{Members: []string{"cn=user1,dc=example,dc=com"}}},
			},
			unassigned: []ldap.Group{{Members: []string{}}},
		},
		{
			name: "disabled without mail/description/lastlogon, no groups",
			user: &ldap_cache.FullLDAPUser{
				User: ldap.User{Enabled: false, SAMAccountName: "user2"},
			},
			unassigned: nil,
		},
	}

	for _, tc := range userCases {
		t.Run("User/"+tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			mustRender(t, "User/"+tc.name, func() error {
				return User(tc.user, tc.unassigned, Flashes(SuccessFlash("saved")), "csrf").Render(ctx, &buf)
			})

			if buf.Len() == 0 {
				t.Fatalf("User/%s produced empty output", tc.name)
			}
		})
	}
}

func TestRender_Groups(t *testing.T) {
	ctx := context.Background()

	groups := []ldap.Group{
		{Members: []string{"cn=user1,dc=example,dc=com"}},
		{Members: []string{}},
	}

	var buf bytes.Buffer
	mustRender(t, "Groups", func() error { return Groups(groups).Render(ctx, &buf) })

	if buf.Len() == 0 {
		t.Fatal("Groups produced empty output")
	}

	// Empty Groups list (covers "no groups" branch)
	buf.Reset()
	mustRender(t, "Groups-empty", func() error { return Groups(nil).Render(ctx, &buf) })

	// Detail: cover populated + empty variants
	cases := []struct {
		name       string
		group      *ldap_cache.FullLDAPGroup
		unassigned []ldap.User
	}{
		{
			name: "fully populated with description and parent groups",
			group: &ldap_cache.FullLDAPGroup{
				Group: ldap.Group{
					Description: "Test group",
					Members:     []string{"cn=u1,dc=e,dc=com"},
					MemberOf:    []string{"cn=parentgroup,dc=e,dc=com"},
				},
				Members: []ldap.User{
					{Enabled: true, SAMAccountName: "u1"},
					{Enabled: false, SAMAccountName: "u2"},
				},
				ParentGroups: []ldap.Group{{Members: []string{}}},
			},
			unassigned: []ldap.User{{Enabled: true, SAMAccountName: "other"}},
		},
		{
			name:       "empty group with no members",
			group:      &ldap_cache.FullLDAPGroup{},
			unassigned: nil,
		},
	}

	for _, tc := range cases {
		t.Run("Group/"+tc.name, func(t *testing.T) {
			var b bytes.Buffer
			mustRender(t, "Group/"+tc.name, func() error {
				return Group(tc.group, tc.unassigned, Flashes(ErrorFlash("nope")), "csrf").Render(ctx, &b)
			})

			if b.Len() == 0 {
				t.Fatalf("Group/%s produced empty output", tc.name)
			}
		})
	}
}

func TestRender_Computers(t *testing.T) {
	ctx := context.Background()

	computers := []ldap.Computer{
		{Enabled: true, SAMAccountName: "pc1$"},
		{Enabled: false, SAMAccountName: "pc2$"},
	}

	var buf bytes.Buffer
	mustRender(t, "Computers", func() error { return Computers(computers).Render(ctx, &buf) })

	if buf.Len() == 0 {
		t.Fatal("Computers produced empty output")
	}

	buf.Reset()
	mustRender(t, "Computers-empty", func() error { return Computers(nil).Render(ctx, &buf) })

	// Detail: with and without groups, enabled and disabled, with lastlogon
	cases := []struct {
		name string
		comp *ldap_cache.FullLDAPComputer
	}{
		{
			name: "fully populated enabled",
			comp: &ldap_cache.FullLDAPComputer{
				Computer: ldap.Computer{
					Enabled:        true,
					SAMAccountName: "pc1$",
					Description:    "desc",
					DNSHostName:    "pc1.example.com",
					OS:             "Windows",
					OSVersion:      "10.0",
					ServicePack:    "SP1",
					LastLogon:      133500000000000000,
				},
				Groups: []ldap.Group{{Members: []string{"cn=pc1,dc=e,dc=com"}}},
			},
		},
		{
			name: "disabled empty fields no groups",
			comp: &ldap_cache.FullLDAPComputer{
				Computer: ldap.Computer{Enabled: false, SAMAccountName: "pc2$"},
			},
		},
	}

	for _, tc := range cases {
		t.Run("Computer/"+tc.name, func(t *testing.T) {
			var b bytes.Buffer
			mustRender(t, "Computer/"+tc.name, func() error {
				return Computer(tc.comp).Render(ctx, &b)
			})

			if b.Len() == 0 {
				t.Fatalf("Computer/%s produced empty output", tc.name)
			}
		})
	}
}

func TestRender_TopLevelHelpers(t *testing.T) {
	ctx := context.Background()

	// ToggleButtons is the only exported top-level component not tied to entity data.
	var buf bytes.Buffer
	mustRender(t, "ToggleButtons", func() error { return ToggleButtons().Render(ctx, &buf) })

	if buf.Len() == 0 {
		t.Fatal("ToggleButtons produced empty output")
	}

	// Copyable exercises the interactive copy-to-clipboard snippet.
	buf.Reset()
	mustRender(t, "Copyable", func() error { return Copyable("some-dn").Render(ctx, &buf) })

	if buf.Len() == 0 {
		t.Fatal("Copyable produced empty output")
	}
}

func TestFormatLastLogon(t *testing.T) {
	for _, tc := range []struct {
		name    string
		in      int64
		wantHas string
	}{
		{"zero timestamp", 0, "Never"},
		{"recent timestamp", 133500000000000000, "-"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := formatLastLogon(tc.in)
			if got == "" {
				t.Fatalf("formatLastLogon(%d) returned empty string", tc.in)
			}

			if tc.wantHas != "" && tc.wantHas == "Never" && got != "Never" {
				t.Errorf("expected %q, got %q", tc.wantHas, got)
			}
		})
	}
}

func TestGetNavbarClasses(t *testing.T) {
	if out := getNavbarClasses("users", "users"); out == "" {
		t.Error("expected non-empty result for active page")
	}

	if out := getNavbarClasses("users", "groups"); out == "" {
		t.Error("expected non-empty result for inactive page")
	}

	// Active and inactive should return different strings.
	if getNavbarClasses("a", "a") == getNavbarClasses("a", "b") {
		t.Error("expected active and inactive classes to differ")
	}
}

func strPtr(s string) *string { return &s }

// TestRender_UnexportedHelpers directly exercises the unexported base*,
// loggedIn, and list templates so they're attributed coverage independent of
// the outer page templates that call them.
func TestRender_UnexportedHelpers(t *testing.T) {
	ctx := context.Background()

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	cases := []struct {
		name string
		fn   func(c context.Context) error
	}{
		{"base", func(c context.Context) error { var b bytes.Buffer; return base("T").Render(c, &b) }},
		{"baseWithManifest", func(c context.Context) error {
			var b bytes.Buffer

			return baseWithManifest("T", "/static/styles.css").Render(c, &b)
		}},
		{"baseWithAssets", func(c context.Context) error {
			var b bytes.Buffer

			return baseWithAssets("T", "/static/styles.css").Render(c, &b)
		}},
		{"loggedIn-no-flashes", func(c context.Context) error {
			var b bytes.Buffer

			return loggedIn("/", "Home", nil).Render(c, &b)
		}},
		{"loggedIn-with-flashes", func(c context.Context) error {
			var b bytes.Buffer

			return loggedIn("/users", "Users", Flashes(SuccessFlash("ok"), ErrorFlash("bad"))).Render(c, &b)
		}},
		{"list-empty", func(c context.Context) error {
			var b bytes.Buffer

			return list(nil).Render(c, &b)
		}},
		{"list-populated", func(c context.Context) error {
			var b bytes.Buffer
			// Use specializeGroups (already covered) to build a []Displayer.
			items := specializeGroups([]ldap.Group{{Members: []string{}}})

			return list(items).Render(c, &b)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(ctx); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			// Also run with cancelled context to cover ctx.Err() branch.
			_ = tc.fn(cancelCtx)
		})
	}
}

// TestRender_Icons directly exercises each icon component so its
// instrumentation is recorded independently from the enclosing templates.
func TestRender_Icons(t *testing.T) {
	ctx := context.Background()

	iconComponents := map[string]func() error{}

	// Build a map of all icon renderers. We have to reference them via
	// closures because icons are unexported and must be called from inside
	// the templates package (this test file is in the templates package).
	iconComponents["homeIcon"] = func() error {
		var buf bytes.Buffer

		return homeIcon().Render(ctx, &buf)
	}
	iconComponents["usersIcon"] = func() error {
		var buf bytes.Buffer

		return usersIcon().Render(ctx, &buf)
	}
	iconComponents["groupIcon"] = func() error {
		var buf bytes.Buffer

		return groupIcon().Render(ctx, &buf)
	}
	iconComponents["laptopIcon"] = func() error {
		var buf bytes.Buffer

		return laptopIcon().Render(ctx, &buf)
	}
	iconComponents["logoutIcon"] = func() error {
		var buf bytes.Buffer

		return logoutIcon().Render(ctx, &buf)
	}
	iconComponents["lockIcon"] = func() error {
		var buf bytes.Buffer

		return lockIcon("extra").Render(ctx, &buf)
	}
	iconComponents["lockIcon-noClass"] = func() error {
		var buf bytes.Buffer

		return lockIcon().Render(ctx, &buf)
	}
	iconComponents["lockOpenIcon"] = func() error {
		var buf bytes.Buffer

		return lockOpenIcon("extra").Render(ctx, &buf)
	}
	iconComponents["plusIcon"] = func() error {
		var buf bytes.Buffer

		return plusIcon().Render(ctx, &buf)
	}
	iconComponents["rightArrowIcon"] = func() error {
		var buf bytes.Buffer

		return rightArrowIcon().Render(ctx, &buf)
	}
	iconComponents["xIcon"] = func() error {
		var buf bytes.Buffer

		return xIcon().Render(ctx, &buf)
	}
	iconComponents["sunIcon"] = func() error {
		var buf bytes.Buffer

		return sunIcon().Render(ctx, &buf)
	}
	iconComponents["moonIcon"] = func() error {
		var buf bytes.Buffer

		return moonIcon().Render(ctx, &buf)
	}
	iconComponents["computerDesktopIcon"] = func() error {
		var buf bytes.Buffer

		return computerDesktopIcon().Render(ctx, &buf)
	}
	iconComponents["squares2x2Icon"] = func() error {
		var buf bytes.Buffer

		return squares2x2Icon().Render(ctx, &buf)
	}
	iconComponents["viewColumnsIcon"] = func() error {
		var buf bytes.Buffer

		return viewColumnsIcon().Render(ctx, &buf)
	}
	iconComponents["sparklesIcon"] = func() error {
		var buf bytes.Buffer

		return sparklesIcon().Render(ctx, &buf)
	}
	iconComponents["copyIcon"] = func() error {
		var buf bytes.Buffer

		return copyIcon().Render(ctx, &buf)
	}
	iconComponents["checkIcon"] = func() error {
		var buf bytes.Buffer

		return checkIcon().Render(ctx, &buf)
	}

	for name, render := range iconComponents {
		t.Run(name, func(t *testing.T) {
			if err := render(); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		})
	}

	// Also exercise icons with a cancelled context to cover the ctx.Err()
	// early-return branch inside every icon.
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	cancelComponents := []func() error{
		func() error { var b bytes.Buffer; return homeIcon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return usersIcon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return groupIcon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return laptopIcon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return logoutIcon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return lockIcon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return lockOpenIcon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return plusIcon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return rightArrowIcon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return xIcon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return sunIcon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return moonIcon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return computerDesktopIcon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return squares2x2Icon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return viewColumnsIcon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return sparklesIcon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return copyIcon().Render(cancelCtx, &b) },
		func() error { var b bytes.Buffer; return checkIcon().Render(cancelCtx, &b) },
	}

	for _, fn := range cancelComponents {
		_ = fn()
	}
}

// TestRender_WithExistingBuffer exercises the branch where the Writer passed
// to Render is already a *templruntime.Buffer (existing=true in
// templruntime.GetBuffer), which skips the defer-release path of the
// generated code.
func TestRender_WithExistingBuffer(t *testing.T) {
	ctx := context.Background()

	// Wrap a bytes.Buffer inside a templruntime.Buffer so the generated code
	// sees IsBuffer=true and takes the short path.
	var inner bytes.Buffer

	tmplBuf, existing := templruntime.GetBuffer(&inner)
	if existing {
		t.Fatal("expected GetBuffer to return existing=false for bytes.Buffer")
	}

	defer func() { _ = templruntime.ReleaseBuffer(tmplBuf) }()

	components := []struct {
		name     string
		renderer func() error
	}{
		{"FourOhFour", func() error { return FourOhFour("/x").Render(ctx, tmplBuf) }},
		{"FourOhThree", func() error { return FourOhThree("x").Render(ctx, tmplBuf) }},
		{"FiveHundred", func() error { return FiveHundred(errors.New("x")).Render(ctx, tmplBuf) }},
		{"Code", func() error { return Code("x").Render(ctx, tmplBuf) }},
		{"Copyable", func() error { return Copyable("x").Render(ctx, tmplBuf) }},
		{"ToggleButtons", func() error { return ToggleButtons().Render(ctx, tmplBuf) }},
		{"Login", func() error { return Login(nil, "v", "csrf").Render(ctx, tmplBuf) }},
	}

	for _, c := range components {
		t.Run(c.name, func(t *testing.T) {
			if err := c.renderer(); err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
		})
	}
}

// TestRender_WithCancelledContext ensures the early ctx.Err() check at the top
// of every generated template is exercised.
func TestRender_WithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var buf bytes.Buffer

	// All components share the same check, so rendering any one of them with
	// a cancelled context covers the early-return branch.
	components := []struct {
		name     string
		renderer func() error
	}{
		{"FourOhFour", func() error { return FourOhFour("/x").Render(ctx, &buf) }},
		{"FourOhThree", func() error { return FourOhThree("x").Render(ctx, &buf) }},
		{"FiveHundred", func() error { return FiveHundred(errors.New("x")).Render(ctx, &buf) }},
		{"Login", func() error { return Login(nil, "v", "csrf").Render(ctx, &buf) }},
		{"Code", func() error { return Code("x").Render(ctx, &buf) }},
		{"Copyable", func() error { return Copyable("x").Render(ctx, &buf) }},
		{"ToggleButtons", func() error { return ToggleButtons().Render(ctx, &buf) }},
	}

	for _, c := range components {
		t.Run(c.name, func(t *testing.T) {
			buf.Reset()
			err := c.renderer()

			// With a cancelled context we expect an error (context.Canceled).
			if !errors.Is(err, context.Canceled) {
				t.Logf("%s: got err=%v (expected context.Canceled)", c.name, err)
			}
		})
	}
}

// TestDisplayerAdapters directly exercises the user/computer/group wrapper
// types' Displayer methods. These wrappers exist so that the templates can
// present users/computers/groups via a common interface; several paths are
// currently only reachable via specializeUsers/specializeComputers helpers
// that aren't in the live render pipeline.
func TestDisplayerAdapters(t *testing.T) {
	u := ldap.User{Enabled: true, SAMAccountName: "alice"}
	g := ldap.Group{}
	c := ldap.Computer{Enabled: false, SAMAccountName: "pc1$"}

	t.Run("user adapter round-trip via specializeUsers", func(t *testing.T) {
		displayers := specializeUsers([]ldap.User{u})
		if len(displayers) != 1 {
			t.Fatalf("expected 1 displayer, got %d", len(displayers))
		}

		d := displayers[0]
		_ = d.ID()
		_ = d.Name()
		_ = d.URL()

		if !d.Enabled() {
			t.Error("user displayer should report enabled")
		}
	})

	t.Run("group adapter round-trip via specializeGroups", func(t *testing.T) {
		displayers := specializeGroups([]ldap.Group{g})
		if len(displayers) != 1 {
			t.Fatalf("expected 1 displayer, got %d", len(displayers))
		}

		d := displayers[0]
		_ = d.ID()
		_ = d.Name()
		_ = d.URL()
		_ = d.Enabled()
	})

	t.Run("computer adapter round-trip via specializeComputers", func(t *testing.T) {
		displayers := specializeComputers([]ldap.Computer{c})
		if len(displayers) != 1 {
			t.Fatalf("expected 1 displayer, got %d", len(displayers))
		}

		d := displayers[0]
		_ = d.ID()
		_ = d.Name()
		_ = d.URL()

		if d.Enabled() {
			t.Error("computer displayer with Enabled=false should not report enabled")
		}
	})
}
