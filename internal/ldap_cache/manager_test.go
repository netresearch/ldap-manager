// Package name uses underscore for LDAP domain clarity (ldap_cache vs ldapcache).
// nolint:revive
package ldap_cache

import (
	"errors"
	"testing"
	"time"

	ldap "github.com/netresearch/simple-ldap-go"
)

// Test helpers for common assertion patterns
func assertEntityNotFound[T any](t *testing.T, entity *T, err, expectedError error) {
	t.Helper()
	if !errors.Is(err, expectedError) {
		t.Errorf("Expected %v, got %v", expectedError, err)
	}
	if entity != nil {
		t.Error("Expected nil entity when not found")
	}
}

func createMockUsers() []ldap.User {
	return []ldap.User{
		NewMockUser("cn=john.doe,ou=users,dc=example,dc=com", "john.doe", true,
			[]string{"cn=users,ou=groups,dc=example,dc=com"}),
		NewMockUser("cn=jane.smith,ou=users,dc=example,dc=com", "jane.smith", false,
			[]string{"cn=admins,ou=groups,dc=example,dc=com"}),
	}
}

func createMockGroups() []ldap.Group {
	return []ldap.Group{
		NewMockGroup("cn=users,ou=groups,dc=example,dc=com", "users",
			[]string{"cn=john.doe,ou=users,dc=example,dc=com"}),
		NewMockGroup("cn=admins,ou=groups,dc=example,dc=com", "admins",
			[]string{"cn=jane.smith,ou=users,dc=example,dc=com"}),
	}
}

func createMockComputers() []ldap.Computer {
	return []ldap.Computer{
		NewMockComputer("cn=workstation-01,ou=computers,dc=example,dc=com", "workstation-01$", true,
			[]string{"cn=computers,ou=groups,dc=example,dc=com"}),
	}
}

func TestNewManager(t *testing.T) {
	mockClient := &mockLDAPClient{}
	manager := New(mockClient)

	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}

	if manager.client != mockClient {
		t.Error("Expected client to be set correctly")
	}

	if manager.stop == nil {
		t.Error("Expected stop channel to be initialized")
	}

	if manager.Users.Count() != 0 {
		t.Errorf("Expected empty Users cache, got %d", manager.Users.Count())
	}

	if manager.Groups.Count() != 0 {
		t.Errorf("Expected empty Groups cache, got %d", manager.Groups.Count())
	}

	if manager.Computers.Count() != 0 {
		t.Errorf("Expected empty Computers cache, got %d", manager.Computers.Count())
	}
}

// refreshTestParams holds parameters for refresh test scenarios
type refreshTestParams struct {
	name          string
	setupData     func() *mockLDAPClient
	refreshFunc   func(*Manager) error
	getCount      func(*Manager) int
	getCallCount  func(*mockLDAPClient) int
	expectedCount int
}

// testRefreshScenarios tests successful and error scenarios for refresh operations
func testRefreshScenarios(t *testing.T, params refreshTestParams) {
	t.Run("successful refresh", func(t *testing.T) {
		mockClient := params.setupData()
		manager := New(mockClient)

		err := params.refreshFunc(manager)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if params.getCount(manager) != params.expectedCount {
			t.Errorf("Expected %d %s, got %d", params.expectedCount, params.name, params.getCount(manager))
		}

		if params.getCallCount(mockClient) != 1 {
			t.Errorf("Expected 1 call to Find%s, got %d", params.name, params.getCallCount(mockClient))
		}
	})

	t.Run("error during refresh", func(t *testing.T) {
		expectedError := errors.New("LDAP connection failed")
		mockClient := params.setupData()
		// Set the appropriate error based on the test type
		setRefreshError(mockClient, params.name, expectedError)
		manager := New(mockClient)

		err := params.refreshFunc(manager)
		if err == nil {
			t.Error("Expected error, got nil")
		}

		if !errors.Is(err, expectedError) {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}

		if params.getCount(manager) != 0 {
			t.Errorf("Expected 0 %s after error, got %d", params.name, params.getCount(manager))
		}
	})
}

