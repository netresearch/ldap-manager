// Package ldap_cache — graph builder: walks the cached user/group/computer
// tables plus OU DNs to produce a concentric relationship graph rooted at
// a focal entity. Pure in-memory; no LDAP round-trips.
//
//nolint:revive // package name intentionally uses underscore
package ldap_cache

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	goldap "github.com/go-ldap/ldap/v3"
	ldap "github.com/netresearch/simple-ldap-go"
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

// Caps from spec §4.3. Used by applyCaps (Task 6).
const (
	graphMaxNodesPerRing = 60
	graphMaxNodesTotal   = 200
)

// BuildGraph walks the cache starting at focus and returns the concentric
// graph truncated by the depth and cap constants. Depth is clamped to
// [1, 3] (out-of-range values silently round to nearest, as per the spec
// endpoint contract).
//
// Focus resolution order: user → group → computer → OU. The first match
// wins; if the DN matches none, returns ErrGraphNotFound.
//
// Layout ((ring, angle) per node) is applied before returning — callers
// get a ready-to-render GraphData.
func (m *Manager) BuildGraph(focusDN string, depth int) (*GraphData, error) {
	if depth < 1 {
		depth = 1
	} else if depth > 3 {
		depth = 3
	}

	// Resolve focus type.
	if u, ok := m.Users.FindByDN(focusDN); ok {
		data := m.buildGraphFromUser(*u, depth)
		assignConcentric(data)

		return data, nil
	}
	if g, ok := m.Groups.FindByDN(focusDN); ok {
		data := m.buildGraphFromGroup(*g, depth)
		assignConcentric(data)

		return data, nil
	}
	if c, ok := m.Computers.FindByDN(focusDN); ok {
		data := m.buildGraphFromComputer(*c, depth)
		assignConcentric(data)

		return data, nil
	}
	// OU focus: any DN whose immediate RDN is ou=.
	if parsed, err := goldap.ParseDN(focusDN); err == nil &&
		len(parsed.RDNs) > 0 &&
		strings.EqualFold(parsed.RDNs[0].Attributes[0].Type, "ou") {
		data := m.buildGraphFromOU(focusDN, depth)
		assignConcentric(data)

		return data, nil
	}

	return nil, fmt.Errorf("%w: %q", ErrGraphNotFound, focusDN)
}

func (m *Manager) buildGraphFromGroup(g ldap.Group, depth int) *GraphData {
	data := &GraphData{Focus: g.DN(), Depth: depth}
	data.Nodes = append(data.Nodes, groupNode(g, 0, false))
	seen := map[string]int{g.DN(): 0}

	// Ring 1: members (users, computers, or nested groups) + parent groups
	// (any cached group whose Members contains g.DN()).
	for _, memberDN := range g.Members {
		addRingMember(data, seen, m, memberDN, 1, Edge{Source: memberDN, Target: g.DN(), Kind: EdgeMemberOf})
	}

	for _, parent := range m.Groups.Get() {
		for _, memDN := range parent.Members {
			if memDN == g.DN() {
				if _, dup := seen[parent.DN()]; !dup {
					data.Nodes = append(data.Nodes, groupNode(parent, 1, true))
					seen[parent.DN()] = 1
				}

				data.Edges = append(data.Edges, Edge{Source: g.DN(), Target: parent.DN(), Kind: EdgeMemberOf})

				break
			}
		}
	}

	if depth >= 2 {
		for _, n := range nodesInRing(data, 1) {
			m.expandNode(data, seen, n, 2)
		}
	}

	if depth >= 3 {
		for _, n := range nodesInRing(data, 2) {
			m.expandNode(data, seen, n, 3)
		}
	}

	applyCaps(data)

	return data
}

