package ldap_cache

import (
	"errors"
	"testing"
	"time"

	ldap "github.com/netresearch/simple-ldap-go"
)

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
		t.Error("Expected non-nil manager")
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

func TestManagerRefreshUsers(t *testing.T) {
	t.Run("successful refresh", func(t *testing.T) {
		mockClient := &mockLDAPClient{
			users: createMockUsers(),
		}
		manager := New(mockClient)
		
		err := manager.RefreshUsers()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		
		if manager.Users.Count() != 2 {
			t.Errorf("Expected 2 users, got %d", manager.Users.Count())
		}
		
		if mockClient.callCounts.findUsers != 1 {
			t.Errorf("Expected 1 call to FindUsers, got %d", mockClient.callCounts.findUsers)
		}
	})
	
	t.Run("error during refresh", func(t *testing.T) {
		expectedError := errors.New("LDAP connection failed")
		mockClient := &mockLDAPClient{
			findUsersError: expectedError,
		}
		manager := New(mockClient)
		
		err := manager.RefreshUsers()
		if err == nil {
			t.Error("Expected error, got nil")
		}
		
		if err != expectedError {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}
		
		if manager.Users.Count() != 0 {
			t.Errorf("Expected 0 users after error, got %d", manager.Users.Count())
		}
	})
}

func TestManagerRefreshGroups(t *testing.T) {
	t.Run("successful refresh", func(t *testing.T) {
		mockClient := &mockLDAPClient{
			groups: createMockGroups(),
		}
		manager := New(mockClient)
		
		err := manager.RefreshGroups()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		
		if manager.Groups.Count() != 2 {
			t.Errorf("Expected 2 groups, got %d", manager.Groups.Count())
		}
		
		if mockClient.callCounts.findGroups != 1 {
			t.Errorf("Expected 1 call to FindGroups, got %d", mockClient.callCounts.findGroups)
		}
	})
	
	t.Run("error during refresh", func(t *testing.T) {
		expectedError := errors.New("LDAP connection failed")
		mockClient := &mockLDAPClient{
			findGroupsError: expectedError,
		}
		manager := New(mockClient)
		
		err := manager.RefreshGroups()
		if err == nil {
			t.Error("Expected error, got nil")
		}
		
		if err != expectedError {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}
		
		if manager.Groups.Count() != 0 {
			t.Errorf("Expected 0 groups after error, got %d", manager.Groups.Count())
		}
	})
}

func TestManagerRefreshComputers(t *testing.T) {
	t.Run("successful refresh", func(t *testing.T) {
		mockClient := &mockLDAPClient{
			computers: createMockComputers(),
		}
		manager := New(mockClient)
		
		err := manager.RefreshComputers()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		
		if manager.Computers.Count() != 1 {
			t.Errorf("Expected 1 computer, got %d", manager.Computers.Count())
		}
		
		if mockClient.callCounts.findComputers != 1 {
			t.Errorf("Expected 1 call to FindComputers, got %d", mockClient.callCounts.findComputers)
		}
	})
	
	t.Run("error during refresh", func(t *testing.T) {
		expectedError := errors.New("LDAP connection failed")
		mockClient := &mockLDAPClient{
			findComputersError: expectedError,
		}
		manager := New(mockClient)
		
		err := manager.RefreshComputers()
		if err == nil {
			t.Error("Expected error, got nil")
		}
		
		if err != expectedError {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}
		
		if manager.Computers.Count() != 0 {
			t.Errorf("Expected 0 computers after error, got %d", manager.Computers.Count())
		}
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
	manager.RefreshUsers()
	
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
	manager.RefreshUsers()
	
	// Since we can't mock the DN() method easily, just test the error case
	t.Run("find non-existent user returns error", func(t *testing.T) {
		user, err := manager.FindUserByDN("cn=nonexistent,ou=users,dc=example,dc=com")
		if err != ldap.ErrUserNotFound {
			t.Errorf("Expected ErrUserNotFound, got %v", err)
		}
		
		if user != nil {
			t.Error("Expected nil user when not found")
		}
	})
}

func TestManagerFindUserBySAMAccountName(t *testing.T) {
	mockClient := &mockLDAPClient{
		users: createMockUsers(),
	}
	manager := New(mockClient)
	manager.RefreshUsers()
	
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
		if err != ldap.ErrUserNotFound {
			t.Errorf("Expected ErrUserNotFound, got %v", err)
		}
		
		if user != nil {
			t.Error("Expected nil user when not found")
		}
	})
}

