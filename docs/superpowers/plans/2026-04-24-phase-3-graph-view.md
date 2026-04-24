# Phase 3 Relationship Graph View — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the Phase 3 relationship graph view per the design spec — a dedicated `/graph?entity=<dn>&depth=<N>` route plus a `List | Graph` mode toggle on the three list pages, with click-to-pivot and click-to-expand interactions, hand-rolled SVG rendering, and an always-visible AAA parallel edge table.

**Architecture:** Go server walks the existing `ldap_cache` via BFS to build a `{nodes, edges, overflow}` JSON blob with server-assigned `(ring, angle)` for each node. Handler renders a Templ page embedding that JSON in a `<script type="application/json">` plus an SSR fallback `<table>` of edges. A plain-JS client (`v2-graph.js`) parses the JSON, paints SVG, wires pan/zoom/keyboard-nav/click-to-expand. Click-to-expand fetches `/api/graph.json?entity=<clicked-dn>&depth=1` and merges. No graph library — concentric layout is closed-form geometry.

**Tech Stack:** Go 1.26, Fiber v2, Templ, `github.com/netresearch/simple-ldap-go`, Pico CSS v2 (vendored), htmx v2 (vendored), Alpine CSP build (vendored). No new dependencies.

**Spec reference:** [`docs/superpowers/specs/2026-04-24-phase-3-graph-view-design.md`](../specs/2026-04-24-phase-3-graph-view-design.md).

**Commits:** one atomic commit per task, conventional-commit prefix (`feat(graph):`, `test(graph):`, `docs(graph):`, `refactor(cache):`), signed (`-S`) and signed-off (`--signoff`). No AI-attribution lines.

**Worktree:** this plan assumes implementation on a branch `feat/phase-3-graph-view` off current `main`. Create before starting Slice 1.

---

## File Structure

### New files

| Path | Responsibility |
| --- | --- |
| `internal/ldap_cache/graph.go` | `Node`, `Edge`, `GraphData`, `GraphFocus` types. `BuildGraph(focus, depth)` BFS walker. `assignConcentric(data)` layout math. `NodeType`, `EdgeKind` enums. |
| `internal/ldap_cache/graph_test.go` | Unit tests for builder (per focus type, depth, caps, cycles), layout (determinism, even distribution). |
| `internal/web/graph_v2_handler.go` | `handleGraphV2` (HTML), `handleGraphJSON` (application/json), helpers for DN validation and depth clamping. |
| `internal/web/graph_v2_handler_test.go` | Handler tests (ETag, 400, 404, depth clamping, both response types). |
| `internal/web/graph_integration_test.go` | Integration tests against the OpenLDAP test container (skip when unavailable). |
| `internal/web/templates/graph_v2.templ` | `GraphPageV2`, `graphNode`, `graphEdgeLine`, `graphEdgeTable`, `graphSlider` components. |
| `internal/web/static/js/v2-graph.js` | Canvas render, pan/zoom, keyboard nav, click-to-pivot, click-to-expand, aria-live announcements. |

### Modified files

| Path | Change |
| --- | --- |
| `internal/web/server.go` | Register `/graph`, `/api/graph.json`. Add `view=graph` branch to list handlers is wired via handler dispatch (see Slice 5 per-handler changes). |
| `internal/web/users_v2_handler.go` | Early-return `view=graph` branch before list-view rendering. |
| `internal/web/groups_v2_handler.go` | Same. |
| `internal/web/computers_v2_handler.go` | Same. |
| `internal/web/templates/users_v2.templ` | Add `List \| Graph` segmented control above the filter chips row. |
| `internal/web/templates/groups_v2.templ` | Same. |
| `internal/web/templates/computers_v2.templ` | Same. |
| `internal/web/templates/user_drawer_fragment.templ` | Add `View relationships` pivot anchor in the pivots section. |
| `internal/web/templates/group_drawer_fragment.templ` | Same. |
| `internal/web/templates/computer_drawer_fragment.templ` | Same. |
| `internal/web/templates/base_v2.templ` | `<script defer src="/static/js/v2-graph.js">` only when `graph` body flag is set (pass-through CSP-safe). |
| `internal/web/static/app.css` | Append graph section: `--graph-edge`, `--graph-edge-focus`, `--graph-node-border`, `--graph-node-focus-ring` tokens; `.graph-canvas`, `.graph-node`, `.graph-edge`, `.graph-table`, `.graph-slider`, `.graph-segmented` rules. |
| `internal/web/contrast_test.go` | Add new graph tokens to the contrast matrix. |
| `README.md` | Add graph conformance statement to the accessibility section. |

### Not modified

Legacy `users.go`, `groups.go`, `computers.go` handlers — those are the pre-revamp routes retained for transitional reasons and out of scope here.

---

## Pre-flight — Verify Open Assumptions

Spec §13 lists five assumptions that MUST be verified before implementation. Each gets a cheap empirical check. If one fails, stop and update the spec before continuing.

### Task 0.1 — ldap.ParseDN escaped-comma handling

**Files:** none modified; this is a REPL-style check.

- [ ] **Step 1: Confirm the search-index handler already uses `ldap.ParseDN`**

Run: `grep -n 'ldap.ParseDN' internal/web/search_index.go`
Expected: matches `immediateOU` calling `goldap.ParseDN(dn)`.

- [ ] **Step 2: Write a throwaway test asserting escaped-comma behaviour**

Create `internal/ldap_cache/parsedn_check_test.go` (will be deleted in Step 4):

```go
package ldap_cache

import (
	"testing"

	goldap "github.com/go-ldap/ldap/v3"
)

func TestParseDNEscapedComma(t *testing.T) {
	parsed, err := goldap.ParseDN(`cn=Last\, First,ou=Sales,dc=example,dc=com`)
	if err != nil {
		t.Fatalf("ParseDN: %v", err)
	}
	if len(parsed.RDNs) != 4 {
		t.Fatalf("expected 4 RDNs, got %d", len(parsed.RDNs))
	}
	if got := parsed.RDNs[0].Attributes[0].Value; got != "Last, First" {
		t.Errorf("expected CN value %q, got %q", "Last, First", got)
	}
}
```

- [ ] **Step 3: Run it**

Run: `go test ./internal/ldap_cache/ -run TestParseDNEscapedComma -count=1 -v`
Expected: PASS.

- [ ] **Step 4: Delete the throwaway test**

Run: `rm internal/ldap_cache/parsedn_check_test.go`

Assumption 2 confirmed. No commit.

### Task 0.2 — Cache version counter exists and is exposed

**Files:** none modified.

- [ ] **Step 1: Inspect the existing search-index ETag derivation**

Run: `grep -nE 'ETag|sha256|cache.*version' internal/web/search_index.go`
Expected: `handleSearchIndex` hashes the JSON body directly (not a counter). The function is our reference — ETag over the marshalled payload works because the `sort.Slice` gives stable ordering.

- [ ] **Step 2: Confirm the same approach applies to graph JSON**

Reading `buildSearchIndex` reveals it re-materialises every request. Our graph builder will do the same. Rather than exposing a separate cache version, hash the graph JSON body (mirror of search-index). No code change required; update assumption 3 in the spec's `Open assumptions` section from "cache-version hash" to "hash of the marshalled graph JSON body, like `handleSearchIndex`".

- [ ] **Step 3: Update the spec**

Edit `docs/superpowers/specs/2026-04-24-phase-3-graph-view-design.md` §4.2:

Replace:
```
Set `ETag` to `sha256(focus-dn + depth + cache-version)[:16]`, mirror the `/api/search-index.json` pattern.
```
With:
```
Set `ETag` to `sha256(<marshalled JSON body>)[:16]`, mirror the `/api/search-index.json` pattern (where the same approach gives a stable ETag as long as the cache contents and the builder inputs are the same).
```

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/specs/2026-04-24-phase-3-graph-view-design.md
git commit -S --signoff -m "docs(spec): clarify graph ETag is hash of body (like search-index)"
```

### Task 0.3 — OU-children scan performance

**Files:** none modified.

- [ ] **Step 1: Benchmark the worst-case scan**

Add a throwaway benchmark `internal/ldap_cache/ou_children_bench_test.go`:

```go
package ldap_cache

import (
	"strings"
	"testing"

	ldap "github.com/netresearch/simple-ldap-go"
)

func BenchmarkOUChildrenScan10k(b *testing.B) {
	users := make([]ldap.User, 10000)
	for i := range users {
		users[i] = NewMockUser("", "u"+string(rune('a'+i%26)), true, nil)
	}

	// We can't inject DNs via simple-ldap-go's unexported fields — this
	// benchmark measures the hot path shape only (iteration + strings.HasSuffix)
	// and documents the expected N.
	parent := "ou=Engineering,dc=example,dc=com"
	suffix := "," + parent
	count := 0

	b.ResetTimer()
	for b.Loop() {
		count = 0
		for _, u := range users {
			if strings.HasSuffix(u.DN(), suffix) {
				count++
			}
		}
	}
	_ = count
}
```

- [ ] **Step 2: Run the benchmark**

Run: `go test ./internal/ldap_cache/ -bench BenchmarkOUChildrenScan10k -benchtime=1s -run='^$'`
Expected: <200 μs per iteration on a modern laptop. Document the number.

- [ ] **Step 3: Delete the benchmark**

Run: `rm internal/ldap_cache/ou_children_bench_test.go`

Decision: if the scan is under 500 μs, proceed without a new index (YAGNI). If it exceeds 500 μs, add an `ouChildrenIndex map[string][]*T` to `Cache[T]` in a separate PR before starting Slice 1.

### Task 0.4 — Group-of-group membership

**Files:** none modified.

- [ ] **Step 1: Inspect how `Group.Members` is populated**

Run: `grep -rn 'Members' /home/cybot/go/pkg/mod/github.com/netresearch/simple-ldap-go@v1.12.0/groups.go | head`
Expected: `Members` is a `[]string` populated from LDAP `member` attribute with whatever DNs the directory returns (user, group, computer, or all).

- [ ] **Step 2: Confirm the BFS walk must treat unknown-type DNs as groups-if-found**

No code change. The builder (Slice 1) will resolve each `Members` DN against all three caches — if it's found in `m.Groups`, the edge is `memberOf` to that group and we recurse; if in `m.Users` / `m.Computers`, the edge is `memberOf` to that entity (leaf).

- [ ] **Step 3: Spec update**

Assumption 4 in the spec's §13 is already phrased correctly — no change.

### Task 0.5 — prefers-reduced-motion side effects

**Files:** none modified.

- [ ] **Step 1: Audit existing JS for motion handlers**

Run: `grep -rn 'prefers-reduced-motion\|matchMedia' internal/web/static/js/`
Expected: only `v2-preferences-init.js` references it (for the density auto-select). No animation JS bypasses the user preference.

- [ ] **Step 2: Confirm CSS honours it**

Run: `grep -n 'prefers-reduced-motion' internal/web/static/app.css`
Expected: a `@media (prefers-reduced-motion: reduce) { * { transition-duration: 0.01ms !important; ... } }` block.

Assumption 5 confirmed. The new `v2-graph.js` must therefore call `matchMedia('(prefers-reduced-motion: reduce)').matches` and gate all its own transition durations accordingly.

---

## Slice 1 — Graph Builder

Pure Go layer. No handler, no template, no client. This slice ends with a fully-tested `ldap_cache.BuildGraph(focus, depth) (*GraphData, error)` that downstream slices consume.

### Task 1 — Types and enums

**Files:**
- Create: `internal/ldap_cache/graph.go`

- [ ] **Step 1: Define the core types**

Write to `internal/ldap_cache/graph.go`:

```go
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

// Caps from spec §4.3.
const (
	graphMaxNodesPerRing = 60
	graphMaxNodesTotal   = 200
)
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/ldap_cache/`
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/ldap_cache/graph.go
git commit -S --signoff -m "feat(cache): add graph types (Node, Edge, GraphData, enums)"
```

### Task 2 — BuildGraph for user focus

**Files:**
- Modify: `internal/ldap_cache/graph.go`
- Create: `internal/ldap_cache/graph_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/ldap_cache/graph_test.go`:

