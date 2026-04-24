// internal/web/templates/drawer_disable_gating_test.go
package templates

import (
	"bytes"
	"context"
	"strings"
	"testing"

	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
)

// TestUserDrawerDisableGating asserts that the Disable action form is only
// rendered when both (a) the backend is Active Directory and (b) the user
// is currently enabled. Bug: the drawer previously emitted the Disable
// button for an already-disabled user, so the account was told to disable
// itself again — visually contradictory and operationally useless.
func TestUserDrawerDisableGating(t *testing.T) {
	const disableMarker = `action="/users/bulk?action=disable"`

	cases := []struct {
		name         string
		isAD         bool
		enabled      bool
		wantDisable  bool
		wantStatusOK string
	}{
		{"AD + enabled shows Disable", true, true, true, "Enabled"},
		{"AD + already disabled hides Disable", true, false, false, "Disabled"},
		{"OpenLDAP + enabled hides Disable", false, true, false, "Enabled"},
		{"OpenLDAP + disabled hides Disable", false, false, false, "Disabled"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vm := UserDrawerVM{
				User: &ldap_cache.FullLDAPUser{
					User: ldap.User{Enabled: tc.enabled, SAMAccountName: "bob"},
				},
				IsAD: tc.isAD,
			}

			var buf bytes.Buffer
			if err := UserDrawerFragment(vm).Render(context.Background(), &buf); err != nil {
				t.Fatalf("render drawer fragment: %v", err)
			}

			html := buf.String()
			hasDisable := strings.Contains(html, disableMarker)

			if hasDisable != tc.wantDisable {
				t.Errorf("disable form present=%v, want=%v (IsAD=%v, Enabled=%v)",
					hasDisable, tc.wantDisable, tc.isAD, tc.enabled)
			}
			if !strings.Contains(html, tc.wantStatusOK) {
				t.Errorf("expected drawer to render status %q, missing in output", tc.wantStatusOK)
			}
		})
	}
}

// TestComputerDrawerDisableGating mirrors TestUserDrawerDisableGating for
// the computer detail drawer. Same bug symptom (Disable shown on an
// already-disabled machine account), same fix shape.
func TestComputerDrawerDisableGating(t *testing.T) {
	const disableMarker = `action="/computers/bulk?action=disable"`

	cases := []struct {
		name        string
		isAD        bool
		enabled     bool
		wantDisable bool
	}{
		{"AD + enabled shows Disable", true, true, true},
		{"AD + already disabled hides Disable", true, false, false},
		{"OpenLDAP + enabled hides Disable", false, true, false},
		{"OpenLDAP + disabled hides Disable", false, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vm := ComputerDrawerVM{
				Computer: ldap.Computer{Enabled: tc.enabled, SAMAccountName: "pc01$"},
				IsAD:     tc.isAD,
			}

			var buf bytes.Buffer
			if err := ComputerDrawerFragment(vm).Render(context.Background(), &buf); err != nil {
				t.Fatalf("render drawer fragment: %v", err)
			}

			hasDisable := strings.Contains(buf.String(), disableMarker)
			if hasDisable != tc.wantDisable {
				t.Errorf("disable form present=%v, want=%v (IsAD=%v, Enabled=%v)",
					hasDisable, tc.wantDisable, tc.isAD, tc.enabled)
			}
		})
	}
}