// setRefreshError sets the appropriate error for the given refresh type
func setRefreshError(mockClient *mockLDAPClient, refreshType string, err error) {
	switch refreshType {
	case "users":
		mockClient.findUsersError = err
	case "groups":
		mockClient.findGroupsError = err
	case "computers":
		mockClient.findComputersError = err
	}
}

func TestManagerRefreshUsers(t *testing.T) {
	testRefreshScenarios(t, refreshTestParams{
		name: "users",
		setupData: func() *mockLDAPClient {
			return &mockLDAPClient{users: createMockUsers()}
		},
		refreshFunc:   (*Manager).RefreshUsers,
		getCount:      func(m *Manager) int { return m.Users.Count() },
		getCallCount:  func(m *mockLDAPClient) int { return m.callCounts.findUsers },
		expectedCount: 2,
	})
}

func TestManagerRefreshGroups(t *testing.T) {
	testRefreshScenarios(t, refreshTestParams{
		name: "groups",
		setupData: func() *mockLDAPClient {
			return &mockLDAPClient{groups: createMockGroups()}
		},
		refreshFunc:   (*Manager).RefreshGroups,
		getCount:      func(m *Manager) int { return m.Groups.Count() },
		getCallCount:  func(m *mockLDAPClient) int { return m.callCounts.findGroups },
		expectedCount: 2,
	})
}

func TestManagerRefreshComputers(t *testing.T) {
	testRefreshScenarios(t, refreshTestParams{
		name: "computers",
		setupData: func() *mockLDAPClient {
			return &mockLDAPClient{computers: createMockComputers()}
		},
		refreshFunc:   (*Manager).RefreshComputers,
		getCount:      func(m *Manager) int { return m.Computers.Count() },
		getCallCount:  func(m *mockLDAPClient) int { return m.callCounts.findComputers },
		expectedCount: 1,
	})
}

func TestManagerRefresh(t *testing.T) {
	mockClient := &mockLDAPClient{
		users:     createMockUsers(),
		groups:    createMockGroups(),
		computers: createMockComputers(),
	}
	manager := New(mockClient)

	manager.Refresh()

	if manager.Users.Count() != 2 {
		t.Errorf("Expected 2 users, got %d", manager.Users.Count())
	}

	if manager.Groups.Count() != 2 {
		t.Errorf("Expected 2 groups, got %d", manager.Groups.Count())
	}

	if manager.Computers.Count() != 1 {
		t.Errorf("Expected 1 computer, got %d", manager.Computers.Count())
	}

	// Verify all methods were called
	if mockClient.callCounts.findUsers != 1 {
		t.Errorf("Expected 1 call to FindUsers, got %d", mockClient.callCounts.findUsers)
	}

	if mockClient.callCounts.findGroups != 1 {
		t.Errorf("Expected 1 call to FindGroups, got %d", mockClient.callCounts.findGroups)
	}

	if mockClient.callCounts.findComputers != 1 {
		t.Errorf("Expected 1 call to FindComputers, got %d", mockClient.callCounts.findComputers)
	}
}

func TestManagerFindUsers(t *testing.T) {
	mockClient := &mockLDAPClient{
		users: createMockUsers(),
	}
	manager := New(mockClient)
	if err := manager.RefreshUsers(); err != nil {
		t.Fatalf("Failed to refresh users: %v", err)
	}

	t.Run("find users including disabled", func(t *testing.T) {
		users := manager.FindUsers(true)
		if len(users) != 2 {
			t.Errorf("Expected 2 users, got %d", len(users))
		}
	})

	t.Run("find users excluding disabled", func(t *testing.T) {
		users := manager.FindUsers(false)
		if len(users) != 1 {
			t.Errorf("Expected 1 enabled user, got %d", len(users))
		}

		if !users[0].Enabled {
			t.Error("Expected returned user to be enabled")
		}
	})
}