func (m *Manager) buildGraphFromComputer(c ldap.Computer, depth int) *GraphData {
	data := &GraphData{Focus: c.DN(), Depth: depth}
	data.Nodes = append(data.Nodes, computerNode(c, 0))
	seen := map[string]int{c.DN(): 0}

	for _, gDN := range c.Groups {
		if _, dup := seen[gDN]; dup {
			continue
		}
		if g, ok := m.Groups.FindByDN(gDN); ok {
			data.Nodes = append(data.Nodes, groupNode(*g, 1, true))
			seen[gDN] = 1
			data.Edges = append(data.Edges, Edge{Source: c.DN(), Target: gDN, Kind: EdgeMemberOf})
		}
	}

	if ouDN := immediateOUFromDN(c.DN()); ouDN != "" {
		addOU(data, seen, ouDN, 1, Edge{Source: ouDN, Target: c.DN(), Kind: EdgeContains})
	}

	if depth >= 2 {
		for _, n := range nodesInRing(data, 1) {
			m.expandNode(data, seen, n, 2)
		}
	}

	if depth >= 3 {
		for _, n := range nodesInRing(data, 2) {
			m.expandNode(data, seen, n, 3)
		}
	}

	applyCaps(data)

	return data
}

func (m *Manager) buildGraphFromOU(dn string, depth int) *GraphData {
	data := &GraphData{Focus: dn, Depth: depth}
	data.Nodes = append(data.Nodes, Node{DN: dn, Type: NodeOU, Label: labelForOU(dn), Ring: 0, Expandable: false})
	seen := map[string]int{dn: 0}

	m.addOUChildren(data, seen, dn, 1)

	if depth >= 2 {
		for _, n := range nodesInRing(data, 1) {
			m.expandNode(data, seen, n, 2)
		}
	}

	if depth >= 3 {
		for _, n := range nodesInRing(data, 2) {
			m.expandNode(data, seen, n, 3)
		}
	}

	applyCaps(data)

	return data
}

// assignConcentric writes (ring, angle) onto each Node. Ring is already
// set by the builder; this function picks angles. Nodes in the same ring
// are sorted by (Type, Label, DN) for determinism, then evenly spaced.
func assignConcentric(d *GraphData) {
	buckets := map[int][]*Node{}
	for i := range d.Nodes {
		n := &d.Nodes[i]
		buckets[n.Ring] = append(buckets[n.Ring], n)
	}

	for ring, nodes := range buckets {
		if ring == 0 {
			for _, n := range nodes {
				n.Angle = 0
			}

			continue
		}

		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].Type != nodes[j].Type {
				return nodes[i].Type < nodes[j].Type
			}
			if nodes[i].Label != nodes[j].Label {
				return nodes[i].Label < nodes[j].Label
			}

			return nodes[i].DN < nodes[j].DN
		})

		count := len(nodes)
		for i, n := range nodes {
			n.Angle = float64(i) * 2 * math.Pi / float64(count)
		}
	}
}

func (m *Manager) buildGraphFromUser(user ldap.User, depth int) *GraphData {
	data := &GraphData{Focus: user.DN(), Depth: depth}
	data.Nodes = append(data.Nodes, userNode(user, 0, true))

	seen := map[string]int{user.DN(): 0}

	// Ring 1: direct groups + immediate OU.
	for _, groupDN := range user.Groups {
		if _, dup := seen[groupDN]; dup {
			continue
		}
		if g, ok := m.Groups.FindByDN(groupDN); ok {
			data.Nodes = append(data.Nodes, groupNode(*g, 1, true))
			seen[groupDN] = 1
			data.Edges = append(data.Edges, Edge{Source: user.DN(), Target: groupDN, Kind: EdgeMemberOf})
		}
	}

	if ouDN := immediateOUFromDN(user.DN()); ouDN != "" {
		data.Nodes = append(data.Nodes, Node{DN: ouDN, Type: NodeOU, Label: labelForOU(ouDN), Ring: 1, Expandable: true})
		seen[ouDN] = 1
		data.Edges = append(data.Edges, Edge{Source: ouDN, Target: user.DN(), Kind: EdgeContains})
	}

	if depth < 2 {
		applyCaps(data)

		return data
	}

	// Ring 2: per-ring-1 expansion.
	for _, n1 := range nodesInRing(data, 1) {
		m.expandNode(data, seen, n1, 2)
	}

	if depth < 3 {
		applyCaps(data)

		return data
	}

	// Ring 3: one more hop.
	for _, n2 := range nodesInRing(data, 2) {
		m.expandNode(data, seen, n2, 3)
	}

	applyCaps(data)

	return data
}

