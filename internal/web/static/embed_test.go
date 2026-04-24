package static

import (
	"io/fs"
	"testing"
)

// TestVendorFilesEmbedded asserts that the vendored frontend files refreshed
// by scripts/vendor.sh are embedded into the binary. Prevents silent 404s at
// runtime when the //go:embed directive omits a path.
func TestVendorFilesEmbedded(t *testing.T) {
	want := []string{
		"app.css",
		"vendor/pico.min.css",
		"vendor/htmx.min.js",
		"vendor/alpine.min.js",
	}
	for _, name := range want {
		info, err := fs.Stat(Static, name)
		if err != nil {
			t.Errorf("embedded FS missing %q: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("embedded %q is empty", name)
		}
	}
}
