package web

import "testing"

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
