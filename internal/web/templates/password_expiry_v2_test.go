package templates

import (
	"strings"
	"testing"
)

func TestExpiryBadgeClassAndLabel(t *testing.T) {
	tests := []struct {
		name      string
		row       ExpiryRow
		wantClass string
		wantLabel string
	}{
		{
			name:      "expired dated account is danger/Expired",
			row:       ExpiryRow{Status: "expires", HasDeadline: true, Expired: true},
			wantClass: "list-row__badge drawer__badge--danger",
			wantLabel: "Expired",
		},
		{
			name:      "future dated account is warn/Expiring",
			row:       ExpiryRow{Status: "expires", HasDeadline: true, Expired: false},
			wantClass: "list-row__badge drawer__badge--warn",
			wantLabel: "Expiring",
		},
		{
			name:      "must-change is danger (it is blocked now)",
			row:       ExpiryRow{Status: "must-change", Expired: true},
			wantClass: "list-row__badge drawer__badge--danger",
			wantLabel: "Must change",
		},
		{
			name:      "never-expires is a muted pill",
			row:       ExpiryRow{Status: "never-expires"},
			wantClass: "list-row__badge",
			wantLabel: "Never expires",
		},
		{
			name:      "unknown is a muted pill",
			row:       ExpiryRow{Status: "unknown"},
			wantClass: "list-row__badge",
			wantLabel: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expiryBadgeClass(tt.row); got != tt.wantClass {
				t.Errorf("class = %q, want %q", got, tt.wantClass)
			}
			if got := expiryBadgeLabel(tt.row); got != tt.wantLabel {
				t.Errorf("label = %q, want %q", got, tt.wantLabel)
			}
		})
	}
}

func TestExpiryDeadlineLabel(t *testing.T) {
	dated := ExpiryRow{HasDeadline: true, ExpiresAt: 1785585600} // 2026-08-01 00:00 UTC
	if got := expiryDeadlineLabel(dated); !strings.HasPrefix(got, "2026-08-01") {
		t.Errorf("dated label = %q, want it to start with the date", got)
	}

	undated := ExpiryRow{HasDeadline: false}
	if got := expiryDeadlineLabel(undated); got != "—" {
		t.Errorf("undated label = %q, want an em dash", got)
	}
}

func TestExpiryFilterQS_CarriesWindowAndScope(t *testing.T) {
	if got := expiryFilterQS(7, false); got != "days=7" {
		t.Errorf("due-only QS = %q, want days=7", got)
	}

	got := expiryFilterQS(14, true)
	if !strings.Contains(got, "days=14") || !strings.Contains(got, "show=all") {
		t.Errorf("show-all QS = %q, want both days=14 and show=all", got)
	}
}

func TestExpiryHrefIsSafe(t *testing.T) {
	// The href must be a well-formed same-origin path — no scheme, no host.
	got := string(expiryHref(30, true))
	if !strings.HasPrefix(got, "/password-expiry?") {
		t.Errorf("href = %q, want a same-origin /password-expiry path", got)
	}
}
