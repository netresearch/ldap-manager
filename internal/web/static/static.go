// Package static provides embedded static web assets for the LDAP Manager web interface.
// Includes CSS stylesheets, images, icons, and web manifest files.
package static

import "embed"

// Static contains all embedded static web assets including CSS, images, and configuration files.
//
//go:embed *.css *.png *.ico *.svg *.webp site.webmanifest browserconfig.xml
var Static embed.FS
