// internal/web/ou_rail.go — distinct-OU helpers for the list-page OU rail.
package web

import (
	"sort"

	ldap "github.com/netresearch/simple-ldap-go"
)

// distinctImmediateOUs returns the sorted unique immediate OU RDNs
// extracted from a slice of DNs. Empty-OU DNs (entries that live directly
// under a dc=) are skipped; the rail shows only OUs worth filtering by.
func distinctImmediateOUs(dns []string) []string {
	seen := make(map[string]struct{}, len(dns))

	out := make([]string, 0, len(dns))
	for _, dn := range dns {
		ou := immediateOU(dn)
		if ou == "" {
			continue
		}

		if _, ok := seen[ou]; ok {
			continue
		}

		seen[ou] = struct{}{}
		out = append(out, ou)
	}

	sort.Strings(out)

	return out
}

// distinctImmediateOUsFromUsers is the entity-specific adapter used by
// handleUsersV2. Kept as a thin wrapper so the handler stays readable and
// the underlying helper stays DN-generic.
func distinctImmediateOUsFromUsers(users []ldap.User) []string {
	dns := make([]string, 0, len(users))
	for _, u := range users {
		dns = append(dns, u.DN())
	}

	return distinctImmediateOUs(dns)
}

// distinctImmediateOUsFromGroups mirrors the user helper for groups.
func distinctImmediateOUsFromGroups(groups []ldap.Group) []string {
	dns := make([]string, 0, len(groups))
	for _, g := range groups {
		dns = append(dns, g.DN())
	}

	return distinctImmediateOUs(dns)
}

// distinctImmediateOUsFromComputers mirrors the user helper for computers.
func distinctImmediateOUsFromComputers(computers []ldap.Computer) []string {
	dns := make([]string, 0, len(computers))
	for _, cp := range computers {
		dns = append(dns, cp.DN())
	}

	return distinctImmediateOUs(dns)
}