// expandNode adds the 1-hop neighbourhood of n to data at ring `targetRing`.
// Skips nodes already in `seen`. Source/target on edges reflects the
// underlying relationship direction (memberOf or contains), not the walk
// direction.
func (m *Manager) expandNode(data *GraphData, seen map[string]int, n Node, targetRing int) {
	switch n.Type {
	case NodeUser:
		if u, ok := m.Users.FindByDN(n.DN); ok {
			for _, gDN := range u.Groups {
				addRingMember(data, seen, m, gDN, targetRing, Edge{Source: u.DN(), Target: gDN, Kind: EdgeMemberOf})
			}

			if ouDN := immediateOUFromDN(u.DN()); ouDN != "" {
				addOU(data, seen, ouDN, targetRing, Edge{Source: ouDN, Target: u.DN(), Kind: EdgeContains})
			}
		}
	case NodeGroup:
		if g, ok := m.Groups.FindByDN(n.DN); ok {
			for _, memberDN := range g.Members {
				addRingMember(data, seen, m, memberDN, targetRing, Edge{Source: memberDN, Target: g.DN(), Kind: EdgeMemberOf})
			}
		}
	case NodeComputer:
		if c, ok := m.Computers.FindByDN(n.DN); ok {
			for _, gDN := range c.Groups {
				addRingMember(data, seen, m, gDN, targetRing, Edge{Source: c.DN(), Target: gDN, Kind: EdgeMemberOf})
			}
		}
	case NodeOU:
		m.addOUChildren(data, seen, n.DN, targetRing)
	}
}

func userNode(u ldap.User, ring int, expandable bool) Node {
	enabled := u.Enabled

	return Node{DN: u.DN(), Type: NodeUser, Label: u.CN(), Ring: ring, Enabled: &enabled, Expandable: expandable}
}

func groupNode(g ldap.Group, ring int, expandable bool) Node {
	count := len(g.Members)

	return Node{DN: g.DN(), Type: NodeGroup, Label: g.CN(), Ring: ring, MemberCount: &count, Expandable: expandable}
}

func computerNode(c ldap.Computer, ring int) Node {
	enabled := c.Enabled

	return Node{DN: c.DN(), Type: NodeComputer, Label: c.CN(), Ring: ring, Enabled: &enabled, Expandable: false}
}

func immediateOUFromDN(dn string) string {
	parsed, err := goldap.ParseDN(dn)
	if err != nil || len(parsed.RDNs) < 2 {
		return ""
	}

	tail := parsed.RDNs[1:]
	if len(tail[0].Attributes) == 0 || !strings.EqualFold(tail[0].Attributes[0].Type, "ou") {
		return ""
	}

	return (&goldap.DN{RDNs: tail}).String()
}

func labelForOU(dn string) string {
	parsed, err := goldap.ParseDN(dn)
	if err != nil || len(parsed.RDNs) == 0 {
		return dn
	}

	return parsed.RDNs[0].Attributes[0].Type + "=" + parsed.RDNs[0].Attributes[0].Value
}

func nodesInRing(d *GraphData, ring int) []Node {
	out := make([]Node, 0, len(d.Nodes))

	for _, n := range d.Nodes {
		if n.Ring == ring {
			out = append(out, n)
		}
	}

	return out
}

func addRingMember(data *GraphData, seen map[string]int, m *Manager, dn string, ring int, edge Edge) {
	if _, dup := seen[dn]; dup {
		if !hasEdge(data, edge) {
			data.Edges = append(data.Edges, edge)
		}

		return
	}

	if u, ok := m.Users.FindByDN(dn); ok {
		data.Nodes = append(data.Nodes, userNode(*u, ring, false))
	} else if g, ok := m.Groups.FindByDN(dn); ok {
		data.Nodes = append(data.Nodes, groupNode(*g, ring, true))
	} else if c, ok := m.Computers.FindByDN(dn); ok {
		data.Nodes = append(data.Nodes, computerNode(*c, ring))
	} else {
		return
	}

	seen[dn] = ring
	data.Edges = append(data.Edges, edge)
}