```go
package ldap_cache

import (
	"testing"

	ldap "github.com/netresearch/simple-ldap-go"
)

// Fixture: bob is member of "admins" (contains carol) and "engineers"
// (contains dave + alice). "engineers" is a member of "all-staff".
// OU=Engineering contains bob, alice, dave.
func graphFixture(t *testing.T) *Manager {
	t.Helper()
	m := &mockLDAPClient{
		users: []ldap.User{
			NewMockUser("cn=bob,ou=Engineering,dc=ex,dc=com", "bob", true,
				[]string{"cn=admins,ou=Groups,dc=ex,dc=com", "cn=engineers,ou=Groups,dc=ex,dc=com"}),
			NewMockUser("cn=carol,ou=Sales,dc=ex,dc=com", "carol", true,
				[]string{"cn=admins,ou=Groups,dc=ex,dc=com"}),
			NewMockUser("cn=dave,ou=Engineering,dc=ex,dc=com", "dave", true,
				[]string{"cn=engineers,ou=Groups,dc=ex,dc=com"}),
			NewMockUser("cn=alice,ou=Engineering,dc=ex,dc=com", "alice", true,
				[]string{"cn=engineers,ou=Groups,dc=ex,dc=com"}),
		},
		groups: []ldap.Group{
			{Members: []string{"cn=bob,ou=Engineering,dc=ex,dc=com", "cn=carol,ou=Sales,dc=ex,dc=com"}},        // admins
			{Members: []string{"cn=bob,ou=Engineering,dc=ex,dc=com", "cn=dave,ou=Engineering,dc=ex,dc=com", "cn=alice,ou=Engineering,dc=ex,dc=com"}}, // engineers
			{Members: []string{"cn=engineers,ou=Groups,dc=ex,dc=com"}},                                         // all-staff
		},
	}
	manager := New(m)
	manager.Refresh()
	return manager
}

func TestBuildGraph_UserFocus_Depth1(t *testing.T) {
	t.Skip("mock DNs are empty — enable once graph builder matches by CN/SAMAccountName for tests")
	// Real assertion pending a test-friendly DN seeding path. See Task 2.4.
}
```

Run: `go test ./internal/ldap_cache/ -run TestBuildGraph -count=1`
Expected: test SKIPS with the note.

- [ ] **Step 2: Implement BuildGraph for a user focus (skeleton)**

Append to `internal/ldap_cache/graph.go`:

```go
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

// buildGraphFromUser BFS from a user focal.
// Ring 0: the user. Ring 1: groups user is memberOf + user's immediate OU.
// Ring 2 (depth >= 2): other members of those groups, other users in the OU,
// parent groups (if group-of-group membership resolves to cached groups).
// Ring 3 (depth == 3): one further hop from ring-2 nodes.
func (m *Manager) buildGraphFromUser(user ldap.User, depth int) *GraphData {
	// Implementation deferred to Step 3 — this is the structural stub.
	return &GraphData{Focus: user.DN(), Depth: depth}
}

// buildGraphFromGroup, buildGraphFromComputer, buildGraphFromOU — defined
// in Task 3, 4, 5 respectively.
func (m *Manager) buildGraphFromGroup(g ldap.Group, depth int) *GraphData {
	return &GraphData{Focus: g.DN(), Depth: depth}
}
func (m *Manager) buildGraphFromComputer(c ldap.Computer, depth int) *GraphData {
	return &GraphData{Focus: c.DN(), Depth: depth}
}
func (m *Manager) buildGraphFromOU(dn string, depth int) *GraphData {
	return &GraphData{Focus: dn, Depth: depth}
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
```

- [ ] **Step 3: Implement buildGraphFromUser BFS**

Replace the stub in `internal/ldap_cache/graph.go`:

```go
func (m *Manager) buildGraphFromUser(user ldap.User, depth int) *GraphData {
	data := &GraphData{Focus: user.DN(), Depth: depth}
	data.Nodes = append(data.Nodes, userNode(user, 0, true))

	seen := map[string]int{user.DN(): 0}
	enabled := user.Enabled
	_ = enabled // set on userNode

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

// Helpers (trim for brevity in plan — full impl in Task 6 with caps):
// - userNode, groupNode, computerNode, ouNode construct Node from cache entry.
// - immediateOUFromDN, labelForOU parse RDN.
// - nodesInRing returns a slice of nodes at a given ring (snapshot).
// - addRingMember resolves a DN against any cache and adds if new.
// - addOU adds an OU node and a contains edge.
// - addOUChildren scans m.Users + m.Computers for direct children under an OU.
// - applyCaps enforces per-ring and total caps.

// Stubs so the file compiles — real bodies in Task 6.
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
	var buf strings.Builder
	for i, r := range tail {
		if i > 0 {
			buf.WriteString(",")
		}
		for _, a := range r.Attributes {
			buf.WriteString(a.Type)
			buf.WriteString("=")
			buf.WriteString(a.Value)
		}
	}
	out := buf.String()
	// Only return if first RDN is ou=
	if len(tail) > 0 && strings.EqualFold(tail[0].Attributes[0].Type, "ou") {
		return out
	}
	return ""
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
		// Edge still matters if we haven't already recorded it.
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
	suffix := "," + ouDN
	for _, u := range m.Users.Get() {
		if strings.HasSuffix(u.DN(), suffix) && !strings.Contains(strings.TrimSuffix(u.DN(), suffix), ",") == false {
			// Immediate child only
			if _, dup := seen[u.DN()]; dup {
				continue
			}
			data.Nodes = append(data.Nodes, userNode(u, ring, false))
			seen[u.DN()] = ring
			data.Edges = append(data.Edges, Edge{Source: ouDN, Target: u.DN(), Kind: EdgeContains})
		}
	}
	// Mirror for computers — see Task 4.
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
	// Implemented in Task 6.
	data.Overflow.Rendered = len(data.Nodes)
	data.Overflow.Available = len(data.Nodes)
}
```

> **Note to implementer:** the `addOUChildren` "immediate child only" check above has a bug in the boolean logic — re-write as `!strings.Contains(strings.TrimSuffix(u.DN(), suffix), ",")` (drop the double-negation). Task 5 revisits this.

- [ ] **Step 4: Add a DN-aware test fixture path**

To test properly without simple-ldap-go's unexported DN fields, add a helper to `internal/ldap_cache/test_helpers.go`:

```go
// seedCacheWithDNs is a test-only helper that bypasses the mockLDAPClient
// path and populates caches directly with synthetic items whose DN() works.
// Used by graph_test.go because the graph builder relies on FindByDN which
// requires a non-empty DN.
func seedCacheWithDNs(t *testing.T, m *Manager, users, groups, computers []any) {
	// Stub — implementation in Task 2 Step 5.
	t.Helper()
}
```

- [ ] **Step 5: Replace the mock-based fixture with a direct seed**

The builder needs real DNs. Since `ldap.User.Object.dn` is unexported, use `reflect.Value.Elem().FieldByName` with `unsafe`:

```go
// In internal/ldap_cache/test_helpers.go
import (
	"reflect"
	"unsafe"
)

// newUserWithDN builds an ldap.User with a real DN by poking the unexported
// Object.dn / Object.cn fields. Test-only; production code builds Users via
// simple-ldap-go's objectFromEntry.
func newUserWithDN(dn, cn, sam string, enabled bool, groups []string) ldap.User {
	u := ldap.User{SAMAccountName: sam, Enabled: enabled, Groups: groups}
	setObjectFields(reflect.ValueOf(&u).Elem().FieldByName("Object"), dn, cn)
	return u
}
func newGroupWithDN(dn, cn string, members []string) ldap.Group {
	g := ldap.Group{Members: members}
	setObjectFields(reflect.ValueOf(&g).Elem().FieldByName("Object"), dn, cn)
	return g
}
func newComputerWithDN(dn, cn, sam string, enabled bool, groups []string) ldap.Computer {
	c := ldap.Computer{SAMAccountName: sam, Enabled: enabled, Groups: groups}
	setObjectFields(reflect.ValueOf(&c).Elem().FieldByName("Object"), dn, cn)
	return c
}
func setObjectFields(obj reflect.Value, dn, cn string) {
	dnField := obj.FieldByName("dn")
	cnField := obj.FieldByName("cn")
	reflect.NewAt(dnField.Type(), unsafe.Pointer(dnField.UnsafeAddr())).Elem().SetString(dn)
	reflect.NewAt(cnField.Type(), unsafe.Pointer(cnField.UnsafeAddr())).Elem().SetString(cn)
}
```

- [ ] **Step 6: Rewrite the test to use DN-aware helpers**

Replace `graphFixture` body and un-skip the test:

```go
func graphFixture(t *testing.T) *Manager {
	t.Helper()
	m := &mockLDAPClient{}
	manager := New(m)
	// Seed directly, bypassing the mockLDAPClient path.
	manager.Users.setAll([]ldap.User{
		newUserWithDN("cn=bob,ou=Engineering,dc=ex,dc=com", "bob", "bob", true, []string{
			"cn=admins,ou=Groups,dc=ex,dc=com", "cn=engineers,ou=Groups,dc=ex,dc=com",
		}),
		newUserWithDN("cn=carol,ou=Sales,dc=ex,dc=com", "carol", "carol", true, []string{
			"cn=admins,ou=Groups,dc=ex,dc=com",
		}),
		newUserWithDN("cn=dave,ou=Engineering,dc=ex,dc=com", "dave", "dave", true, []string{
			"cn=engineers,ou=Groups,dc=ex,dc=com",
		}),
		newUserWithDN("cn=alice,ou=Engineering,dc=ex,dc=com", "alice", "alice", true, []string{
			"cn=engineers,ou=Groups,dc=ex,dc=com",
		}),
	})
	manager.Groups.setAll([]ldap.Group{
		newGroupWithDN("cn=admins,ou=Groups,dc=ex,dc=com", "admins", []string{
			"cn=bob,ou=Engineering,dc=ex,dc=com", "cn=carol,ou=Sales,dc=ex,dc=com",
		}),
		newGroupWithDN("cn=engineers,ou=Groups,dc=ex,dc=com", "engineers", []string{
			"cn=bob,ou=Engineering,dc=ex,dc=com", "cn=dave,ou=Engineering,dc=ex,dc=com", "cn=alice,ou=Engineering,dc=ex,dc=com",
		}),
		newGroupWithDN("cn=all-staff,ou=Groups,dc=ex,dc=com", "all-staff", []string{
			"cn=engineers,ou=Groups,dc=ex,dc=com",
		}),
	})
	return manager
}

func TestBuildGraph_UserFocus_Depth1(t *testing.T) {
	m := graphFixture(t)
	data, err := m.BuildGraph("cn=bob,ou=Engineering,dc=ex,dc=com", 1)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	if data.Focus != "cn=bob,ou=Engineering,dc=ex,dc=com" {
		t.Errorf("focus: %q", data.Focus)
	}
	if data.Depth != 1 {
		t.Errorf("depth: %d", data.Depth)
	}
	// Expect: bob (ring 0) + 2 groups (ring 1) + 1 OU (ring 1) = 4 nodes
	if got := len(data.Nodes); got != 4 {
		t.Errorf("node count at depth 1: got %d, want 4", got)
	}
	// Expect edges: bob→admins (memberOf), bob→engineers (memberOf),
	// ou=Engineering→bob (contains)
	if got := len(data.Edges); got != 3 {
		t.Errorf("edge count at depth 1: got %d, want 3", got)
	}
}
```

- [ ] **Step 7: Run and iterate until green**

Run: `go test ./internal/ldap_cache/ -run TestBuildGraph_UserFocus_Depth1 -count=1 -v`
Expected: PASS (may require iterating on helper bugs — see implementer note on `addOUChildren`).

- [ ] **Step 8: Commit**

```bash
git add internal/ldap_cache/graph.go internal/ldap_cache/graph_test.go internal/ldap_cache/test_helpers.go
git commit -S --signoff -m "feat(cache): BuildGraph user focus at depth 1 with BFS walk"
```

### Task 3 — BuildGraph for group focus

**Files:**
- Modify: `internal/ldap_cache/graph.go` (replace `buildGraphFromGroup` stub)
- Modify: `internal/ldap_cache/graph_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/ldap_cache/graph_test.go`:

