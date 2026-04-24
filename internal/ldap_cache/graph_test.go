package ldap_cache

import (
	"fmt"
	"math"
	"sort"
	"testing"
	"time"

	ldap "github.com/netresearch/simple-ldap-go"
)

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
		newUserWithDN("cn=deep,ou=Nested,ou=Engineering,dc=ex,dc=com", "deep", "deep", true, []string{}),
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
	manager.Computers.setAll([]ldap.Computer{
		newComputerWithDN("cn=ws01,ou=Computers,dc=ex,dc=com", "ws01", "ws01$", true, []string{
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

	// Edge kind correctness — catches a regression where direction is
	// right but kind is swapped.
	bobDN := "cn=bob,ou=Engineering,dc=ex,dc=com"
	ouDN := "ou=Engineering,dc=ex,dc=com"
	for _, e := range data.Edges {
		switch {
		case e.Source == bobDN && e.Kind != EdgeMemberOf:
			t.Errorf("edge from bob expected EdgeMemberOf, got %q: %+v", e.Kind, e)
		case e.Source == ouDN && e.Kind != EdgeContains:
			t.Errorf("edge from OU expected EdgeContains, got %q: %+v", e.Kind, e)
		}
	}

	// No self-loops — a user/group/OU shouldn't appear as both source
	// and target of the same edge.
	for _, e := range data.Edges {
		if e.Source == e.Target {
			t.Errorf("self-loop edge: %+v", e)
		}
	}
}

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

	// Expect edges: bob→engineers, dave→engineers, alice→engineers
	// (memberOf) + engineers→all-staff (memberOf) = 4 edges.
	if got := len(data.Edges); got != 4 {
		t.Errorf("edge count: got %d, want 4", got)
	}

	// Edge kind + direction correctness on the parent edge — the
	// non-obvious branch of the group-focus builder.
	engineersDN := "cn=engineers,ou=Groups,dc=ex,dc=com"
	allStaffDN := "cn=all-staff,ou=Groups,dc=ex,dc=com"

	var foundParent bool

	for _, e := range data.Edges {
		if e.Source == engineersDN && e.Target == allStaffDN {
			if e.Kind != EdgeMemberOf {
				t.Errorf("engineers→all-staff edge: kind %q, want %q", e.Kind, EdgeMemberOf)
			}

			foundParent = true
		}

		if e.Source == e.Target {
			t.Errorf("self-loop edge: %+v", e)
		}
	}

	if !foundParent {
		t.Errorf("missing engineers→all-staff memberOf edge")
	}
}

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
	// Expect 2 edges: ws01→engineers (memberOf), ou=Computers→ws01 (contains).
	if got := len(data.Edges); got != 2 {
		t.Errorf("edge count: got %d, want 2", got)
	}

	ws01DN := "cn=ws01,ou=Computers,dc=ex,dc=com"
	ouDN := "ou=Computers,dc=ex,dc=com"

	for _, e := range data.Edges {
		switch {
		case e.Source == ws01DN && e.Kind != EdgeMemberOf:
			t.Errorf("edge from ws01 expected EdgeMemberOf, got %q: %+v", e.Kind, e)
		case e.Source == ouDN && e.Kind != EdgeContains:
			t.Errorf("edge from OU expected EdgeContains, got %q: %+v", e.Kind, e)
		}

		if e.Source == e.Target {
			t.Errorf("self-loop edge: %+v", e)
		}
	}
}

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

	ouDN := "ou=Engineering,dc=ex,dc=com"
	if got := len(data.Edges); got != 3 {
		t.Errorf("edge count: got %d, want 3", got)
	}

	for _, e := range data.Edges {
		if e.Source != ouDN {
			t.Errorf("edge source: got %q, want %q: %+v", e.Source, ouDN, e)
		}

		if e.Kind != EdgeContains {
			t.Errorf("edge kind: got %q, want %q: %+v", e.Kind, EdgeContains, e)
		}

		if e.Source == e.Target {
			t.Errorf("self-loop edge: %+v", e)
		}
	}

	// Nested descendants must not leak through — deep lives two levels
	// below ou=Engineering and should be filtered by the immediate-child
	// check.
	deepDN := "cn=deep,ou=Nested,ou=Engineering,dc=ex,dc=com"
	for _, n := range data.Nodes {
		if n.DN == deepDN {
			t.Errorf("nested-descendant %q must not appear as ring-1 child of %q", deepDN, ouDN)
		}
	}
}

