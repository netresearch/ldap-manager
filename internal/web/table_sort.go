// internal/web/table_sort.go — sort helpers for the Table view
// (List | Table | Graph). Sort key + direction are server-side
// query params (?sort=col&dir=asc|desc), so the URL captures the
// sort state and back/forward navigation works without JS.
package web

import (
	"sort"
	"strings"

	ldap "github.com/netresearch/simple-ldap-go"
)

// normaliseSortDir clamps an unknown direction to "asc". Used so an
// arbitrary ?dir=foo doesn't turn into a bug downstream.
func normaliseSortDir(d string) string {
	if d == "desc" {
		return "desc"
	}

	return "asc"
}

// sortUsersTable sorts users in place by the requested column. Unknown
// sort keys fall back to CN — the default column the table is
// initially shown in.
func sortUsersTable(users []ldap.User, key, dir string) {
	dir = normaliseSortDir(dir)

	less := func(i, j int) bool {
		var a, b string

		switch key {
		case "sam":
			a, b = users[i].SAMAccountName, users[j].SAMAccountName
		case "mail":
			if users[i].Mail != nil {
				a = *users[i].Mail
			}
			if users[j].Mail != nil {
				b = *users[j].Mail
			}
		case "status":
			a, b = userStatusForSort(users[i]), userStatusForSort(users[j])
		default: // "cn"
			a, b = users[i].CN(), users[j].CN()
		}

		if a == b {
			// Stable secondary key on DN so otherwise-equal rows have a
			// deterministic order across renders.
			return users[i].DN() < users[j].DN()
		}

		if dir == "desc" {
			return strings.ToLower(a) > strings.ToLower(b)
		}

		return strings.ToLower(a) < strings.ToLower(b)
	}

	sort.SliceStable(users, less)
}

// sortGroupsTable sorts groups by the requested column.
func sortGroupsTable(groups []ldap.Group, key, dir string) {
	dir = normaliseSortDir(dir)

	less := func(i, j int) bool {
		switch key {
		case "members":
			a, b := len(groups[i].Members), len(groups[j].Members)
			if a == b {
				return groups[i].DN() < groups[j].DN()
			}

			if dir == "desc" {
				return a > b
			}

			return a < b
		case "dn":
			a, b := groups[i].DN(), groups[j].DN()
			if dir == "desc" {
				return strings.ToLower(a) > strings.ToLower(b)
			}

			return strings.ToLower(a) < strings.ToLower(b)
		default: // "cn"
			a, b := groups[i].CN(), groups[j].CN()
			if a == b {
				return groups[i].DN() < groups[j].DN()
			}

			if dir == "desc" {
				return strings.ToLower(a) > strings.ToLower(b)
			}

			return strings.ToLower(a) < strings.ToLower(b)
		}
	}

	sort.SliceStable(groups, less)
}

// sortComputersTable sorts computers by the requested column.
func sortComputersTable(computers []ldap.Computer, key, dir string) {
	dir = normaliseSortDir(dir)

	less := func(i, j int) bool {
		var a, b string

		switch key {
		case "sam":
			a, b = computers[i].SAMAccountName, computers[j].SAMAccountName
		case "status":
			a, b = computerStatusForSort(computers[i]), computerStatusForSort(computers[j])
		case "dn":
			a, b = computers[i].DN(), computers[j].DN()
		default: // "cn"
			a, b = computers[i].CN(), computers[j].CN()
		}

		if a == b {
			return computers[i].DN() < computers[j].DN()
		}

		if dir == "desc" {
			return strings.ToLower(a) > strings.ToLower(b)
		}

		return strings.ToLower(a) < strings.ToLower(b)
	}

	sort.SliceStable(computers, less)
}

// userStatusForSort maps Enabled→"a" / Disabled→"b" so an asc sort
// puts enabled rows on top — the more useful default than alphabetical
// "Disabled" / "Enabled".
func userStatusForSort(u ldap.User) string {
	if u.Enabled {
		return "a"
	}

	return "b"
}

// computerStatusForSort mirrors userStatusForSort.
func computerStatusForSort(c ldap.Computer) string {
	if c.Enabled {
		return "a"
	}

	return "b"
}
