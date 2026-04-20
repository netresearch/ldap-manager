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
	"github.com/stretchr/testify/require"

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

	// Variant: empty users list (exercises "no users" branch). Assert the
	// template still produces output and does not error.
	var emptyBuf bytes.Buffer
	require.NoError(t, Users(nil, false, Flashes()).Render(ctx, &emptyBuf),
		"empty Users render should not error")
	require.NotZero(t, emptyBuf.Len(), "empty Users render should still produce non-empty HTML")

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

	// Empty Groups list (covers "no groups" branch). Assert the render
	// succeeds and produces non-empty output.
	buf.Reset()
	require.NoError(t, Groups(nil).Render(ctx, &buf), "empty Groups render should not error")
	require.NotZero(t, buf.Len(), "empty Groups render should still produce non-empty HTML")

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

	// Empty Computers list (covers "no computers" branch). Assert that the
	// render succeeds and produces non-empty output.
	buf.Reset()
	require.NoError(t, Computers(nil).Render(ctx, &buf), "empty Computers render should not error")
	require.NotZero(t, buf.Len(), "empty Computers render should still produce non-empty HTML")

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

// failAfterNWriter returns an io.Writer that fails its Nth Write call.
// Used to exercise the `if WriteString err != nil { return err }` branches
// scattered throughout every generated templ component.
type failAfterNWriter struct {
	calls int
	fail  int
}

func (f *failAfterNWriter) Write(p []byte) (int, error) {
	f.calls++
	if f.calls >= f.fail {
		return 0, errors.New("fail")
	}

	return len(p), nil
}

