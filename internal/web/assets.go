package web

import (
	"encoding/json"
	"os"
)

// AssetManifest represents the asset manifest structure
type AssetManifest struct {
	Assets    map[string]string `json:"assets,omitempty"`
	Generated string            `json:"generated,omitempty"`
	Hash      string            `json:"hash,omitempty"`
	// Direct mapping for backwards compatibility
	StylesCSS string `json:"styles.css,omitempty"`
}

// LoadAssetManifest loads the asset manifest from disk
func LoadAssetManifest(manifestPath string) (*AssetManifest, error) {
	// Check if file exists
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		// Return default manifest if file doesn't exist
		return &AssetManifest{
			Assets: map[string]string{
				"styles.css": "styles.css",
			},
			StylesCSS: "styles.css",
		}, nil
	}

	data, err := os.ReadFile(manifestPath) // #nosec G304 - manifestPath is a trusted static path
	if err != nil {
		return nil, err
	}

	var manifest AssetManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	// Handle backwards compatibility
	if manifest.Assets == nil {
		manifest.Assets = make(map[string]string)
	}

	// Map direct properties to assets map
	if manifest.StylesCSS != "" {
		manifest.Assets["styles.css"] = manifest.StylesCSS
	}

	return &manifest, nil
}

// GetAssetPath returns the hashed path for an asset
func (m *AssetManifest) GetAssetPath(assetName string) string {
	if hashedName, exists := m.Assets[assetName]; exists {
		return hashedName
	}

	// Fallback to original name
	return assetName
}

// GetStylesPath returns the path to the styles CSS file
func (m *AssetManifest) GetStylesPath() string {
	return m.GetAssetPath("styles.css")
}
