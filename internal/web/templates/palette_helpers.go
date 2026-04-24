// Package templates — palette helpers shared by palette_v2.templ.
package templates

import "encoding/json"

// pinnedJSON returns a JSON array suitable for a data-pinned attribute
// on the palette dialog. Empty-safe: returns "[]" on nil/empty input or
// on marshal failure.
func pinnedJSON(pinned []PinnedEntry) string {
	if len(pinned) == 0 {
		return "[]"
	}

	out := make([]map[string]string, 0, len(pinned))
	for _, p := range pinned {
		out = append(out, map[string]string{
			"type": p.Type,
			"dn":   p.DN,
			"cn":   p.CN,
		})
	}

	b, err := json.Marshal(out)
	if err != nil {
		return "[]"
	}

	return string(b)
}