```go
func TestBuildGraph_GroupFocus_Depth1(t *testing.T) {
	m := graphFixture(t)
	data, err := m.BuildGraph("cn=engineers,ou=Groups,dc=ex,dc=com", 1)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	// engineers has 3 user members + is a member of all-staff
	// Expect: engineers (ring 0) + 3 users (ring 1) + all-staff (ring 1) = 5 nodes
	if got := len(data.Nodes); got != 5 {
		t.Errorf("node count: got %d, want 5", got)
	}
}
```

- [ ] **Step 2: Implement `buildGraphFromGroup`**

Replace the stub in `internal/ldap_cache/graph.go`:

```go
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
```

- [ ] **Step 3: Run + iterate**

Run: `go test ./internal/ldap_cache/ -run TestBuildGraph_GroupFocus -count=1 -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/ldap_cache/graph.go internal/ldap_cache/graph_test.go
git commit -S --signoff -m "feat(cache): BuildGraph group focus — members + parent groups"
```

### Task 4 — BuildGraph for computer focus

**Files:**
- Modify: `internal/ldap_cache/graph.go` (replace `buildGraphFromComputer` stub)
- Modify: `internal/ldap_cache/graph_test.go`

- [ ] **Step 1: Extend the fixture with a computer**

Append to `graphFixture` in `graph_test.go`:

```go
manager.Computers.setAll([]ldap.Computer{
	newComputerWithDN("cn=ws01,ou=Computers,dc=ex,dc=com", "ws01", "ws01$", true, []string{
		"cn=engineers,ou=Groups,dc=ex,dc=com",
	}),
})
```

- [ ] **Step 2: Write the failing test**

```go
func TestBuildGraph_ComputerFocus_Depth1(t *testing.T) {
	m := graphFixture(t)
	data, err := m.BuildGraph("cn=ws01,ou=Computers,dc=ex,dc=com", 1)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	// Expect: ws01 (ring 0) + engineers (ring 1) + ou=Computers (ring 1) = 3
	if got := len(data.Nodes); got != 3 {
		t.Errorf("node count: got %d, want 3", got)
	}
}
```

- [ ] **Step 3: Implement**

```go
func (m *Manager) buildGraphFromComputer(c ldap.Computer, depth int) *GraphData {
	data := &GraphData{Focus: c.DN(), Depth: depth}
	data.Nodes = append(data.Nodes, computerNode(c, 0))
	seen := map[string]int{c.DN(): 0}

	for _, gDN := range c.Groups {
		if g, ok := m.Groups.FindByDN(gDN); ok {
			if _, dup := seen[gDN]; !dup {
				data.Nodes = append(data.Nodes, groupNode(*g, 1, true))
				seen[gDN] = 1
			}
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
```

- [ ] **Step 4: Run + commit**

Run: `go test ./internal/ldap_cache/ -run TestBuildGraph_ComputerFocus -count=1 -v`

```bash
git add internal/ldap_cache/graph.go internal/ldap_cache/graph_test.go
git commit -S --signoff -m "feat(cache): BuildGraph computer focus"
```

### Task 5 — BuildGraph for OU focus (+ addOUChildren fix)

**Files:**
- Modify: `internal/ldap_cache/graph.go`
- Modify: `internal/ldap_cache/graph_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestBuildGraph_OUFocus_Depth1(t *testing.T) {
	m := graphFixture(t)
	data, err := m.BuildGraph("ou=Engineering,dc=ex,dc=com", 1)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	// Expect: ou=Engineering (ring 0) + bob + dave + alice (ring 1) = 4
	if got := len(data.Nodes); got != 4 {
		t.Errorf("node count: got %d, want 4", got)
	}
}
```

- [ ] **Step 2: Fix `addOUChildren` and implement `buildGraphFromOU`**

Replace both in `graph.go`:

```go
func (m *Manager) addOUChildren(data *GraphData, seen map[string]int, ouDN string, ring int) {
	suffix := "," + ouDN
	for _, u := range m.Users.Get() {
		if !strings.HasSuffix(u.DN(), suffix) {
			continue
		}
		rel := strings.TrimSuffix(u.DN(), suffix)
		if strings.Contains(rel, ",") {
			continue // not an immediate child
		}
		if _, dup := seen[u.DN()]; dup {
			continue
		}
		data.Nodes = append(data.Nodes, userNode(u, ring, false))
		seen[u.DN()] = ring
		data.Edges = append(data.Edges, Edge{Source: ouDN, Target: u.DN(), Kind: EdgeContains})
	}
	for _, c := range m.Computers.Get() {
		if !strings.HasSuffix(c.DN(), suffix) {
			continue
		}
		rel := strings.TrimSuffix(c.DN(), suffix)
		if strings.Contains(rel, ",") {
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
```

- [ ] **Step 3: Run + commit**

Run: `go test ./internal/ldap_cache/ -run TestBuildGraph_OUFocus -count=1 -v`

```bash
git add internal/ldap_cache/graph.go internal/ldap_cache/graph_test.go
git commit -S --signoff -m "feat(cache): BuildGraph OU focus + fix addOUChildren immediate-child check"
```

### Task 6 — Implement caps (per-ring and total)

**Files:**
- Modify: `internal/ldap_cache/graph.go`
- Modify: `internal/ldap_cache/graph_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestBuildGraph_PerRingCap(t *testing.T) {
	// Seed 100 users in one group; expect the ring-1 cap to trim to 60.
	manager := New(&mockLDAPClient{})
	users := make([]ldap.User, 100)
	userDNs := make([]string, 100)
	for i := range users {
		dn := fmt.Sprintf("cn=u%03d,ou=Users,dc=ex,dc=com", i)
		users[i] = newUserWithDN(dn, fmt.Sprintf("u%03d", i), fmt.Sprintf("u%d", i), true,
			[]string{"cn=big,ou=Groups,dc=ex,dc=com"})
		userDNs[i] = dn
	}
	manager.Users.setAll(users)
	manager.Groups.setAll([]ldap.Group{
		newGroupWithDN("cn=big,ou=Groups,dc=ex,dc=com", "big", userDNs),
	})

	data, err := manager.BuildGraph("cn=big,ou=Groups,dc=ex,dc=com", 1)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	ring1 := 0
	for _, n := range data.Nodes {
		if n.Ring == 1 {
			ring1++
		}
	}
	if ring1 != graphMaxNodesPerRing {
		t.Errorf("ring-1 size: got %d, want %d", ring1, graphMaxNodesPerRing)
	}
	if !data.Overflow.Truncated {
		t.Error("Overflow.Truncated should be true")
	}
	if data.Overflow.Available < 100 {
		t.Errorf("Overflow.Available: got %d, want >= 100", data.Overflow.Available)
	}
}
```

Add `"fmt"` to imports in `graph_test.go` if not present.

- [ ] **Step 2: Replace `applyCaps` with the real implementation**

Replace in `graph.go`:

```go
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
		cap := graphMaxNodesPerRing
		if ring == 0 {
			cap = 1
		}
		if len(b) > cap {
			truncated = true
			b = b[:cap]
		}
		kept = append(kept, b...)
	}

	// Total cap: if cumulative length exceeds graphMaxNodesTotal, trim
	// the largest ring first.
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
```

- [ ] **Step 3: Run + commit**

Run: `go test ./internal/ldap_cache/ -run TestBuildGraph_PerRingCap -count=1 -v`

```bash
git add internal/ldap_cache/graph.go internal/ldap_cache/graph_test.go
git commit -S --signoff -m "feat(cache): enforce per-ring (60) and total (200) graph caps"
```

### Task 7 — Concentric layout determinism + even distribution

**Files:**
- Modify: `internal/ldap_cache/graph_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestAssignConcentric_Deterministic(t *testing.T) {
	m := graphFixture(t)
	data1, _ := m.BuildGraph("cn=bob,ou=Engineering,dc=ex,dc=com", 2)
	data2, _ := m.BuildGraph("cn=bob,ou=Engineering,dc=ex,dc=com", 2)
	if len(data1.Nodes) != len(data2.Nodes) {
		t.Fatalf("node count mismatch: %d vs %d", len(data1.Nodes), len(data2.Nodes))
	}
	for i := range data1.Nodes {
		if data1.Nodes[i].DN != data2.Nodes[i].DN {
			t.Errorf("node[%d] DN differs: %q vs %q", i, data1.Nodes[i].DN, data2.Nodes[i].DN)
		}
		if data1.Nodes[i].Angle != data2.Nodes[i].Angle {
			t.Errorf("node[%d] angle differs: %f vs %f", i, data1.Nodes[i].Angle, data2.Nodes[i].Angle)
		}
	}
}

func TestAssignConcentric_EvenDistribution(t *testing.T) {
	m := graphFixture(t)
	data, _ := m.BuildGraph("cn=bob,ou=Engineering,dc=ex,dc=com", 1)
	var ring1 []float64
	for _, n := range data.Nodes {
		if n.Ring == 1 {
			ring1 = append(ring1, n.Angle)
		}
	}
	if len(ring1) < 2 {
		t.Skipf("ring 1 has %d nodes, cannot test spacing", len(ring1))
	}
	sort.Float64s(ring1)
	expected := 2 * math.Pi / float64(len(ring1))
	for i := 1; i < len(ring1); i++ {
		gap := ring1[i] - ring1[i-1]
		if math.Abs(gap-expected) > 1e-9 {
			t.Errorf("gap[%d]: got %f, want %f", i, gap, expected)
		}
	}
}
```

- [ ] **Step 2: Run**

Run: `go test ./internal/ldap_cache/ -run TestAssignConcentric -count=1 -v`
Expected: PASS (assignConcentric was already implemented in Task 2).

- [ ] **Step 3: Commit**

```bash
git add internal/ldap_cache/graph_test.go
git commit -S --signoff -m "test(cache): assignConcentric determinism + even distribution"
```

### Task 8 — Cycle detection in group-of-group walks

**Files:**
- Modify: `internal/ldap_cache/graph_test.go`

- [ ] **Step 1: Write the failing test**

Cycle: A is member of B, B is member of A.

```go
func TestBuildGraph_CycleSafe(t *testing.T) {
	manager := New(&mockLDAPClient{})
	manager.Groups.setAll([]ldap.Group{
		newGroupWithDN("cn=A,ou=Groups,dc=ex,dc=com", "A", []string{"cn=B,ou=Groups,dc=ex,dc=com"}),
		newGroupWithDN("cn=B,ou=Groups,dc=ex,dc=com", "B", []string{"cn=A,ou=Groups,dc=ex,dc=com"}),
	})

	done := make(chan struct{})
	go func() {
		_, _ = manager.BuildGraph("cn=A,ou=Groups,dc=ex,dc=com", 3)
		close(done)
	}()
	select {
	case <-done:
		// completed within deadline
	case <-time.After(500 * time.Millisecond):
		t.Fatal("BuildGraph cycled forever")
	}
}
```

Add `"time"` to imports in `graph_test.go`.

- [ ] **Step 2: Verify it passes**

The existing `seen` map in `expandNode`/`addRingMember` already prevents revisits. Run:

Run: `go test ./internal/ldap_cache/ -run TestBuildGraph_CycleSafe -count=1 -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/ldap_cache/graph_test.go
git commit -S --signoff -m "test(cache): cycle-safe BuildGraph (A→B→A does not infinite-loop)"
```

### Task 9 — Slice 1 wrap

**Files:** none.

- [ ] **Step 1: Run the full cache test suite**

Run: `go test ./internal/ldap_cache/ -count=1`
Expected: all green.

- [ ] **Step 2: Run the linter on new code**

Run: `golangci-lint run ./internal/ldap_cache/...`
Expected: 0 issues.

- [ ] **Step 3: Write a summary commit (no code)**

The individual tasks committed are each atomic — no wrap commit needed unless the linter flagged formatting. Proceed to Slice 2.

---

## Slice 2 — JSON Endpoint

### Task 10 — Handler skeleton

**Files:**
- Create: `internal/web/graph_v2_handler.go`

- [ ] **Step 1: Write the handler**

Create `internal/web/graph_v2_handler.go`:

