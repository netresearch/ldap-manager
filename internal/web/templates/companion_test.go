package templates

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/netresearch/ldap-manager/internal/web/static"
)

func TestLoginWizardProgressiveEnhancement(t *testing.T) {
	var out bytes.Buffer
	if err := LoginV2(nil, "test", "test-csrf").Render(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{`character="wizard"`, `action="special"`, `src="/static/logo.webp"`, `src="/static/companion/wizard-login.js"`, `name="csrf_token" value="test-csrf"`, `autocomplete="current-password"`} {
		if !strings.Contains(out.String(), required) {
			t.Errorf("missing login enhancement or form contract: %s", required)
		}
	}
	for _, asset := range []string{"avatar.css", "wizard-login.js", "scormiq-avatar.js", "gopher-rigs.js", "vendor/three.module.js", "assets/logos/netresearch-symbol-only.svg"} {
		if data, err := static.Static.ReadFile("companion/" + asset); err != nil || len(data) == 0 {
			t.Errorf("companion asset %s is not embedded: %v", asset, err)
		}
	}
}