func TestBuildGraph_OUFocus_ComputersBranch(t *testing.T) {
	m := graphFixture(t)
	data, err := m.BuildGraph("ou=Computers,dc=ex,dc=com", 1)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	// Expect: ou=Computers (ring 0) + ws01 (ring 1) = 2 nodes
	if got := len(data.Nodes); got != 2 {
		t.Errorf("node count: got %d, want 2", got)
	}

	if got := len(data.Edges); got != 1 {
		t.Errorf("edge count: got %d, want 1", got)
	}

	ouDN := "ou=Computers,dc=ex,dc=com"
	ws01DN := "cn=ws01,ou=Computers,dc=ex,dc=com"

	for _, e := range data.Edges {
		if e.Source != ouDN || e.Target != ws01DN || e.Kind != EdgeContains {
			t.Errorf("unexpected edge: %+v", e)
		}

		if e.Source == e.Target {
			t.Errorf("self-loop edge: %+v", e)
		}
	}
}

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

	// Rendered must equal the surviving node count exactly.
	if data.Overflow.Rendered != len(data.Nodes) {
		t.Errorf("Overflow.Rendered: got %d, want %d (len(data.Nodes))",
			data.Overflow.Rendered, len(data.Nodes))
	}

	// Every surviving edge must reference only surviving nodes.
	kept := make(map[string]bool, len(data.Nodes))
	for _, n := range data.Nodes {
		kept[n.DN] = true
	}

	for _, e := range data.Edges {
		if !kept[e.Source] {
			t.Errorf("edge source %q refers to dropped node: %+v", e.Source, e)
		}

		if !kept[e.Target] {
			t.Errorf("edge target %q refers to dropped node: %+v", e.Target, e)
		}
	}
}

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

func TestBuildGraph_CycleSafe(t *testing.T) {
	manager := New(&mockLDAPClient{})
	manager.Groups.setAll([]ldap.Group{
		newGroupWithDN("cn=A,ou=Groups,dc=ex,dc=com", "A", []string{"cn=B,ou=Groups,dc=ex,dc=com"}),
		newGroupWithDN("cn=B,ou=Groups,dc=ex,dc=com", "B", []string{"cn=A,ou=Groups,dc=ex,dc=com"}),
	})

	var data *GraphData

	done := make(chan struct{})
	go func() {
		data, _ = manager.BuildGraph("cn=A,ou=Groups,dc=ex,dc=com", 3)
		close(done)
	}()

	select {
	case <-done:
		if data == nil {
			t.Fatal("BuildGraph returned nil graph")
		}

		if len(data.Nodes) < 2 {
			t.Errorf("expected at least 2 nodes (A, B), got %d", len(data.Nodes))
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("BuildGraph cycled forever")
	}
}

func TestBuildGraph_OUFocus_EscapedCommaChild(t *testing.T) {
	manager := New(&mockLDAPClient{})
	// "Last, First" with the comma escaped per RFC 4514.
	childDN := `cn=Last\, First,ou=Engineering,dc=ex,dc=com`
	manager.Users.setAll([]ldap.User{
		newUserWithDN(childDN, "Last, First", "lastf", true, nil),
	})

	ouDN := "ou=Engineering,dc=ex,dc=com"
	data, err := manager.BuildGraph(ouDN, 1)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	foundNode := false

	for _, n := range data.Nodes {
		if n.DN == childDN {
			foundNode = true

			break
		}
	}

	if !foundNode {
		t.Fatalf("escaped-comma immediate child %q must appear in depth-1 nodes for %q", childDN, ouDN)
	}

	foundEdge := false

	for _, e := range data.Edges {
		if e.Source == ouDN && e.Target == childDN {
			if e.Kind != EdgeContains {
				t.Errorf("edge kind for escaped-comma child: got %q, want %q", e.Kind, EdgeContains)
			}

			foundEdge = true

			break
		}
	}

	if !foundEdge {
		t.Errorf("escaped-comma immediate child %q must have contains edge from %q", childDN, ouDN)
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