```go
// internal/web/graph_v2_handler.go — /graph and /api/graph.json.
package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	goldap "github.com/go-ldap/ldap/v3"
	"github.com/gofiber/fiber/v2"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
)

// handleGraphJSON serves /api/graph.json?entity=<dn>&depth=<N>. Response
// shape documented in the spec §4.1. ETag is sha256 of the marshalled
// body to mirror /api/search-index.json.
func (a *App) handleGraphJSON(c *fiber.Ctx) error {
	data, err := a.buildGraphFromQuery(c)
	if err != nil {
		return err
	}

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal graph: %w", err)
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`

	if match := c.Get("If-None-Match"); match != "" && match == etag {
		return c.SendStatus(fiber.StatusNotModified)
	}

	c.Set("Content-Type", "application/json; charset=utf-8")
	c.Set("ETag", etag)
	c.Set("Cache-Control", "private, must-revalidate")
	return c.Send(body)
}

// buildGraphFromQuery parses ?entity= and ?depth= from c and returns the
// resulting graph, clamping depth and returning 400/404 where appropriate.
func (a *App) buildGraphFromQuery(c *fiber.Ctx) (*ldap_cache.GraphData, error) {
	entity := c.Query("entity")
	if entity == "" {
		return nil, c.Status(fiber.StatusBadRequest).SendString("missing entity")
	}
	if _, err := goldap.ParseDN(entity); err != nil {
		return nil, c.Status(fiber.StatusBadRequest).SendString("invalid DN")
	}

	depth := 2
	if raw := c.Query("depth"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			depth = n
		}
	}
	// Clamping happens inside BuildGraph.

	data, err := a.ldapCache.BuildGraph(entity, depth)
	if err != nil {
		if errors.Is(err, ldap_cache.ErrGraphNotFound) {
			return nil, c.Status(fiber.StatusNotFound).SendString("entity not found")
		}
		return nil, fmt.Errorf("build graph: %w", err)
	}
	return data, nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/web/`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/web/graph_v2_handler.go
git commit -S --signoff -m "feat(web): /api/graph.json handler skeleton with ETag + validation"
```

### Task 11 — Wire route

**Files:**
- Modify: `internal/web/server.go`

- [ ] **Step 1: Register the route**

Find the protected routes block (search: `protected.Get("/api/search-index.json"`) and add:

```go
protected.Get("/api/graph.json", a.handleGraphJSON)
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add internal/web/server.go
git commit -S --signoff -m "feat(web): register /api/graph.json route"
```

### Task 12 — Handler tests (unit)

**Files:**
- Create: `internal/web/graph_v2_handler_test.go`

- [ ] **Step 1: Write the tests**

```go
package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
)

func TestHandleGraphJSON_MissingEntity(t *testing.T) {
	app, _ := setupFullTestApp(t)
	cookies := createAuthSession(t, app.sessionStore)

	req := httptest.NewRequest("GET", "/api/graph.json", nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	resp, err := app.fiber.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleGraphJSON_InvalidDN(t *testing.T) {
	app, _ := setupFullTestApp(t)
	cookies := createAuthSession(t, app.sessionStore)

	req := httptest.NewRequest("GET", "/api/graph.json?entity=not~a~dn", nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	resp, _ := app.fiber.Test(req)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleGraphJSON_UnknownDN(t *testing.T) {
	app, _ := setupFullTestApp(t)
	cookies := createAuthSession(t, app.sessionStore)

	req := httptest.NewRequest("GET", "/api/graph.json?entity=cn=ghost,dc=ex,dc=com", nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	resp, _ := app.fiber.Test(req)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestHandleGraphJSON_ETagStable(t *testing.T) {
	app, _ := setupFullTestApp(t)
	// Seed a user into the cache via the test helper (no LDAP needed).
	app.ldapCache.Users.setAll(...)  // use seedCacheWithDNs from slice 1
	cookies := createAuthSession(t, app.sessionStore)

	req1 := httptest.NewRequest("GET", "/api/graph.json?entity=cn=bob,dc=ex,dc=com", nil)
	for _, ck := range cookies {
		req1.AddCookie(ck)
	}
	resp1, _ := app.fiber.Test(req1)
	etag := resp1.Header.Get("ETag")
	_ = resp1.Body.Close()

	req2 := httptest.NewRequest("GET", "/api/graph.json?entity=cn=bob,dc=ex,dc=com", nil)
	for _, ck := range cookies {
		req2.AddCookie(ck)
	}
	req2.Header.Set("If-None-Match", etag)
	resp2, _ := app.fiber.Test(req2)
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusNotModified {
		t.Errorf("status: got %d, want 304", resp2.StatusCode)
	}
}

func TestHandleGraphJSON_DepthClamping(t *testing.T) {
	// Depth=0 clamps to 1; depth=99 clamps to 3.
	app, _ := setupFullTestApp(t)
	app.ldapCache.Users.setAll(...) // seed bob
	cookies := createAuthSession(t, app.sessionStore)

	for _, raw := range []string{"0", "99", "-5"} {
		url := "/api/graph.json?entity=cn=bob,dc=ex,dc=com&depth=" + raw
		req := httptest.NewRequest("GET", url, nil)
		for _, ck := range cookies {
			req.AddCookie(ck)
		}
		resp, _ := app.fiber.Test(req)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("depth=%q status: got %d, want 200", raw, resp.StatusCode)
		}
		body := make([]byte, 0, 1024)
		buf := make([]byte, 512)
		for {
			n, err := resp.Body.Read(buf)
			body = append(body, buf[:n]...)
			if err != nil {
				break
			}
		}
		_ = resp.Body.Close()
		var data ldap_cache.GraphData
		if err := json.Unmarshal(body, &data); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if data.Depth < 1 || data.Depth > 3 {
			t.Errorf("depth=%q returned Depth=%d, expected [1,3]", raw, data.Depth)
		}
	}
}
```

- [ ] **Step 2: Fill the seed calls + run**

The `app.ldapCache.Users.setAll(...)` placeholders need the real seed from slice 1's test helpers. Import and use `newUserWithDN` from `internal/ldap_cache` (requires the helpers to be exported or co-located; if unexported, copy the helpers into `graph_v2_handler_test.go` or export them).

Decision: promote `seedCacheWithDNs` and `newUserWithDN/newGroupWithDN/newComputerWithDN` to exported test-only helpers by moving them to `internal/ldap_cache/test_helpers.go` and prefixing with `Test` (so they compile only in `*_test.go`).

- [ ] **Step 3: Run**

Run: `go test ./internal/web/ -run TestHandleGraphJSON -count=1`
Expected: PASS on 5 tests.

- [ ] **Step 4: Commit**

```bash
git add internal/web/graph_v2_handler_test.go internal/ldap_cache/test_helpers.go
git commit -S --signoff -m "test(web): /api/graph.json — validation, ETag, depth clamp"
```

### Task 13 — Integration test against OpenLDAP

**Files:**
- Create: `internal/web/graph_integration_test.go`

- [ ] **Step 1: Write the test using `skipIfNoLDAP`**

```go
package web

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
)

func TestGraphJSON_IntegrationUserFocus(t *testing.T) {
	env := skipIfNoLDAP(t)
	app, _ := setupIntegrationTestApp(t, env)
	seedLDAPData(t, env)
	app.ldapCache.Refresh()

	cookies := createAuthSession(t, app.sessionStore)

	// Use one of the seeded users from ldap_integration_test.go.
	entity := "uid=bob,ou=users,dc=test,dc=local"
	req := httptest.NewRequest("GET", "/api/graph.json?entity="+entity, nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	resp, err := app.fiber.Test(req, 5000)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Fatalf("status: %d", resp.StatusCode)
	}

	var data ldap_cache.GraphData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if data.Focus != entity {
		t.Errorf("focus: %q", data.Focus)
	}
	if len(data.Nodes) < 1 {
		t.Error("expected at least one node")
	}
}
```

`setupIntegrationTestApp` is a new helper that builds an `App` with the real LDAP config — mirror `setupFullTestApp` with the integration env.

- [ ] **Step 2: Run (skipped when no OpenLDAP)**

Run: `go test ./internal/web/ -run TestGraphJSON_Integration -count=1 -v`
Expected: PASS or SKIP.

- [ ] **Step 3: Commit**

```bash
git add internal/web/graph_integration_test.go
git commit -S --signoff -m "test(web): integration — /api/graph.json against real OpenLDAP"
```

### Task 14 — Slice 2 wrap

- [ ] **Step 1: Lint + test**

Run: `golangci-lint run ./internal/web/...` and `go test ./internal/web/ -run 'TestHandleGraph|TestGraphJSON_Integration' -count=1`
Expected: 0 issues, all tests green.

---

## Slice 3 — HTML Template + SSR Edge Table

This slice makes the graph work with JavaScript **disabled**. The SSR template renders positioned SVG nodes + an edge table. Slice 4 adds the interactive canvas on top.

### Task 15 — Template view-model

**Files:**
- Create: `internal/web/templates/graph_v2.templ`

- [ ] **Step 1: Define the view-model and skeleton template**

```go
// internal/web/templates/graph_v2.templ
package templates

import (
	"fmt"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
)

// GraphPageVM is the view-model for the /graph page. Slice 5 reuses it
// for the list-page Graph mode with Focus=="".
type GraphPageVM struct {
	Data        *ldap_cache.GraphData
	FocusLabel  string // human-friendly title, e.g. "bob.ops"
	FocusType   string // "user", "group", "computer", "ou", or "" for list mode
	ViewerDN    string
	SortRing    string // "asc" | "desc"
	SortFrom    string
	SortEdge    string
	SortTo      string
	SortType    string
	BackHref    string // URL to return to the referring list/detail page
}

templ GraphPageV2(vm GraphPageVM) {
	@baseV2(graphTitle(vm)) {
		@topnavV2("/graph")
		<main id="main-content" class="graph-page">
			<header class="graph-page__head">
				<h1 class="graph-page__title">{ graphTitle(vm) }</h1>
				@graphDepthSlider(vm.Data.Depth, vm.Data.Focus)
			</header>
			if vm.Data.Overflow.Truncated {
				<p class="graph-page__overflow" role="status">
					{ fmt.Sprintf("Showing %d of %d related entities.", vm.Data.Overflow.Rendered, vm.Data.Overflow.Available) }
				</p>
			}
			<div id="graph-announce" role="status" aria-live="polite" class="sr-only"></div>
			<section class="graph-canvas-section" aria-labelledby="graph-title-id">
				<h2 id="graph-title-id" class="sr-only">Relationship canvas</h2>
				@graphSVG(vm.Data, vm.FocusLabel)
			</section>
			<section class="graph-table-section" aria-labelledby="graph-table-title">
				<h2 id="graph-table-title">Relationships</h2>
				@graphEdgeTable(vm)
			</section>
		</main>
		<script id="graph-data" type="application/json">
			@graphInlineJSON(vm.Data)
		</script>
	}
}

func graphTitle(vm GraphPageVM) string {
	if vm.Data.Focus == "" {
		return "Graph view"
	}
	return "Relationships: " + vm.FocusLabel
}
```

The `graphDepthSlider`, `graphSVG`, `graphEdgeTable`, `graphInlineJSON` sub-components are defined in Tasks 16–19.

- [ ] **Step 2: Run `templ generate`**

Run: `templ generate`
Expected: `graph_v2_templ.go` created; 0 errors.

- [ ] **Step 3: Commit**

```bash
git add internal/web/templates/graph_v2.templ internal/web/templates/graph_v2_templ.go
git commit -S --signoff -m "feat(templ): graph page skeleton + view-model"
```

(Recall `*_templ.go` is .gitignored — the generated file is regenerated by CI.)

### Task 16 — Depth slider + overflow component

**Files:**
- Modify: `internal/web/templates/graph_v2.templ`

- [ ] **Step 1: Append the sub-components**

```go
templ graphDepthSlider(current int, focusDN string) {
	<form method="get" action="/graph" class="graph-slider">
		<input type="hidden" name="entity" value={ focusDN }/>
		<label for="depth-slider" class="graph-slider__label">Depth</label>
		<input
			id="depth-slider"
			type="range"
			name="depth"
			min="1" max="3" step="1"
			value={ fmt.Sprintf("%d", current) }
			aria-label="Graph walk depth, 1 to 3"
			data-graph-slider
		/>
		<output class="graph-slider__value" aria-live="polite" for="depth-slider">{ fmt.Sprintf("%d", current) }</output>
		<noscript><button type="submit" class="graph-slider__go">Apply</button></noscript>
	</form>
}
```