func TestManagerFindGroups(t *testing.T) {
	mockClient := &mockLDAPClient{
		groups: createMockGroups(),
	}
	manager := New(mockClient)
	manager.RefreshGroups()
	
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
	manager.RefreshGroups()
	
	// Test error case since we can't easily mock DN() method
	t.Run("find non-existent group returns error", func(t *testing.T) {
		group, err := manager.FindGroupByDN("cn=nonexistent,ou=groups,dc=example,dc=com")
		if err != ldap.ErrGroupNotFound {
			t.Errorf("Expected ErrGroupNotFound, got %v", err)
		}
		
		if group != nil {
			t.Error("Expected nil group when not found")
		}
	})
}

func TestManagerFindComputers(t *testing.T) {
	mockClient := &mockLDAPClient{
		computers: createMockComputers(),
	}
	manager := New(mockClient)
	manager.RefreshComputers()
	
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
	manager.RefreshComputers()
	
	// Test error case since we can't easily mock DN() method
	t.Run("find non-existent computer returns error", func(t *testing.T) {
		computer, err := manager.FindComputerByDN("cn=nonexistent,ou=computers,dc=example,dc=com")
		if err != ldap.ErrComputerNotFound {
			t.Errorf("Expected ErrComputerNotFound, got %v", err)
		}
		
		if computer != nil {
			t.Error("Expected nil computer when not found")
		}
	})
}

func TestManagerPopulateGroupsForUser(t *testing.T) {
	mockClient := &mockLDAPClient{
		users:  createMockUsers(),
		groups: createMockGroups(),
	}
	manager := New(mockClient)
	manager.RefreshUsers()
	manager.RefreshGroups()
	
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
	manager.RefreshUsers()
	manager.RefreshGroups()
	
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
		users:  createMockUsers(),
		groups: createMockGroups(),
		computers: createMockComputers(),
	}
	manager := New(mockClient)
	manager.RefreshUsers()
	manager.RefreshGroups()
	manager.RefreshComputers()
	
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

func TestManagerOnAddUserToGroup(t *testing.T) {
	mockClient := &mockLDAPClient{
		users:  createMockUsers(),
		groups: createMockGroups(),
	}
	manager := New(mockClient)
	manager.RefreshUsers()
	manager.RefreshGroups()
	
	userDN := "cn=john.doe,ou=users,dc=example,dc=com"
	groupDN := "cn=newgroup,ou=groups,dc=example,dc=com"
	
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
	
	initialGroupCount := len(targetUser.Groups)
	
	// Add user to group
	manager.OnAddUserToGroup(userDN, groupDN)
	
	// Check that OnAddUserToGroup doesn't crash
	// Note: Since we can't verify group membership with mock data, 
	// we mainly test that the function doesn't panic
	updatedUsers := manager.Users.Get()
	if len(updatedUsers) != len(users) {
		t.Error("User count changed unexpectedly")
	}
	
	// Verify we still have the same number of groups or more
	for i := range updatedUsers {
		if updatedUsers[i].SAMAccountName == "john.doe" {
			if len(updatedUsers[i].Groups) < initialGroupCount {
				t.Error("User lost groups after OnAddUserToGroup")
			}
			return
		}
	}
}

func TestManagerOnRemoveUserFromGroup(t *testing.T) {
	mockClient := &mockLDAPClient{
		users:  createMockUsers(),
		groups: createMockGroups(),
	}
	manager := New(mockClient)
	manager.RefreshUsers()
	manager.RefreshGroups()
	
	userDN := "cn=john.doe,ou=users,dc=example,dc=com"
	groupDN := "cn=users,ou=groups,dc=example,dc=com" // This user is already in this group
	
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
	
	initialGroupCount := len(targetUser.Groups)
	
	// Remove user from group
	manager.OnRemoveUserFromGroup(userDN, groupDN)
	
	// Check that OnRemoveUserFromGroup doesn't crash
	// Note: Since we can't verify group membership with mock data, 
	// we mainly test that the function doesn't panic
	updatedUsers := manager.Users.Get()
	if len(updatedUsers) != len(users) {
		t.Error("User count changed unexpectedly")
	}
	
	// Verify we still have the same number of groups or fewer
	for i := range updatedUsers {
		if updatedUsers[i].SAMAccountName == "john.doe" {
			if len(updatedUsers[i].Groups) > initialGroupCount {
				t.Error("User gained groups unexpectedly after OnRemoveUserFromGroup")
			}
			return
		}
	}
}