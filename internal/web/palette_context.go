// Package web — helpers for seeding the command palette's empty-query state.
package web

import "github.com/netresearch/ldap-manager/internal/web/templates"

// paletteContextFor returns the pinned slice a page's palette should seed
// from on empty-query open. Safe on nil pinnedStore / empty viewerDN /
// pinnedStore errors — returns nil in any failure mode so the caller can
// pass it straight into paletteV2WithPinned.
func (a *App) paletteContextFor(viewerDN string) []templates.PinnedEntry {
	if viewerDN == "" {
		return nil
	}

	entries, err := a.pinnedEntriesFor(viewerDN)
	if err != nil {
		return nil
	}

	return entries
}