Slice 4's JS will listen to the slider's `input` event and navigate on change (avoiding the Apply button). No-JS users see the button.

- [ ] **Step 2: Regenerate + commit**

Run: `templ generate`

```bash
git add internal/web/templates/graph_v2.templ
git commit -S --signoff -m "feat(templ): graph depth slider with no-JS fallback button"
```

### Task 17 — SSR SVG nodes + edges

**Files:**
- Modify: `internal/web/templates/graph_v2.templ`

- [ ] **Step 1: Append `graphSVG`, `graphNode`, `graphEdgeLine`**

```go
templ graphSVG(data *ldap_cache.GraphData, focusLabel string) {
	<svg
		id="graph-canvas"
		class="graph-canvas"
		viewBox="-500 -500 1000 1000"
		role="img"
		aria-labelledby="graph-canvas-title"
		aria-describedby="graph-canvas-desc"
		tabindex="0"
	>
		<title id="graph-canvas-title">{ "Relationship graph for " + focusLabel }</title>
		<desc id="graph-canvas-desc">
			{ fmt.Sprintf("Shows %d relationships across %d entities.", len(data.Edges), len(data.Nodes)) }
		</desc>
		<g class="graph-viewport" transform="translate(0,0) scale(1)">
			for _, e := range data.Edges {
				@graphEdgeLine(e, data.Nodes)
			}
			for _, n := range data.Nodes {
				@graphNode(n)
			}
		</g>
	</svg>
}

templ graphEdgeLine(e ldap_cache.Edge, nodes []ldap_cache.Node) {
	{{ sx, sy := nodeXY(e.Source, nodes) }}
	{{ tx, ty := nodeXY(e.Target, nodes) }}
	<line
		class={ "graph-edge", "graph-edge--" + string(e.Kind) }
		x1={ fmt.Sprintf("%.2f", sx) }
		y1={ fmt.Sprintf("%.2f", sy) }
		x2={ fmt.Sprintf("%.2f", tx) }
		y2={ fmt.Sprintf("%.2f", ty) }
		data-source={ e.Source }
		data-target={ e.Target }
	></line>
}

templ graphNode(n ldap_cache.Node) {
	{{ x, y := concentricXY(n.Ring, n.Angle) }}
	<g
		class={ "graph-node", "graph-node--" + string(n.Type) }
		transform={ fmt.Sprintf("translate(%.2f,%.2f)", x, y) }
		role="button"
		tabindex="0"
		data-dn={ n.DN }
		data-type={ string(n.Type) }
		data-expandable={ fmt.Sprintf("%t", n.Expandable) }
		aria-label={ graphNodeLabel(n) }
	>
		<circle r="28" class="graph-node__disc"></circle>
		<text class="graph-node__label" text-anchor="middle" y="4">{ n.Label }</text>
		if n.Expandable {
			<g class="graph-node__expand-badge" transform="translate(18,-18)" aria-hidden="true">
				<circle r="8" class="graph-node__expand-badge-bg"></circle>
				<text class="graph-node__expand-badge-mark" text-anchor="middle" y="3">+</text>
			</g>
		}
	</g>
}

// concentricXY converts (ring, angle) to canvas coords. Ring 0 → origin;
// Ring r → radius r × 180 (viewBox is 1000×1000 centred at origin).
func concentricXY(ring int, angle float64) (float64, float64) {
	if ring == 0 {
		return 0, 0
	}
	r := float64(ring) * 180
	return r * math.Cos(angle), r * math.Sin(angle)
}

// nodeXY looks up a node's coordinates by DN for edge endpoint rendering.
func nodeXY(dn string, nodes []ldap_cache.Node) (float64, float64) {
	for _, n := range nodes {
		if n.DN == dn {
			return concentricXY(n.Ring, n.Angle)
		}
	}
	return 0, 0
}

func graphNodeLabel(n ldap_cache.Node) string {
	switch n.Type {
	case ldap_cache.NodeGroup:
		count := 0
		if n.MemberCount != nil {
			count = *n.MemberCount
		}
		return fmt.Sprintf("Group %s (%d members). Press Enter to open.", n.Label, count)
	case ldap_cache.NodeUser:
		state := "enabled"
		if n.Enabled != nil && !*n.Enabled {
			state = "disabled"
		}
		return fmt.Sprintf("User %s (%s). Press Enter to open.", n.Label, state)
	case ldap_cache.NodeComputer:
		return fmt.Sprintf("Computer %s. Press Enter to open.", n.Label)
	case ldap_cache.NodeOU:
		return fmt.Sprintf("Organisational unit %s. Press Enter to open.", n.Label)
	}
	return n.Label
}
```

Add `"math"` to imports.

- [ ] **Step 2: Regenerate + commit**

Run: `templ generate`

```bash
git add internal/web/templates/graph_v2.templ
git commit -S --signoff -m "feat(templ): SSR SVG rendering for graph nodes + edges"
```

### Task 18 — SSR edge table

**Files:**
- Modify: `internal/web/templates/graph_v2.templ`

- [ ] **Step 1: Append `graphEdgeTable` with sort-header anchors**

```go
templ graphEdgeTable(vm GraphPageVM) {
	<table class="graph-table" aria-describedby="graph-table-title">
		<thead>
			<tr>
				<th scope="col">@graphSortHeader("ring", "Ring", vm)</th>
				<th scope="col">@graphSortHeader("from", "From", vm)</th>
				<th scope="col">@graphSortHeader("edge", "Edge", vm)</th>
				<th scope="col">@graphSortHeader("to", "To", vm)</th>
				<th scope="col">@graphSortHeader("type", "Type", vm)</th>
			</tr>
		</thead>
		<tbody>
			for _, e := range sortEdges(vm.Data, vm) {
				@graphEdgeRow(e, vm.Data.Nodes)
			}
		</tbody>
	</table>
}

templ graphSortHeader(key, label string, vm GraphPageVM) {
	<a
		href={ templ.URL(buildSortHref(vm, key)) }
		class={ "graph-table__sort", graphSortClass(key, vm) }
	>
		{ label }
	</a>
}

templ graphEdgeRow(e ldap_cache.Edge, nodes []ldap_cache.Node) {
	{{ source, sourceType := nodeDesc(e.Source, nodes) }}
	{{ target, targetType := nodeDesc(e.Target, nodes) }}
	{{ ring := ringForEdge(e, nodes) }}
	<tr tabindex="0" data-dn={ e.Target } data-type={ targetType }>
		<td>{ fmt.Sprintf("%d", ring) }</td>
		<td><a href={ templ.URL(entityHref(e.Source, sourceType)) }>{ source }</a></td>
		<td>{ edgeLabel(e.Kind) }</td>
		<td><a href={ templ.URL(entityHref(e.Target, targetType)) }>{ target }</a></td>
		<td>{ string(targetType) }</td>
	</tr>
}

func edgeLabel(k ldap_cache.EdgeKind) string {
	switch k {
	case ldap_cache.EdgeMemberOf:
		return "member of"
	case ldap_cache.EdgeContains:
		return "contains"
	}
	return string(k)
}

// sortEdges, ringForEdge, nodeDesc, entityHref, buildSortHref,
// graphSortClass are straightforward helpers — implement alongside.
```

- [ ] **Step 2: Implement the helper functions**

Append to `graph_v2.templ`'s Go section:

```go
func nodeDesc(dn string, nodes []ldap_cache.Node) (label string, t ldap_cache.NodeType) {
	for _, n := range nodes {
		if n.DN == dn {
			return n.Label, n.Type
		}
	}
	return dn, ""
}

func ringForEdge(e ldap_cache.Edge, nodes []ldap_cache.Node) int {
	// The edge's ring is max(source.Ring, target.Ring). Centre node is
	// ring 0; the edge connects 0 to ring 1 → ring=1. Etc.
	sr, tr := 0, 0
	for _, n := range nodes {
		if n.DN == e.Source {
			sr = n.Ring
		}
		if n.DN == e.Target {
			tr = n.Ring
		}
	}
	if sr > tr {
		return sr
	}
	return tr
}

func entityHref(dn string, t ldap_cache.NodeType) string {
	switch t {
	case ldap_cache.NodeUser:
		return "/users/" + url.PathEscape(dn)
	case ldap_cache.NodeGroup:
		return "/groups/" + url.PathEscape(dn)
	case ldap_cache.NodeComputer:
		return "/computers/" + url.PathEscape(dn)
	case ldap_cache.NodeOU:
		return "/users?ou=" + url.QueryEscape(dn)
	}
	return "#"
}

func sortEdges(data *ldap_cache.GraphData, vm GraphPageVM) []ldap_cache.Edge {
	out := make([]ldap_cache.Edge, len(data.Edges))
	copy(out, data.Edges)
	sort.SliceStable(out, func(i, j int) bool {
		return edgeLess(out[i], out[j], data.Nodes, vm)
	})
	return out
}

func edgeLess(a, b ldap_cache.Edge, nodes []ldap_cache.Node, vm GraphPageVM) bool {
	// Default: ring asc, source label asc, target label asc.
	ra, rb := ringForEdge(a, nodes), ringForEdge(b, nodes)
	if ra != rb {
		return ra < rb
	}
	sla, _ := nodeDesc(a.Source, nodes)
	slb, _ := nodeDesc(b.Source, nodes)
	if sla != slb {
		return sla < slb
	}
	tla, _ := nodeDesc(a.Target, nodes)
	tlb, _ := nodeDesc(b.Target, nodes)
	return tla < tlb
}

func buildSortHref(vm GraphPageVM, key string) string {
	// For Task 18 we ship ascending-only; Task 19 adds toggle.
	return fmt.Sprintf("/graph?entity=%s&depth=%d&sort=%s",
		url.QueryEscape(vm.Data.Focus), vm.Data.Depth, key)
}

func graphSortClass(key string, vm GraphPageVM) string {
	// Populate when Task 19 tracks current sort state.
	return ""
}
```

Add `"net/url"`, `"sort"` to imports.

- [ ] **Step 3: Regenerate + commit**

Run: `templ generate`

```bash
git add internal/web/templates/graph_v2.templ
git commit -S --signoff -m "feat(templ): SSR edge table with entity links"
```

### Task 19 — Embedded JSON script block + graphInlineJSON helper

**Files:**
- Modify: `internal/web/templates/graph_v2.templ`

- [ ] **Step 1: Add the helper**

```go
// graphInlineJSON returns the marshalled GraphData. The template embeds
// it inside <script type="application/json"> so the client-side script
// (v2-graph.js) can read it without a second fetch.
func graphInlineJSON(data *ldap_cache.GraphData) string {
	b, err := json.Marshal(data)
	if err != nil {
		// Defensive: return a valid empty GraphData instead of broken HTML.
		return `{"focus":"","depth":0,"nodes":[],"edges":[],"overflow":{"truncated":false,"rendered":0,"available":0}}`
	}
	return string(b)
}
```

Add `"encoding/json"` to imports.

> **CSP note:** `<script type="application/json">` is a data block, not executable. The site's `script-src 'self'` CSP does not block it. `v2-graph.js` fetches it via `document.getElementById('graph-data').textContent` and `JSON.parse`.

- [ ] **Step 2: Regenerate + commit**

```bash
templ generate
git add internal/web/templates/graph_v2.templ
git commit -S --signoff -m "feat(templ): embed graph JSON inline for client consumption"
```

### Task 20 — HTML handler (`handleGraphV2`)

**Files:**
- Modify: `internal/web/graph_v2_handler.go`

- [ ] **Step 1: Add the HTML handler**

Append:

```go
// handleGraphV2 serves /graph?entity=<dn>&depth=<N> as HTML. Shares the
// build path with handleGraphJSON; wraps the result in the Templ page.
func (a *App) handleGraphV2(c *fiber.Ctx) error {
	data, err := a.buildGraphFromQuery(c)
	if err != nil {
		return err
	}

	vm := templates.GraphPageVM{
		Data:       data,
		FocusLabel: graphFocusLabel(data, a.ldapCache),
		FocusType:  graphFocusType(data, a.ldapCache),
		ViewerDN:   viewerDN(c),
	}

	return a.templateCache.RenderWithCache(c, templates.GraphPageV2(vm))
}

func graphFocusLabel(data *ldap_cache.GraphData, cache *ldap_cache.Manager) string {
	if data.Focus == "" {
		return ""
	}
	for _, n := range data.Nodes {
		if n.Ring == 0 {
			return n.Label
		}
	}
	return data.Focus
}

func graphFocusType(data *ldap_cache.GraphData, cache *ldap_cache.Manager) string {
	for _, n := range data.Nodes {
		if n.Ring == 0 {
			return string(n.Type)
		}
	}
	return ""
}

// viewerDN — helper already present in package. Reuse.
```

