package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAssetManifest(t *testing.T) {
	t.Run("loads valid manifest", func(t *testing.T) {
		// Create temporary manifest file
		tmpDir := t.TempDir()
		manifestPath := filepath.Join(tmpDir, "manifest.json")
		manifestContent := `{
			"styles.css": "styles.abc123.css",
			"generated": "2025-01-01T00:00:00Z",
			"hash": "abc123"
		}`
		err := os.WriteFile(manifestPath, []byte(manifestContent), 0o600)
		require.NoError(t, err)

		// Load manifest
		manifest, err := LoadAssetManifest(manifestPath)
		require.NoError(t, err)
		assert.Equal(t, "styles.abc123.css", manifest.StylesCSS)
		assert.Equal(t, "abc123", manifest.Hash)
		assert.Equal(t, "styles.abc123.css", manifest.Assets["styles.css"])
	})

	t.Run("returns default when file missing", func(t *testing.T) {
		manifest, err := LoadAssetManifest("/nonexistent/manifest.json")
		require.NoError(t, err)
		assert.Equal(t, "styles.css", manifest.StylesCSS)
		assert.Equal(t, "styles.css", manifest.Assets["styles.css"])
	})

	t.Run("handles invalid JSON gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()
		manifestPath := filepath.Join(tmpDir, "manifest.json")
		err := os.WriteFile(manifestPath, []byte("invalid json"), 0o600)
		require.NoError(t, err)

		_, err = LoadAssetManifest(manifestPath)
		assert.Error(t, err)
	})
}

func TestGetCachedManifest(t *testing.T) {
	t.Run("caches manifest on first load", func(t *testing.T) {
		// Clear cache before test
		manifestCache = nil

		tmpDir := t.TempDir()
		manifestPath := filepath.Join(tmpDir, "manifest.json")
		manifestContent := `{
			"styles.css": "styles.def456.css",
			"generated": "2025-01-01T00:00:00Z",
			"hash": "def456"
		}`
		err := os.WriteFile(manifestPath, []byte(manifestContent), 0o600)
		require.NoError(t, err)

		// First call should load and cache
		manifest1 := GetCachedManifest(manifestPath)
		assert.Equal(t, "styles.def456.css", manifest1.StylesCSS)

		// Second call should return cached version
		manifest2 := GetCachedManifest(manifestPath)
		assert.Equal(t, manifest1, manifest2)
	})

	t.Run("returns default on error", func(t *testing.T) {
		// Clear cache before test
		manifestCache = nil

		tmpDir := t.TempDir()
		manifestPath := filepath.Join(tmpDir, "manifest.json")
		// Write invalid JSON
		err := os.WriteFile(manifestPath, []byte("not json"), 0o600)
		require.NoError(t, err)

		manifest := GetCachedManifest(manifestPath)
		assert.Equal(t, "styles.css", manifest.StylesCSS)
	})
}

func TestAssetManifestGetAssetPath(t *testing.T) {
	manifest := &AssetManifest{
		Assets: map[string]string{
			"styles.css": "styles.xyz789.css",
			"app.js":     "app.123abc.js",
		},
	}

	t.Run("returns hashed path when exists", func(t *testing.T) {
		assert.Equal(t, "styles.xyz789.css", manifest.GetAssetPath("styles.css"))
		assert.Equal(t, "app.123abc.js", manifest.GetAssetPath("app.js"))
	})

	t.Run("returns original name when not found", func(t *testing.T) {
		assert.Equal(t, "unknown.css", manifest.GetAssetPath("unknown.css"))
	})
}

func TestAssetManifestGetStylesPath(t *testing.T) {
	t.Run("returns hashed styles path", func(t *testing.T) {
		manifest := &AssetManifest{
			Assets: map[string]string{
				"styles.css": "styles.test123.css",
			},
		}
		assert.Equal(t, "styles.test123.css", manifest.GetStylesPath())
	})

	t.Run("returns default when styles.css not in manifest", func(t *testing.T) {
		manifest := &AssetManifest{
			Assets: map[string]string{},
		}
		assert.Equal(t, "styles.css", manifest.GetStylesPath())
	})
}