func addOU(data *GraphData, seen map[string]int, ouDN string, ring int, edge Edge) {
	if _, dup := seen[ouDN]; !dup {
		data.Nodes = append(data.Nodes, Node{DN: ouDN, Type: NodeOU, Label: labelForOU(ouDN), Ring: ring, Expandable: true})
		seen[ouDN] = ring
	}

	if !hasEdge(data, edge) {
		data.Edges = append(data.Edges, edge)
	}
}

func (m *Manager) addOUChildren(data *GraphData, seen map[string]int, ouDN string, ring int) {
	ouParsed, err := goldap.ParseDN(ouDN)
	if err != nil {
		return
	}

	isImmediateChild := func(childDN string) bool {
		p, err := goldap.ParseDN(childDN)
		if err != nil || len(p.RDNs) != len(ouParsed.RDNs)+1 {
			return false
		}

		for i, rdn := range ouParsed.RDNs {
			if !rdn.Equal(p.RDNs[i+1]) {
				return false
			}
		}

		return true
	}

	for _, u := range m.Users.Get() {
		if !isImmediateChild(u.DN()) {
			continue
		}

		if _, dup := seen[u.DN()]; dup {
			continue
		}

		data.Nodes = append(data.Nodes, userNode(u, ring, false))
		seen[u.DN()] = ring
		data.Edges = append(data.Edges, Edge{Source: ouDN, Target: u.DN(), Kind: EdgeContains})
	}

	for _, c := range m.Computers.Get() {
		if !isImmediateChild(c.DN()) {
			continue
		}

		if _, dup := seen[c.DN()]; dup {
			continue
		}

		data.Nodes = append(data.Nodes, computerNode(c, ring))
		seen[c.DN()] = ring
		data.Edges = append(data.Edges, Edge{Source: ouDN, Target: c.DN(), Kind: EdgeContains})
	}
}

func hasEdge(data *GraphData, e Edge) bool {
	for _, existing := range data.Edges {
		if existing.Source == e.Source && existing.Target == e.Target && existing.Kind == e.Kind {
			return true
		}
	}

	return false
}

func applyCaps(data *GraphData) {
	available := len(data.Nodes)

	// Per-ring cap: sort within each ring by (Type, Label, DN) and drop
	// beyond graphMaxNodesPerRing. Record overflow.
	buckets := map[int][]Node{}
	for _, n := range data.Nodes {
		buckets[n.Ring] = append(buckets[n.Ring], n)
	}

	truncated := false
	kept := make([]Node, 0, len(data.Nodes))

	for ring := 0; ring <= 3; ring++ {
		b, ok := buckets[ring]
		if !ok {
			continue
		}

		sort.Slice(b, func(i, j int) bool {
			if b[i].Type != b[j].Type {
				return b[i].Type < b[j].Type
			}
			if b[i].Label != b[j].Label {
				return b[i].Label < b[j].Label
			}

			return b[i].DN < b[j].DN
		})

		ringCap := graphMaxNodesPerRing
		if ring == 0 {
			ringCap = 1
		}

		if len(b) > ringCap {
			truncated = true
			b = b[:ringCap]
		}

		kept = append(kept, b...)
	}

	// Total cap: trim the tail until we're within the total budget.
	if len(kept) > graphMaxNodesTotal {
		truncated = true
		kept = kept[:graphMaxNodesTotal]
	}

	// Drop edges whose endpoints didn't survive the cap.
	keptDN := make(map[string]bool, len(kept))
	for _, n := range kept {
		keptDN[n.DN] = true
	}

	filteredEdges := make([]Edge, 0, len(data.Edges))

	for _, e := range data.Edges {
		if keptDN[e.Source] && keptDN[e.Target] {
			filteredEdges = append(filteredEdges, e)
		}
	}

	data.Nodes = kept
	data.Edges = filteredEdges
	data.Overflow = Overflow{
		Truncated: truncated,
		Rendered:  len(kept),
		Available: available,
	}
}
