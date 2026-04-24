// Package ldap_cache — graph builder: walks the cached user/group/computer
// tables plus OU DNs to produce a concentric relationship graph rooted at
// a focal entity. Pure in-memory; no LDAP round-trips.
//
//nolint:revive // package name intentionally uses underscore
package ldap_cache

import (
	"errors"
)

// NodeType enumerates the four entity kinds the graph can display.
type NodeType string

const (
	NodeUser     NodeType = "user"
	NodeGroup    NodeType = "group"
	NodeComputer NodeType = "computer"
	NodeOU       NodeType = "ou"
)

// EdgeKind enumerates the relationship kinds v1 ships. Extensible:
// future values (manager, managedBy) can land without changing the
// JSON shape.
type EdgeKind string

const (
	EdgeMemberOf EdgeKind = "memberOf" // user/computer/group → group
	EdgeContains EdgeKind = "contains" // ou → user/computer
)

// Node is a single entry in the graph. Angle is in radians, [0, 2π).
// Ring 0 is the focus (entity-centred mode); rings 1+ are BFS distances.
// List-page Graph mode uses ring 1 for groups and ring 2 for users — see
// the spec §4.4.
type Node struct {
	DN          string   `json:"dn"`
	Type        NodeType `json:"type"`
	Label       string   `json:"label"`
	Ring        int      `json:"ring"`
	Angle       float64  `json:"angle"`
	Enabled     *bool    `json:"enabled,omitempty"`     // user / computer only
	MemberCount *int     `json:"memberCount,omitempty"` // group only
	Expandable  bool     `json:"expandable"`
}

// Edge is a directed relationship between two Node.DNs.
type Edge struct {
	Source string   `json:"source"`
	Target string   `json:"target"`
	Kind   EdgeKind `json:"kind"`
}

// Overflow reports whether the BFS walk hit the caps. If truncated, the
// Nodes / Edges slices are a subset and the UI renders a "showing N of
// M" indicator.
type Overflow struct {
	Truncated bool `json:"truncated"`
	Rendered  int  `json:"rendered"`
	Available int  `json:"available"`
}

// GraphData is the full JSON payload the handler returns. Focus is "" in
// list-page Graph mode (no single focal entity) — see spec §4.4.
type GraphData struct {
	Focus    string   `json:"focus"`
	Depth    int      `json:"depth"`
	Nodes    []Node   `json:"nodes"`
	Edges    []Edge   `json:"edges"`
	Overflow Overflow `json:"overflow"`
}

// ErrGraphNotFound is returned when BuildGraph is asked for a DN not
// present in any cache. The handler translates this to HTTP 404.
var ErrGraphNotFound = errors.New("graph: focus DN not found in cache")

// Caps from spec §4.3.
const (
	graphMaxNodesPerRing = 60
	graphMaxNodesTotal   = 200
)