func TestManagerFindUserByDN(t *testing.T) {
	mockClient := &mockLDAPClient{
		users: createMockUsers(),
	}
	manager := New(mockClient)
	if err := manager.RefreshUsers(); err != nil {
		t.Fatalf("Failed to refresh users: %v", err)
	}

	// Since we can't mock the DN() method easily, just test the error case
	t.Run("find non-existent user returns error", func(t *testing.T) {
		user, err := manager.FindUserByDN("cn=nonexistent,ou=users,dc=example,dc=com")
		assertEntityNotFound(t, user, err, ldap.ErrUserNotFound)
	})
}

func TestManagerFindUserBySAMAccountName(t *testing.T) {
	mockClient := &mockLDAPClient{
		users: createMockUsers(),
	}
	manager := New(mockClient)
	if err := manager.RefreshUsers(); err != nil {
		t.Fatalf("Failed to refresh users: %v", err)
	}

	t.Run("find existing user by SAMAccountName", func(t *testing.T) {
		user, err := manager.FindUserBySAMAccountName("jane.smith")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if user.SAMAccountName != "jane.smith" {
			t.Errorf("Expected SAMAccountName 'jane.smith', got '%s'", user.SAMAccountName)
		}
	})

	t.Run("find non-existent user by SAMAccountName", func(t *testing.T) {
		user, err := manager.FindUserBySAMAccountName("nonexistent")
		assertEntityNotFound(t, user, err, ldap.ErrUserNotFound)
	})
}

func TestManagerFindGroups(t *testing.T) {
	mockClient := &mockLDAPClient{
		groups: createMockGroups(),
	}
	manager := New(mockClient)
	if err := manager.RefreshGroups(); err != nil {
		t.Fatalf("Failed to refresh groups: %v", err)
	}

	groups := manager.FindGroups()
	if len(groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(groups))
	}
}

func TestManagerFindGroupByDN(t *testing.T) {
	mockClient := &mockLDAPClient{
		groups: createMockGroups(),
	}
	manager := New(mockClient)
	if err := manager.RefreshGroups(); err != nil {
		t.Fatalf("Failed to refresh groups: %v", err)
	}

	// Test error case since we can't easily mock DN() method
	t.Run("find non-existent group returns error", func(t *testing.T) {
		group, err := manager.FindGroupByDN("cn=nonexistent,ou=groups,dc=example,dc=com")
		assertEntityNotFound(t, group, err, ldap.ErrGroupNotFound)
	})
}

func TestManagerFindComputers(t *testing.T) {
	mockClient := &mockLDAPClient{
		computers: createMockComputers(),
	}
	manager := New(mockClient)
	if err := manager.RefreshComputers(); err != nil {
		t.Fatalf("Failed to refresh computers: %v", err)
	}

	t.Run("find computers including disabled", func(t *testing.T) {
		computers := manager.FindComputers(true)
		if len(computers) != 1 {
			t.Errorf("Expected 1 computer, got %d", len(computers))
		}
	})

	t.Run("find computers excluding disabled", func(t *testing.T) {
		computers := manager.FindComputers(false)
		if len(computers) != 1 {
			t.Errorf("Expected 1 enabled computer, got %d", len(computers))
		}
	})
}

func TestManagerFindComputerByDN(t *testing.T) {
	mockClient := &mockLDAPClient{
		computers: createMockComputers(),
	}
	manager := New(mockClient)
	if err := manager.RefreshComputers(); err != nil {
		t.Fatalf("Failed to refresh computers: %v", err)
	}

	// Test error case since we can't easily mock DN() method
	t.Run("find non-existent computer returns error", func(t *testing.T) {
		computer, err := manager.FindComputerByDN("cn=nonexistent,ou=computers,dc=example,dc=com")
		assertEntityNotFound(t, computer, err, ldap.ErrComputerNotFound)
	})
}

