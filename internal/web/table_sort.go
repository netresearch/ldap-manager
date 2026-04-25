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

// stringLess applies the requested direction to a case-insensitive
// comparison. The DN secondary key needs to be applied when the two
// inputs are equal under the same case-insensitive rule the primary
// comparison uses — earlier `a == b` (case-sensitive) skipped the
// tie-break for inputs that differed only in case, leaving ordering
// at the mercy of input-slice order.
func stringLess(a, b, tieA, tieB, dir string) bool {
	if strings.EqualFold(a, b) {
		return tieA < tieB
	}

	la, lb := strings.ToLower(a), strings.ToLower(b)
	if dir == "desc" {
		return la > lb
	}

	return la < lb
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

		return stringLess(a, b, users[i].DN(), users[j].DN(), dir)
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
			return stringLess(groups[i].DN(), groups[j].DN(), groups[i].DN(), groups[j].DN(), dir)
		default: // "cn"
			return stringLess(groups[i].CN(), groups[j].CN(), groups[i].DN(), groups[j].DN(), dir)
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

		return stringLess(a, b, computers[i].DN(), computers[j].DN(), dir)
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