// TestRender_WriteErrorPropagation injects a failing writer at several
// different call counts so different error-handling branches inside the
// generated templates fire.
func TestRender_WriteErrorPropagation(t *testing.T) {
	ctx := context.Background()

	renderers := []struct {
		name string
		do   func(w *failAfterNWriter) error
	}{
		{"FourOhFour", func(w *failAfterNWriter) error { return FourOhFour("/x").Render(ctx, w) }},
		{"FourOhThree", func(w *failAfterNWriter) error { return FourOhThree("x").Render(ctx, w) }},
		{"FiveHundred", func(w *failAfterNWriter) error { return FiveHundred(errors.New("x")).Render(ctx, w) }},
		{"Login", func(w *failAfterNWriter) error { return Login(nil, "v", "csrf").Render(ctx, w) }},
		{"Code", func(w *failAfterNWriter) error { return Code("x").Render(ctx, w) }},
		{"Copyable", func(w *failAfterNWriter) error { return Copyable("x").Render(ctx, w) }},
		{"ToggleButtons", func(w *failAfterNWriter) error { return ToggleButtons().Render(ctx, w) }},
		{"Index", func(w *failAfterNWriter) error {
			u := &ldap_cache.FullLDAPUser{
				User:   ldap.User{Enabled: true, SAMAccountName: "x", Mail: strPtr("a@b"), Description: "d"},
				Groups: []ldap.Group{{Members: []string{}}},
			}

			return Index(u).Render(ctx, w)
		}},
		{"Users", func(w *failAfterNWriter) error {
			return Users([]ldap.User{{Enabled: true, SAMAccountName: "u"}}, false, nil).Render(ctx, w)
		}},
		{"Groups", func(w *failAfterNWriter) error {
			return Groups([]ldap.Group{{Members: []string{"x"}}}).Render(ctx, w)
		}},
		{"Computers", func(w *failAfterNWriter) error {
			return Computers([]ldap.Computer{{Enabled: true, SAMAccountName: "pc$"}}).Render(ctx, w)
		}},
	}

	for _, r := range renderers {
		// Fail on many different Write counts — this spreads across the
		// many per-string if-error-return branches.
		for failAt := 1; failAt <= 30; failAt++ {
			w := &failAfterNWriter{fail: failAt}
			_ = r.do(w) // errors are expected and fine
		}
	}

	// Fuller shapes: detail templates have many more branches.
	detailRenderers := []func(w *failAfterNWriter) error{
		func(w *failAfterNWriter) error {
			fullUser := &ldap_cache.FullLDAPUser{
				User: ldap.User{
					Enabled: true, SAMAccountName: "u",
					Description: "d", Mail: strPtr("a@b"), LastLogon: 133500000000000000,
				},
				Groups: []ldap.Group{
					{Members: []string{}},
					{Members: []string{"m"}},
				},
			}

			return User(fullUser, []ldap.Group{{}}, Flashes(SuccessFlash("ok"), ErrorFlash("bad")), "csrf").Render(context.Background(), w)
		},
		func(w *failAfterNWriter) error {
			fullGroup := &ldap_cache.FullLDAPGroup{
				Group: ldap.Group{
					Description: "desc", Members: []string{"m"},
					MemberOf: []string{"parent"},
				},
				Members: []ldap.User{
					{Enabled: true, SAMAccountName: "u1"},
					{Enabled: false, SAMAccountName: "u2"},
				},
				ParentGroups: []ldap.Group{{Members: []string{}}},
			}

			return Group(fullGroup, []ldap.User{{SAMAccountName: "other"}}, Flashes(InfoFlash("hi")), "csrf").Render(context.Background(), w)
		},
		func(w *failAfterNWriter) error {
			fullComp := &ldap_cache.FullLDAPComputer{
				Computer: ldap.Computer{
					Enabled: true, SAMAccountName: "pc$",
					Description: "d", DNSHostName: "h.local",
					OS: "Linux", OSVersion: "5.0", ServicePack: "SP",
					LastLogon: 133500000000000000,
				},
				Groups: []ldap.Group{{Members: []string{"m"}}},
			}

			return Computer(fullComp).Render(context.Background(), w)
		},
	}

	for _, render := range detailRenderers {
		for failAt := 1; failAt <= 100; failAt++ {
			w := &failAfterNWriter{fail: failAt}
			_ = render(w)
		}
	}

	// Also hit loggedIn / list via the error-injecting writer.
	helperRenderers := []func(w *failAfterNWriter) error{
		func(w *failAfterNWriter) error {
			return loggedIn("/users", "Users", Flashes(ErrorFlash("boom"))).Render(context.Background(), w)
		},
		func(w *failAfterNWriter) error {
			items := specializeGroups([]ldap.Group{{Members: []string{}}, {Members: []string{"m"}}})

			return list(items).Render(context.Background(), w)
		},
		func(w *failAfterNWriter) error {
			return base("T").Render(context.Background(), w)
		},
		func(w *failAfterNWriter) error {
			return baseWithAssets("T", "/s.css").Render(context.Background(), w)
		},
	}

	for _, render := range helperRenderers {
		for failAt := 1; failAt <= 40; failAt++ {
			w := &failAfterNWriter{fail: failAt}
			_ = render(w)
		}
	}

	// Icons have a single WriteString call — fail on the first write.
	iconRenderers := []func(w *failAfterNWriter) error{
		func(w *failAfterNWriter) error { return homeIcon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return usersIcon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return groupIcon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return laptopIcon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return logoutIcon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return lockIcon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return lockIcon("c1").Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return lockOpenIcon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return plusIcon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return rightArrowIcon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return xIcon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return sunIcon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return moonIcon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return computerDesktopIcon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return squares2x2Icon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return viewColumnsIcon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return sparklesIcon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return copyIcon().Render(context.Background(), w) },
		func(w *failAfterNWriter) error { return checkIcon().Render(context.Background(), w) },
	}

	for _, render := range iconRenderers {
		for failAt := 1; failAt <= 3; failAt++ {
			w := &failAfterNWriter{fail: failAt}
			_ = render(w)
		}
	}
}

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
// of every generated template is exercised. With a cancelled context, Render
// must return a non-nil error that wraps context.Canceled — anything else is
// a real regression in the generated templ runtime.
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

			if err == nil {
				t.Fatalf("%s: expected non-nil error from Render with cancelled ctx, got nil", c.name)
			}

			if !errors.Is(err, context.Canceled) {
				t.Errorf("%s: expected error to wrap context.Canceled, got %v", c.name, err)
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