func TestManagerPopulateGroupsForUser(t *testing.T) {
	mockClient := &mockLDAPClient{
		users:  createMockUsers(),
		groups: createMockGroups(),
	}
	manager := New(mockClient)
	if err := manager.RefreshUsers(); err != nil {
		t.Fatalf("Failed to refresh users: %v", err)
	}
	if err := manager.RefreshGroups(); err != nil {
		t.Fatalf("Failed to refresh groups: %v", err)
	}

	// Test with a user from our cache
	users := manager.FindUsers(true)
	if len(users) > 0 {
		fullUser := manager.PopulateGroupsForUser(&users[0])

		if fullUser.SAMAccountName != users[0].SAMAccountName {
			t.Errorf("Expected SAMAccountName '%s', got '%s'", users[0].SAMAccountName, fullUser.SAMAccountName)
		}

		// Groups will be empty since DN lookup doesn't work in our mock setup
		// But the method should not crash and should return a valid structure
		if fullUser.Groups == nil {
			t.Error("Expected Groups slice to be initialized")
		}
	}
}

func TestManagerPopulateUsersForGroup(t *testing.T) {
	mockClient := &mockLDAPClient{
		users:  createMockUsers(),
		groups: createMockGroups(),
	}
	manager := New(mockClient)
	if err := manager.RefreshUsers(); err != nil {
		t.Fatalf("Failed to refresh users: %v", err)
	}
	if err := manager.RefreshGroups(); err != nil {
		t.Fatalf("Failed to refresh groups: %v", err)
	}

	// Test with a group from our cache
	groups := manager.FindGroups()
	if len(groups) > 0 {
		fullGroup := manager.PopulateUsersForGroup(&groups[0], false)

		// Members will be empty since DN lookup doesn't work in our mock setup
		// But the method should not crash and should return a valid structure
		if fullGroup.Members == nil {
			t.Error("Expected Members slice to be initialized")
		}

		if len(groups[0].Members) != len(fullGroup.Members) {
			// Since DN lookup fails, populated members should be empty
			// while original group members should have the DNs
			if len(fullGroup.Members) != 0 {
				t.Errorf("Expected 0 populated members (DN lookup fails), got %d", len(fullGroup.Members))
			}
		}
	}
}

func TestManagerStop(t *testing.T) {
	mockClient := &mockLDAPClient{}
	manager := New(mockClient)

	// Start the manager in a goroutine so it can receive the stop signal
	go manager.Run()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Test that Stop sends signal
	done := make(chan bool)
	go func() {
		manager.Stop()
		done <- true
	}()

	select {
	case <-done:
		// Success - Stop() completed
	case <-time.After(100 * time.Millisecond):
		t.Error("Stop() blocked longer than expected")
	}

	// Give manager time to actually stop
	time.Sleep(10 * time.Millisecond)
}

func TestManagerPopulateGroupsForComputer(t *testing.T) {
	mockClient := &mockLDAPClient{
		users:     createMockUsers(),
		groups:    createMockGroups(),
		computers: createMockComputers(),
	}
	manager := New(mockClient)
	if err := manager.RefreshUsers(); err != nil {
		t.Fatalf("Failed to refresh users: %v", err)
	}
	if err := manager.RefreshGroups(); err != nil {
		t.Fatalf("Failed to refresh groups: %v", err)
	}
	if err := manager.RefreshComputers(); err != nil {
		t.Fatalf("Failed to refresh computers: %v", err)
	}

	// Test with a computer from our cache
	computers := manager.FindComputers(true)
	if len(computers) > 0 {
		fullComputer := manager.PopulateGroupsForComputer(&computers[0])

		if fullComputer.SAMAccountName != computers[0].SAMAccountName {
			t.Errorf("Expected SAMAccountName '%s', got '%s'", computers[0].SAMAccountName, fullComputer.SAMAccountName)
		}

		// Groups will be empty since DN lookup doesn't work in our mock setup
		// But the method should not crash and should return a valid structure
		if fullComputer.Groups == nil {
			t.Error("Expected Groups slice to be initialized")
		}
	}
}

