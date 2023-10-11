package static

import "embed"

//go:embed *.css *.png *.ico *.svg *.webp site.webmanifest browserconfig.xml
var Static embed.FS