- [ ] **Step 2: Register the route**

Modify `internal/web/server.go` to add alongside the JSON route:

```go
protected.Get("/graph", a.handleGraphV2)
```

- [ ] **Step 3: Add a render test**

In `internal/web/graph_v2_handler_test.go`:

```go
func TestHandleGraphV2_RendersHTML(t *testing.T) {
	app, _ := setupFullTestApp(t)
	// seed bob into cache
	app.ldapCache.Users.setAll([]ldap.User{
		TestNewUserWithDN("cn=bob,dc=ex,dc=com", "bob", "bob", true, nil),
	})
	cookies := createAuthSession(t, app.sessionStore)

	req := httptest.NewRequest("GET", "/graph?entity=cn=bob,dc=ex,dc=com", nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	resp, err := app.fiber.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	for _, marker := range []string{
		`id="graph-canvas"`,
		`id="graph-data"`,
		`class="graph-table"`,
		`Relationships: bob`,
	} {
		if !strings.Contains(html, marker) {
			t.Errorf("missing HTML marker %q", marker)
		}
	}
}
```

- [ ] **Step 4: Run + commit**

Run: `go test ./internal/web/ -run TestHandleGraphV2 -count=1`

```bash
git add internal/web/graph_v2_handler.go internal/web/server.go internal/web/graph_v2_handler_test.go
git commit -S --signoff -m "feat(web): /graph HTML handler + render test"
```

### Task 21 — Base CSS for the graph page

**Files:**
- Modify: `internal/web/static/app.css`
- Modify: `internal/web/contrast_test.go`

- [ ] **Step 1: Append the graph section to `app.css`**

Append (at the end of the file, before the dark-theme section):

```css
/* ============================================================
   Graph view — spec 2026-04-24 §5, §6
   ============================================================ */

:root {
	--graph-edge: #a3a3a3;            /* AA 4.7 on #fff */
	--graph-edge-focus: #0a0a0a;
	--graph-node-border: #525252;     /* AAA 7.8 on #fff */
	--graph-node-focus-ring: #0a0a0a;
	--graph-node-bg-user: #fafafa;
	--graph-node-bg-group: #e5e5e5;
	--graph-node-bg-computer: #d4d4d4;
	--graph-node-bg-ou: #fff;
}

:root[data-theme="dark"] {
	--graph-edge: #525252;
	--graph-edge-focus: #f5f5f5;
	--graph-node-border: #a5a5a5;
	--graph-node-focus-ring: #4ade80;
	--graph-node-bg-user: #1a1a1a;
	--graph-node-bg-group: #262626;
	--graph-node-bg-computer: #333;
	--graph-node-bg-ou: #0d0d0d;
}

.graph-page { padding: 1rem; display: flex; flex-direction: column; gap: 1.5rem; }
.graph-page__head { display: flex; justify-content: space-between; align-items: baseline; gap: 1rem; }
.graph-page__title { margin: 0; }
.graph-page__overflow { color: var(--fg-muted); font-size: 0.9rem; }

.graph-slider { display: inline-flex; gap: 0.5rem; align-items: center; }
.graph-slider__label { font-weight: 600; }
.graph-slider__value { font-variant-numeric: tabular-nums; min-width: 1.5em; text-align: center; }

.graph-canvas { width: 100%; height: min(70vh, 640px); background: var(--bg-subtle); border: 1px solid var(--border); border-radius: 8px; }
.graph-canvas:focus-visible { outline: 2px solid var(--border-strong); outline-offset: 2px; }

.graph-edge { stroke: var(--graph-edge); stroke-width: 1.5; fill: none; }
.graph-edge:hover,
.graph-node:hover ~ .graph-edge[data-source] { stroke: var(--graph-edge-focus); stroke-width: 2; }

.graph-node { cursor: pointer; }
.graph-node__disc { fill: var(--graph-node-bg-user); stroke: var(--graph-node-border); stroke-width: 1.5; }
.graph-node--group .graph-node__disc { fill: var(--graph-node-bg-group); stroke-dasharray: 4 3; }
.graph-node--computer .graph-node__disc { fill: var(--graph-node-bg-computer); }
.graph-node--ou .graph-node__disc { fill: var(--graph-node-bg-ou); rx: 12; ry: 12; }
.graph-node__label { fill: var(--fg); font-size: 12px; font-weight: 500; pointer-events: none; }
.graph-node:focus-visible .graph-node__disc { outline: 2px solid var(--graph-node-focus-ring); outline-offset: 2px; }
.graph-node__expand-badge-bg { fill: var(--accent); }
.graph-node__expand-badge-mark { fill: var(--bg); font-size: 12px; font-weight: 700; }

.graph-table { width: 100%; border-collapse: collapse; font-size: 0.9rem; }
.graph-table th, .graph-table td { text-align: left; padding: 0.4rem 0.8rem; border-bottom: 1px solid var(--border); }
.graph-table th { background: var(--bg-subtle); text-transform: uppercase; font-size: 0.8rem; letter-spacing: 0.03em; }
.graph-table tr:focus-within { outline: 2px solid var(--border-strong); outline-offset: -2px; }
.graph-table__sort { color: var(--fg); text-decoration: none; }
.graph-table__sort:hover { text-decoration: underline; }

@media (prefers-reduced-motion: reduce) {
	.graph-node, .graph-edge { transition: none !important; }
}
```

- [ ] **Step 2: Add the new tokens to the contrast test**

Modify `internal/web/contrast_test.go` — look for the existing `expectedPairs` slice and append:

```go
{name: "graph-node-border on bg", tokenA: "--graph-node-border", tokenB: "--bg", minRatio: 7.0},
{name: "graph-edge on bg (non-text)", tokenA: "--graph-edge", tokenB: "--bg", minRatio: 3.0},
{name: "graph-edge-focus on bg", tokenA: "--graph-edge-focus", tokenB: "--bg", minRatio: 7.0},
```

(Exact shape depends on the current test — keep the pattern consistent.)

- [ ] **Step 3: Run + commit**

Run: `go test ./internal/web/ -run TestAppCSSTokensMeetAAAContrast -count=1 -v`
Expected: PASS.

```bash
git add internal/web/static/app.css internal/web/contrast_test.go
git commit -S --signoff -m "feat(css): graph view styles + AAA-verified tokens"
```

### Task 22 — Slice 3 wrap

- [ ] **Step 1: Full build + lint + test**

Run: `go build ./... && golangci-lint run ./... && go test ./internal/web/ ./internal/web/templates/ -run 'TestHandleGraph|TestBuildGraph|TestAppCSS' -count=1`
Expected: clean.

---

## Slice 4 — SVG Canvas + Interaction (JavaScript)

### Task 23 — `v2-graph.js` skeleton

**Files:**
- Create: `internal/web/static/js/v2-graph.js`

- [ ] **Step 1: Write the skeleton**

```js
// v2-graph.js — Phase 3 graph view client. Reads JSON embedded by the
// template, renders/enhances the SVG, wires pan/zoom/keyboard nav and
// click-to-expand. No dependencies; all CSP-safe (no eval, no inline
// scripts, no dynamic Function).

(function () {
	'use strict';
	if (!document.getElementById('graph-data')) return;

	var reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

	function parseData() {
		try {
			return JSON.parse(document.getElementById('graph-data').textContent);
		} catch (e) {
			console.error('graph-data JSON parse failed', e);
			return null;
		}
	}

	document.addEventListener('DOMContentLoaded', function () {
		var state = parseData();
		if (!state) return;
		var svg = document.getElementById('graph-canvas');
		if (!svg) return;

		var viewport = svg.querySelector('.graph-viewport');
		if (!viewport) return;

		wirePanZoom(svg, viewport);
		wireKeyboardNav(svg);
		wireNodeClicks(svg, state);
		wireDepthSlider();
	});

	function wirePanZoom(svg, viewport) {
		// see Task 24
	}
	function wireKeyboardNav(svg) {
		// see Task 25
	}
	function wireNodeClicks(svg, state) {
		// see Task 26
	}
	function wireDepthSlider() {
		// see Task 27
	}
})();
```

- [ ] **Step 2: Reference from base_v2.templ**