// userGroupTestParams holds parameters for user-group operation tests
type userGroupTestParams struct {
	name          string
	groupDN       string
	operationFunc func(*Manager, string, string)
	validateCount func(initial, updated int) bool
	errorMessage  string
}

// setupUserGroupTest creates a manager with test data and finds the target user
func setupUserGroupTest(t *testing.T) (manager *Manager, initialGroupCount int) {
	mockClient := &mockLDAPClient{
		users:  createMockUsers(),
		groups: createMockGroups(),
	}
	manager = New(mockClient)
	if err := manager.RefreshUsers(); err != nil {
		t.Fatalf("Failed to refresh users: %v", err)
	}
	if err := manager.RefreshGroups(); err != nil {
		t.Fatalf("Failed to refresh groups: %v", err)
	}

	// Get user by SAMAccountName since DN() doesn't work with mock data
	users := manager.Users.Get()
	if len(users) == 0 {
		t.Fatal("No users available for testing")
	}

	// Find john.doe by SAMAccountName
	var targetUser *ldap.User
	for i := range users {
		if users[i].SAMAccountName == "john.doe" {
			targetUser = &users[i]

			break
		}
	}

	if targetUser == nil {
		t.Fatal("Could not find john.doe user for testing")
	}

	initialGroupCount = len(targetUser.Groups)

	return manager, initialGroupCount
}

// testUserGroupOperation tests user-group operations with common validation logic
func testUserGroupOperation(t *testing.T, params userGroupTestParams) {
	manager, initialGroupCount := setupUserGroupTest(t)

	userDN := "cn=john.doe,ou=users,dc=example,dc=com"
	initialUsers := manager.Users.Get()

	// Perform the operation
	params.operationFunc(manager, userDN, params.groupDN)

	// Check that the operation doesn't crash and basic invariants hold
	// Note: Since we can't verify group membership with mock data,
	// we mainly test that the function doesn't panic
	updatedUsers := manager.Users.Get()
	if len(updatedUsers) != len(initialUsers) {
		t.Error("User count changed unexpectedly")
	}

	// Verify group count meets expectations
	for i := range updatedUsers {
		if updatedUsers[i].SAMAccountName == "john.doe" {
			updatedGroupCount := len(updatedUsers[i].Groups)
			if !params.validateCount(initialGroupCount, updatedGroupCount) {
				t.Errorf("%s (initial: %d, updated: %d)", params.errorMessage, initialGroupCount, updatedGroupCount)
			}

			return
		}
	}

	t.Error("Could not find john.doe user after operation")
}

func TestManagerOnAddUserToGroup(t *testing.T) {
	testUserGroupOperation(t, userGroupTestParams{
		name:          "add",
		groupDN:       "cn=newgroup,ou=groups,dc=example,dc=com",
		operationFunc: (*Manager).OnAddUserToGroup,
		validateCount: func(initial, updated int) bool { return updated >= initial },
		errorMessage:  "User lost groups after OnAddUserToGroup",
	})
}

func TestManagerOnRemoveUserFromGroup(t *testing.T) {
	testUserGroupOperation(t, userGroupTestParams{
		name:          "remove",
		groupDN:       "cn=users,ou=groups,dc=example,dc=com", // This user is already in this group
		operationFunc: (*Manager).OnRemoveUserFromGroup,
		validateCount: func(initial, updated int) bool { return updated <= initial },
		errorMessage:  "User gained groups unexpectedly after OnRemoveUserFromGroup",
	})
}

func TestManagerIsWarmedUp(t *testing.T) {
	mockClient := &mockLDAPClient{
		users:     createMockUsers(),
		groups:    createMockGroups(),
		computers: createMockComputers(),
	}
	manager := New(mockClient)

	// Test that IsWarmedUp doesn't panic and returns a boolean
	_ = manager.IsWarmedUp()

	// After refreshing all caches, test again
	if err := manager.RefreshUsers(); err != nil {
		t.Fatalf("Failed to refresh users: %v", err)
	}
	if err := manager.RefreshGroups(); err != nil {
		t.Fatalf("Failed to refresh groups: %v", err)
	}
	if err := manager.RefreshComputers(); err != nil {
		t.Fatalf("Failed to refresh computers: %v", err)
	}

	// Test that method still works after refreshes
	_ = manager.IsWarmedUp()
}

