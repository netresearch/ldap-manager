package web

// HTTP helpers for computer entities. The V2 handlers
// (computers_v2_handler.go) serve the list and detail routes; this file
// retains a DN lookup helper used by the cache-backed V2 handler.

import (
	ldap "github.com/netresearch/simple-ldap-go"
)

// findComputerByDN searches for a computer by DN in a slice.
func findComputerByDN(computers []ldap.Computer, dn string) *ldap.Computer {
	for i := range computers {
		if computers[i].DN() == dn {
			return &computers[i]
		}
	}

	return nil
}