Add a conditional script tag — prefer: always-defer include (reads a single ID and exits if missing, so it's cheap on pages without the graph):

```go
// In internal/web/templates/base_v2.templ, inside <head>:
<script defer src="/static/js/v2-graph.js"></script>
```

Run `templ generate`.

- [ ] **Step 3: Commit**

```bash
git add internal/web/static/js/v2-graph.js internal/web/templates/base_v2.templ
git commit -S --signoff -m "feat(js): v2-graph.js skeleton (parses embedded JSON, locates canvas)"
```

### Task 24 — Pan + zoom

**Files:**
- Modify: `internal/web/static/js/v2-graph.js`

- [ ] **Step 1: Implement `wirePanZoom`**

Replace the stub:

```js
function wirePanZoom(svg, viewport) {
	var tx = 0, ty = 0, scale = 1;
	var dragging = false, sx = 0, sy = 0;

	function apply() {
		viewport.setAttribute('transform', 'translate(' + tx + ',' + ty + ') scale(' + scale + ')');
	}

	svg.addEventListener('mousedown', function (e) {
		if (e.target !== svg && !e.target.classList.contains('graph-viewport')) return;
		dragging = true;
		sx = e.clientX - tx;
		sy = e.clientY - ty;
		e.preventDefault();
	});
	window.addEventListener('mousemove', function (e) {
		if (!dragging) return;
		tx = e.clientX - sx;
		ty = e.clientY - sy;
		apply();
	});
	window.addEventListener('mouseup', function () { dragging = false; });

	svg.addEventListener('wheel', function (e) {
		if (!(e.ctrlKey || e.metaKey)) return;
		e.preventDefault();
		var delta = -e.deltaY * 0.001;
		scale = Math.min(3, Math.max(0.3, scale * (1 + delta)));
		apply();
	}, { passive: false });

	// Arrow-key pan when canvas is focused.
	svg.addEventListener('keydown', function (e) {
		var step = 32;
		switch (e.key) {
			case 'ArrowLeft': tx += step; apply(); e.preventDefault(); break;
			case 'ArrowRight': tx -= step; apply(); e.preventDefault(); break;
			case 'ArrowUp': ty += step; apply(); e.preventDefault(); break;
			case 'ArrowDown': ty -= step; apply(); e.preventDefault(); break;
			case '+': case '=': scale = Math.min(3, scale * 1.1); apply(); e.preventDefault(); break;
			case '-': case '_': scale = Math.max(0.3, scale / 1.1); apply(); e.preventDefault(); break;
		}
	});
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/web/static/js/v2-graph.js
git commit -S --signoff -m "feat(js): pan + zoom (mouse, wheel+ctrl, arrow keys)"
```

### Task 25 — Keyboard navigation between nodes

**Files:**
- Modify: `internal/web/static/js/v2-graph.js`

- [ ] **Step 1: Implement `wireKeyboardNav`**

```js
function wireKeyboardNav(svg) {
	var nodes = Array.prototype.slice.call(svg.querySelectorAll('.graph-node'));
	// Sort by ring, then angle (they're already in that order in DOM because
	// the template writes them per-ring; but be defensive).
	var index = 0;
	function focusAt(i) {
		index = ((i % nodes.length) + nodes.length) % nodes.length;
		nodes[index].focus();
	}
	svg.addEventListener('keydown', function (e) {
		if (e.target.classList.contains('graph-node')) {
			if (e.key === 'Tab' && !e.shiftKey) { focusAt(index + 1); e.preventDefault(); }
			else if (e.key === 'Tab' && e.shiftKey) { focusAt(index - 1); e.preventDefault(); }
		}
	});
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/web/static/js/v2-graph.js
git commit -S --signoff -m "feat(js): keyboard nav cycles graph nodes in ring order"
```

### Task 26 — Click-to-pivot and click-to-expand

**Files:**
- Modify: `internal/web/static/js/v2-graph.js`

- [ ] **Step 1: Implement `wireNodeClicks`**

```js
function wireNodeClicks(svg, state) {
	svg.addEventListener('click', function (e) {
		var node = e.target.closest('.graph-node');
		if (!node) return;
		var dn = node.getAttribute('data-dn');
		var type = node.getAttribute('data-type');
		var expandable = node.getAttribute('data-expandable') === 'true';
		var clickedBadge = !!e.target.closest('.graph-node__expand-badge');

		if (expandable && clickedBadge) {
			expandNode(dn, state, svg);
		} else {
			pivotToDrawer(dn, type);
		}
	});
	svg.addEventListener('keydown', function (e) {
		if (e.key !== 'Enter' && e.key !== ' ') return;
		var node = e.target.closest('.graph-node');
		if (!node) return;
		var dn = node.getAttribute('data-dn');
		var type = node.getAttribute('data-type');
		var expandable = node.getAttribute('data-expandable') === 'true';
		e.preventDefault();
		if (expandable) expandNode(dn, state, svg);
		else pivotToDrawer(dn, type);
	});
}

function pivotToDrawer(dn, type) {
	var base = { user: '/users/', group: '/groups/', computer: '/computers/', ou: '/users?ou=' }[type];
	if (!base) return;
	var href = type === 'ou' ? base + encodeURIComponent(dn) : base + encodeURIComponent(dn);
	window.location.href = href;
}

function announce(msg) {
	var el = document.getElementById('graph-announce');
	if (el) { el.textContent = ''; setTimeout(function () { el.textContent = msg; }, 10); }
}

function expandNode(dn, state, svg) {
	var url = '/api/graph.json?entity=' + encodeURIComponent(dn) + '&depth=1';
	fetch(url, { credentials: 'same-origin' })
		.then(function (r) { return r.json(); })
		.then(function (data) {
			var added = 0;
			var existingDNs = {};
			state.nodes.forEach(function (n) { existingDNs[n.dn] = true; });
			data.nodes.forEach(function (n) {
				if (existingDNs[n.dn]) return;
				n.ring = (state.nodes.find(function (x) { return x.dn === dn; }) || {}).ring + 1 || 2;
				state.nodes.push(n);
				renderNode(svg, n);
				added++;
			});
			data.edges.forEach(function (e) {
				if (!state.edges.some(function (x) { return x.source === e.source && x.target === e.target && x.kind === e.kind; })) {
					state.edges.push(e);
					renderEdge(svg, e, state.nodes);
				}
			});
			// Mark clicked node as non-expandable
			var el = svg.querySelector('.graph-node[data-dn="' + CSS.escape(dn) + '"]');
			if (el) {
				el.setAttribute('data-expandable', 'false');
				var badge = el.querySelector('.graph-node__expand-badge');
				if (badge) badge.remove();
			}
			announce('Expanded ' + dn + ': added ' + added + ' nodes.');
		});
}

function renderNode(svg, n) {
	var ns = 'http://www.w3.org/2000/svg';
	var viewport = svg.querySelector('.graph-viewport');
	var r = n.ring * 180;
	var x = r * Math.cos(n.angle), y = r * Math.sin(n.angle);
	var g = document.createElementNS(ns, 'g');
	g.setAttribute('class', 'graph-node graph-node--' + n.type + ' graph-node--added');
	g.setAttribute('transform', 'translate(' + x + ',' + y + ')');
	g.setAttribute('tabindex', '0');
	g.setAttribute('role', 'button');
	g.setAttribute('data-dn', n.dn);
	g.setAttribute('data-type', n.type);
	g.setAttribute('data-expandable', String(!!n.expandable));
	var circ = document.createElementNS(ns, 'circle');
	circ.setAttribute('r', '28');
	circ.setAttribute('class', 'graph-node__disc');
	g.appendChild(circ);
	var text = document.createElementNS(ns, 'text');
	text.setAttribute('text-anchor', 'middle');
	text.setAttribute('y', '4');
	text.setAttribute('class', 'graph-node__label');
	text.textContent = n.label;
	g.appendChild(text);
	viewport.appendChild(g);
}

function renderEdge(svg, e, nodes) {
	var ns = 'http://www.w3.org/2000/svg';
	var viewport = svg.querySelector('.graph-viewport');
	function xy(dn) {
		var n = nodes.find(function (x) { return x.dn === dn; });
		if (!n) return [0, 0];
		var r = n.ring * 180;
		return [r * Math.cos(n.angle), r * Math.sin(n.angle)];
	}
	var s = xy(e.source), t = xy(e.target);
	var line = document.createElementNS(ns, 'line');
	line.setAttribute('class', 'graph-edge graph-edge--' + e.kind);
	line.setAttribute('x1', s[0]); line.setAttribute('y1', s[1]);
	line.setAttribute('x2', t[0]); line.setAttribute('y2', t[1]);
	line.setAttribute('data-source', e.source);
	line.setAttribute('data-target', e.target);
	viewport.insertBefore(line, viewport.firstChild); // render behind nodes
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/web/static/js/v2-graph.js
git commit -S --signoff -m "feat(js): click-to-pivot + click-to-expand with aria-live announce"
```

### Task 27 — Depth slider JS + responsive resize

**Files:**
- Modify: `internal/web/static/js/v2-graph.js`

- [ ] **Step 1: Implement `wireDepthSlider`**

```js
function wireDepthSlider() {
	var slider = document.querySelector('[data-graph-slider]');
	if (!slider) return;
	var out = document.querySelector('.graph-slider__value');
	slider.addEventListener('input', function () { if (out) out.textContent = slider.value; });
	slider.addEventListener('change', function () {
		var form = slider.form;
		if (form) form.submit();
	});
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/web/static/js/v2-graph.js
git commit -S --signoff -m "feat(js): depth slider auto-submits on change (no button)"
```

### Task 28 — Slice 4 E2E test

**Files:**
- Create: `internal/e2e/graph_test.go` (or similar — follow existing e2e test naming)

- [ ] **Step 1: Write the happy-path Playwright test**

Use the existing e2e patterns in `internal/e2e/` (hook into Playwright via `chromedp` or whatever the project uses — follow what Phase 1 slices did). The test:

```go
//go:build e2e

package e2e

import "testing"

func TestGraphHappyPath(t *testing.T) {
	// 1. Sign in as admin, navigate to /users.
	// 2. Click first user's row.
	// 3. In drawer, click "View relationships".
	// 4. Assert URL is /graph?entity=...&depth=2.
	// 5. Assert <svg id="graph-canvas"> is present.
	// 6. Assert <table class="graph-table"> is present.
	// 7. Click an expandable group node.
	// 8. Assert aria-live announce fires with "Expanded".
	// 9. Assert new <tr> appears in the table.
	// 10. Assert reduced-motion snapshot: repeat with
	//     page.emulate_media({'reducedMotion':'reduce'}) — no transition.
}
```

- [ ] **Step 2: Run + iterate**

Run: `go test -tags e2e ./internal/e2e/ -run TestGraphHappyPath -v`

- [ ] **Step 3: Commit**

```bash
git add internal/e2e/graph_test.go
git commit -S --signoff -m "test(e2e): graph view — happy path, expand, reduced-motion"
```

### Task 29 — Slice 4 wrap

Run: `go test -tags e2e ./internal/e2e/ ./internal/web/ -count=1`
Expected: all green.

---

## Slice 5 — List-Page Graph Mode

### Task 30 — Extend `BuildGraph` for list-mode input

**Files:**
- Modify: `internal/ldap_cache/graph.go`
- Modify: `internal/ldap_cache/graph_test.go`

- [ ] **Step 1: Add `BuildListGraph`**

```go
// BuildListGraph builds a Graph for list-page mode: the filtered set
// plus each member's direct groups. Focus is "" and no node has ring 0.
// Users/computers live in ring 2; groups in ring 1.
func (m *Manager) BuildListGraph(filtered []ldap.User, filteredComputers []ldap.Computer) *GraphData {
	data := &GraphData{Focus: "", Depth: 1}
	seen := map[string]int{}

	for _, u := range filtered {
		if _, dup := seen[u.DN()]; !dup {
			data.Nodes = append(data.Nodes, userNode(u, 2, false))
			seen[u.DN()] = 2
		}
		for _, gDN := range u.Groups {
			if g, ok := m.Groups.FindByDN(gDN); ok {
				if _, dup := seen[gDN]; !dup {
					data.Nodes = append(data.Nodes, groupNode(*g, 1, false))
					seen[gDN] = 1
				}
				data.Edges = append(data.Edges, Edge{Source: u.DN(), Target: gDN, Kind: EdgeMemberOf})
			}
		}
	}
	for _, c := range filteredComputers {
		if _, dup := seen[c.DN()]; !dup {
			data.Nodes = append(data.Nodes, computerNode(c, 2))
			seen[c.DN()] = 2
		}
		for _, gDN := range c.Groups {
			if g, ok := m.Groups.FindByDN(gDN); ok {
				if _, dup := seen[gDN]; !dup {
					data.Nodes = append(data.Nodes, groupNode(*g, 1, false))
					seen[gDN] = 1
				}
				data.Edges = append(data.Edges, Edge{Source: c.DN(), Target: gDN, Kind: EdgeMemberOf})
			}
		}
	}

	applyCaps(data)
	assignConcentric(data)
	return data
}
```

- [ ] **Step 2: Write a test**

```go
func TestBuildListGraph_FilteredUsers(t *testing.T) {
	m := graphFixture(t)
	filtered := m.Users.Filter(func(u ldap.User) bool {
		return strings.Contains(u.DN(), "ou=Engineering")
	})
	data := m.BuildListGraph(filtered, nil)

	// Expect: bob, dave, alice (users) + admins, engineers (groups) = 5 nodes
	if got := len(data.Nodes); got != 5 {
		t.Errorf("node count: got %d, want 5", got)
	}
	if data.Focus != "" {
		t.Errorf("Focus should be empty for list mode, got %q", data.Focus)
	}
}
```

- [ ] **Step 3: Run + commit**

```bash
go test ./internal/ldap_cache/ -run TestBuildListGraph -count=1 -v
git add internal/ldap_cache/graph.go internal/ldap_cache/graph_test.go
git commit -S --signoff -m "feat(cache): BuildListGraph for list-page Graph mode"
```

### Task 31 — `view=graph` branch in users handler

**Files:**
- Modify: `internal/web/users_v2_handler.go`

- [ ] **Step 1: Add the branch**

Find `handleUsersV2` (list handler). Before the normal list render, insert:

```go
if c.Query("view") == "graph" {
	users := a.filterUsers(c) // whatever helper applies the current filters
	data := a.ldapCache.BuildListGraph(users, nil)
	vm := templates.GraphPageVM{Data: data, FocusLabel: "", FocusType: ""}
	return a.templateCache.RenderWithCache(c, templates.GraphPageV2(vm))
}
```

Use the existing filter helper from the list handler — if it is inline, extract it into `filterUsers`.

- [ ] **Step 2: Add an integration test**

```go
func TestUsersListGraphMode(t *testing.T) {
	env := skipIfNoLDAP(t)
	app, _ := setupIntegrationTestApp(t, env)
	seedLDAPData(t, env)
	app.ldapCache.Refresh()

	cookies := createAuthSession(t, app.sessionStore)
	req := httptest.NewRequest("GET", "/users?view=graph", nil)
	for _, ck := range cookies { req.AddCookie(ck) }
	resp, _ := app.fiber.Test(req, 5000)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `id="graph-canvas"`) {
		t.Error("missing graph canvas in list mode response")
	}
}
```

- [ ] **Step 3: Run + commit**

```bash
go test ./internal/web/ -run TestUsersListGraphMode -count=1 -v
git add internal/web/users_v2_handler.go internal/web/graph_integration_test.go
git commit -S --signoff -m "feat(web): /users?view=graph list-page mode"
```

### Task 32 — Mirror for groups + computers handlers

**Files:**
- Modify: `internal/web/groups_v2_handler.go`
- Modify: `internal/web/computers_v2_handler.go`

- [ ] **Step 1: Groups `view=graph`**

Filtered groups as edges between group and its members' groups is cumbersome. For list-mode on `/groups`, treat the filtered groups as ring 1 and their direct members as ring 2:

```go
if c.Query("view") == "graph" {
	groups := a.filterGroups(c)
	members := make([]ldap.User, 0)
	computers := make([]ldap.Computer, 0)
	for _, g := range groups {
		for _, mDN := range g.Members {
			if u, ok := a.ldapCache.Users.FindByDN(mDN); ok { members = append(members, *u) }
			if c2, ok := a.ldapCache.Computers.FindByDN(mDN); ok { computers = append(computers, *c2) }
		}
	}
	data := a.ldapCache.BuildListGraph(members, computers)
	vm := templates.GraphPageVM{Data: data}
	return a.templateCache.RenderWithCache(c, templates.GraphPageV2(vm))
}
```

- [ ] **Step 2: Computers `view=graph`**

```go
if c.Query("view") == "graph" {
	computers := a.filterComputers(c)
	data := a.ldapCache.BuildListGraph(nil, computers)
	vm := templates.GraphPageVM{Data: data}
	return a.templateCache.RenderWithCache(c, templates.GraphPageV2(vm))
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/web/groups_v2_handler.go internal/web/computers_v2_handler.go
git commit -S --signoff -m "feat(web): /groups + /computers view=graph list-page modes"
```

### Task 33 — Segmented List | Graph control in list templates

**Files:**
- Modify: `internal/web/templates/users_v2.templ`
- Modify: `internal/web/templates/groups_v2.templ`
- Modify: `internal/web/templates/computers_v2.templ`

- [ ] **Step 1: Add the control to each list template**

Insert at the top of the list-page main content:

```go
templ listGraphToggle(base string, currentView string, filters string) {
	<div class="graph-segmented" role="group" aria-label="List or graph view">
		<a
			class={ "graph-segmented__option", toggleClass(currentView, "") }
			href={ templ.URL(base + filters) }
			aria-pressed={ toggleAriaPressed(currentView, "") }
		>List</a>
		<a
			class={ "graph-segmented__option", toggleClass(currentView, "graph") }
			href={ templ.URL(base + "?view=graph" + prefixWithAmp(filters)) }
			aria-pressed={ toggleAriaPressed(currentView, "graph") }
		>Graph</a>
	</div>
}

func toggleClass(current, target string) string {
	if current == target { return "graph-segmented__option--active" }
	return ""
}
func toggleAriaPressed(current, target string) string {
	if current == target { return "true" }
	return "false"
}
func prefixWithAmp(q string) string {
	if q == "" || strings.HasPrefix(q, "?") { return q }
	return "&" + strings.TrimPrefix(q, "?")
}
```

Inject `@listGraphToggle("/users", c.Query("view"), queryStringWithoutView(c))` (or the groups/computers base) in each list template.

- [ ] **Step 2: Add CSS for the segmented control**

Append to `app.css`:

```css
.graph-segmented { display: inline-flex; border: 1px solid var(--border); border-radius: 999px; overflow: hidden; }
.graph-segmented__option { padding: 0.4rem 1rem; color: var(--fg-muted); text-decoration: none; }
.graph-segmented__option:hover { background: var(--bg-subtle); }
.graph-segmented__option--active { background: var(--accent); color: var(--bg); }
.graph-segmented__option:focus-visible { outline: 2px solid var(--border-strong); outline-offset: 2px; }
```

- [ ] **Step 3: Regenerate + commit**

```bash
templ generate
git add internal/web/templates/users_v2.templ internal/web/templates/groups_v2.templ internal/web/templates/computers_v2.templ internal/web/static/app.css
git commit -S --signoff -m "feat(ui): List | Graph segmented control on list pages"
```

### Task 34 — Slice 5 wrap

Run: `go test ./internal/web/ ./internal/web/templates/ ./internal/ldap_cache/ -count=1`

---

## Slice 6 — Drawer Pivots + AAA Ratcheting + Docs

### Task 35 — "View relationships" pivot in user drawer

**Files:**
- Modify: `internal/web/templates/user_drawer_fragment.templ` (or `users_v2.templ` where the pivot section is defined)

- [ ] **Step 1: Locate the pivot section**

Run: `grep -n 'Pivot\|drawer__pivot' internal/web/templates/users_v2.templ`
Find the `<ul class="drawer__pivots">` and the surrounding `<section>`.

- [ ] **Step 2: Add the anchor**

Inside the `<ul class="drawer__pivots">` add a new `<li>` as the second item (after "Open full page"):

```go
<li>
	<a class="drawer__pivot" href={ templ.URL("/graph?entity=" + url.QueryEscape(vm.User.DN())) }
	   aria-label="View relationship graph"
	   title="View relationship graph">
		<span class="drawer__pivot-icon" aria-hidden="true">@iconGraph()</span>
		<span class="drawer__pivot-text">View relationships</span>
	</a>
</li>
```

- [ ] **Step 3: Add `iconGraph` to `icons.templ`**

```go
templ iconGraph() {
	<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">
		<circle cx="6" cy="6" r="2.5"></circle>
		<circle cx="18" cy="6" r="2.5"></circle>
		<circle cx="6" cy="18" r="2.5"></circle>
		<circle cx="18" cy="18" r="2.5"></circle>
		<line x1="8.5" y1="6" x2="15.5" y2="6"></line>
		<line x1="6" y1="8.5" x2="6" y2="15.5"></line>
		<line x1="18" y1="8.5" x2="18" y2="15.5"></line>
		<line x1="8.5" y1="18" x2="15.5" y2="18"></line>
	</svg>
}
```

- [ ] **Step 4: Regenerate + commit**

```bash
templ generate
git add internal/web/templates/users_v2.templ internal/web/templates/icons.templ
git commit -S --signoff -m "feat(ui): View relationships pivot + iconGraph"
```

### Task 36 — Same pivot for groups and computers

**Files:**
- Modify: `internal/web/templates/groups_v2.templ`
- Modify: `internal/web/templates/computers_v2.templ`

- [ ] **Step 1: Mirror the anchor addition**

Same `<li>` + `@iconGraph()` pattern in each pivot list, linked to `/graph?entity=<group-or-computer-DN>`.

- [ ] **Step 2: Commit**

```bash
templ generate
git add internal/web/templates/groups_v2.templ internal/web/templates/computers_v2.templ
git commit -S --signoff -m "feat(ui): View relationships pivot on group + computer drawers"
```

### Task 37 — axe-core E2E ratchet

**Files:**
- Modify: `internal/e2e/axe_test.go` (or whatever holds the axe-core pass)

- [ ] **Step 1: Add `/graph?entity=<seeded-dn>` and `/users?view=graph` to the page list**

```go
var axeGraphPages = []string{
	"/graph?entity=" + url.QueryEscape("cn=bob,ou=Engineering,dc=test,dc=local") + "&depth=2",
	"/users?view=graph",
	"/groups?view=graph",
	"/computers?view=graph",
}
```

Run them through the existing axe-core harness with the AAA ruleset.

- [ ] **Step 2: Run + iterate**

Run: `go test -tags e2e ./internal/e2e/ -run TestAxeAAA -v`
Expected: 0 violations. Fix any CSS/aria issues until clean.

- [ ] **Step 3: Commit**

```bash
git add internal/e2e/axe_test.go
git commit -S --signoff -m "test(e2e): axe-core AAA ratchet includes graph pages"
```

### Task 38 — Tab-order snapshot update

**Files:**
- Modify: `internal/e2e/taborder_test.go` (or equivalent)

- [ ] **Step 1: Add the graph pages**

Same pattern as Task 37 — add each URL to the tab-order snapshot suite, run to capture the baseline, commit the baseline.

- [ ] **Step 2: Commit**

```bash
git add internal/e2e/taborder_test.go docs/accessibility/tab-order-graph.json  # or wherever snapshots live
git commit -S --signoff -m "test(e2e): tab-order snapshot includes graph pages"
```

### Task 39 — README conformance statement

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Find the accessibility section**

Run: `grep -n 'WCAG\|conformance\|accessibility' README.md`

- [ ] **Step 2: Append a sentence about the graph**

After the existing conformance statement (from the Phase 1 spec §7.4), add:

```markdown
The relationship graph view at `/graph` (and the `List | Graph` mode toggle on list pages) meets WCAG 2.2 Level AA. An equivalent flat edge table is always rendered below the visual canvas, providing an AAA-equivalent text alternative for every interaction and relationship the canvas displays.
```

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -S --signoff -m "docs(readme): graph view accessibility conformance statement"
```

### Task 40 — Slice 6 wrap + final full run

- [ ] **Step 1: Full test + lint**

```bash
go test ./... -count=1
golangci-lint run ./...
go test -tags e2e ./internal/e2e/ -count=1
```

Expected: all green.

- [ ] **Step 2: Push branch + open PR**

```bash
git push -u origin feat/phase-3-graph-view
gh pr create --base main --title "feat(ui): phase 3 relationship graph view" --body "$(cat <<'EOF'
## Summary

Ships the Phase 3 relationship graph view per docs/superpowers/specs/2026-04-24-phase-3-graph-view-design.md.

- Dedicated /graph?entity=<dn>&depth=<N> with click-to-pivot + click-to-expand.
- List | Graph mode toggle on /users, /groups, /computers with bounded depth-1 scope.
- Hand-rolled SVG, no new dependencies.
- AAA-compliant flat edge table always rendered below the canvas.

## Test plan

- [x] `go test ./... -count=1` passes.
- [x] `golangci-lint run ./...` clean.
- [x] `go test -tags e2e ./internal/e2e/ -count=1` (axe-core AAA + tab-order) clean.
- [ ] Manual smoke on the dev LDAP: /graph from each drawer, click-to-expand, keyboard-only nav, reduced-motion off + on.
EOF
)"
```

---

## Self-Review

Cross-checking the plan against the spec:

- **§1 Goals / scope posture** — captured in plan header.
- **§2 Scope + two entry points** — Slice 2 (JSON) + Slice 3 (dedicated /graph HTML) + Slice 5 (list-page mode).
- **§3 Architecture / package layout** — plan File Structure table matches.
- **§4 JSON shape + endpoint contract + caps** — Slice 1 Task 1, 2, 6; Slice 2 Task 10, 12.
- **§4.4 List-page Graph mode** — Slice 5 Task 30, 31, 32.
- **§5 Client rendering (no library, hybrid math, click-to-expand state, resize)** — Slice 4 Tasks 23-27.
- **§6 AAA parallel view (always below, ARIA, sort)** — Slice 3 Task 18.
- **§7 Interaction model (R3)** — Slice 4 Task 26.
- **§9 Accessibility strategy (contrast, axe-core, target size, tab-order, aria-live, reduced-motion)** — Slice 3 Task 21 (contrast), Slice 4 Task 26 (aria-live), Slice 6 Task 37 (axe) + Task 38 (tab-order).
- **§10 Testing strategy** — distributed: unit (slice 1), handler (slice 2-3), integration (slice 2), E2E (slice 4-6).
- **§11 Rollout** — plan's 6 slices == spec's 6 slices.
- **§12 Rejected alternatives** — locked in by the spec; plan doesn't need to repeat.
- **§13 Open assumptions** — Pre-flight Tasks 0.1–0.5.

Placeholders found inline during self-review:
- The `addOUChildren` bug in Task 2 Step 3 is flagged with an implementer note and explicitly fixed in Task 5. Intentional.
- `setupIntegrationTestApp` (Task 13) is referenced but not defined — it is a new helper the implementer writes mirroring `setupFullTestApp`. Acceptable for a plan.
- Exact e2e framework (`chromedp` vs. Playwright) left "follow what Phase 1 slices did" in Task 28 — the previous phase's `internal/e2e/` files pick the framework; the plan doesn't re-litigate that choice.

Type consistency: `NodeType` enum values (`NodeUser`, `NodeGroup`, `NodeComputer`, `NodeOU`) match across all tasks. `EdgeKind` values (`EdgeMemberOf`, `EdgeContains`) match. `GraphData` field names match between Go struct and JSON tags throughout.

Scope: this is a single implementation plan — 40 tasks across 6 slices — matching the 6-slice spec rollout exactly. No decomposition needed.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-04-24-phase-3-graph-view.md`.** Two execution options:

1. **Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

2. **Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