func TestManagerFindComputerBySAMAccountName(t *testing.T) {
	mockClient := &mockLDAPClient{
		computers: createMockComputers(),
	}
	manager := New(mockClient)
	if err := manager.RefreshComputers(); err != nil {
		t.Fatalf("Failed to refresh computers: %v", err)
	}

	t.Run("find existing computer by SAMAccountName", func(t *testing.T) {
		computer, err := manager.FindComputerBySAMAccountName("workstation-01$")
		if err != nil {
			t.Errorf("Expected to find computer, got error: %v", err)
		}
		if computer == nil {
			t.Error("Expected to find computer, got nil")
		}
		if computer != nil && computer.SAMAccountName != "workstation-01$" {
			t.Errorf("Expected SAMAccountName 'workstation-01$', got '%s'", computer.SAMAccountName)
		}
	})

	t.Run("find non-existent computer by SAMAccountName", func(t *testing.T) {
		computer, err := manager.FindComputerBySAMAccountName("nonexistent$")
		assertEntityNotFound(t, computer, err, ldap.ErrComputerNotFound)
	})
}

func TestManagerGetMetrics(t *testing.T) {
	mockClient := &mockLDAPClient{
		users:  createMockUsers(),
		groups: createMockGroups(),
	}
	manager := New(mockClient)
	if err := manager.RefreshUsers(); err != nil {
		t.Fatalf("Failed to refresh users: %v", err)
	}
	if err := manager.RefreshGroups(); err != nil {
		t.Fatalf("Failed to refresh groups: %v", err)
	}

	metrics := manager.GetMetrics()

	// Test that GetMetrics doesn't panic and returns valid structure
	if metrics == nil {
		t.Fatal("Expected non-nil metrics")
	}

	// Verify metrics has required fields
	_ = metrics.UserCount
	_ = metrics.GroupCount
	_ = metrics.ComputerCount
	_ = metrics.RefreshCount
	_ = metrics.HealthStatus
}

func TestManagerGetHealthCheck(t *testing.T) {
	mockClient := &mockLDAPClient{
		users:  createMockUsers(),
		groups: createMockGroups(),
	}
	manager := New(mockClient)
	if err := manager.RefreshUsers(); err != nil {
		t.Fatalf("Failed to refresh users: %v", err)
	}
	if err := manager.RefreshGroups(); err != nil {
		t.Fatalf("Failed to refresh groups: %v", err)
	}

	health := manager.GetHealthCheck()

	// Test that GetHealthCheck doesn't panic and returns valid structure
	if health.HealthStatus == "" {
		t.Error("Expected non-empty health status")
	}

	// Verify structure has required fields
	_ = health.RefreshCount
	_ = health.ErrorRate
	_ = health.EntityCounts.Users
	_ = health.EntityCounts.Groups
	_ = health.EntityCounts.Computers
}

func TestManagerIsHealthy(t *testing.T) {
	mockClient := &mockLDAPClient{
		users:  createMockUsers(),
		groups: createMockGroups(),
	}
	manager := New(mockClient)

	// Manager starts as healthy (0 = healthy status)
	// It only becomes unhealthy after errors accumulate
	initialHealth := manager.IsHealthy()
	if !initialHealth {
		t.Log("Note: Manager is not healthy initially, which is acceptable")
	}

	// After successful refreshes, should definitely be healthy
	if err := manager.RefreshUsers(); err != nil {
		t.Fatalf("Failed to refresh users: %v", err)
	}
	if err := manager.RefreshGroups(); err != nil {
		t.Fatalf("Failed to refresh groups: %v", err)
	}

	if !manager.IsHealthy() {
		t.Error("Expected manager to be healthy after successful refreshes")
	}
}
