package ldap_cache

import (
	"fmt"
	"testing"

	ldap "github.com/netresearch/simple-ldap-go"
)

// BenchmarkFindBySAMAccountName_LinearSearch benchmarks the old O(n) linear search approach
func BenchmarkFindBySAMAccountName_LinearSearch(b *testing.B) {
	// Create test data using mock users
	users := make([]ldap.User, 1000)
	for i := 0; i < 1000; i++ {
		dn := fmt.Sprintf("CN=user%d,OU=Users,DC=example,DC=com", i)
		users[i] = NewMockUser(dn, fmt.Sprintf("user%d", i), true, []string{})
	}

	cache := NewCached[ldap.User]()
	cache.setAll(users)

	// Target SAMAccountName to search for (worst case - last item)
	targetSAM := "user999"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate old linear search using Find method
		_, found := cache.Find(func(u ldap.User) bool {
			return u.SAMAccountName == targetSAM
		})
		if !found {
			b.Fatal("User not found")
		}
	}
}

// BenchmarkFindBySAMAccountName_IndexedSearch benchmarks the new O(1) indexed search approach
func BenchmarkFindBySAMAccountName_IndexedSearch(b *testing.B) {
	// Create test data using mock users
	users := make([]ldap.User, 1000)
	for i := 0; i < 1000; i++ {
		dn := fmt.Sprintf("CN=user%d,OU=Users,DC=example,DC=com", i)
		users[i] = NewMockUser(dn, fmt.Sprintf("user%d", i), true, []string{})
	}

	cache := NewCached[ldap.User]()
	cache.setAll(users)

	// Target SAMAccountName to search for (same as linear search)
	targetSAM := "user999"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use new indexed search
		_, found := cache.FindBySAMAccountName(targetSAM)
		if !found {
			b.Fatal("User not found")
		}
	}
}

// BenchmarkFindBySAMAccountName_Scale1k tests 1k users
func BenchmarkFindBySAMAccountName_Scale1k(b *testing.B) {
	users := make([]ldap.User, 1000)
	for i := 0; i < 1000; i++ {
		dn := fmt.Sprintf("CN=user%d,OU=Users,DC=example,DC=com", i)
		users[i] = NewMockUser(dn, fmt.Sprintf("user%d", i), true, []string{})
	}

	cache := NewCached[ldap.User]()
	cache.setAll(users)

	// Search for user in the middle to show average case performance
	targetSAM := "user500"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, found := cache.FindBySAMAccountName(targetSAM)
		if !found {
			b.Fatal("User not found")
		}
	}
}

// BenchmarkFindBySAMAccountName_Scale10k tests 10k users (realistic enterprise scale)
func BenchmarkFindBySAMAccountName_Scale10k(b *testing.B) {
	users := make([]ldap.User, 10000)
	for i := 0; i < 10000; i++ {
		dn := fmt.Sprintf("CN=user%d,OU=Users,DC=example,DC=com", i)
		users[i] = NewMockUser(dn, fmt.Sprintf("user%d", i), true, []string{})
	}

	cache := NewCached[ldap.User]()
	cache.setAll(users)

	// Search for user in the middle to show average case performance
	targetSAM := "user5000"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, found := cache.FindBySAMAccountName(targetSAM)
		if !found {
			b.Fatal("User not found")
		}
	}
}

// BenchmarkFindBySAMAccountName_Scale50k tests 50k users (large enterprise scale)
func BenchmarkFindBySAMAccountName_Scale50k(b *testing.B) {
	users := make([]ldap.User, 50000)
	for i := 0; i < 50000; i++ {
		dn := fmt.Sprintf("CN=user%d,OU=Users,DC=example,DC=com", i)
		users[i] = NewMockUser(dn, fmt.Sprintf("user%d", i), true, []string{})
	}

	cache := NewCached[ldap.User]()
	cache.setAll(users)

	// Search for user in the middle to show average case performance
	targetSAM := "user25000"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, found := cache.FindBySAMAccountName(targetSAM)
		if !found {
			b.Fatal("User not found")
		}
	}
}

// BenchmarkCacheUpdate_IndexRebuild benchmarks the performance impact of index rebuilding
func BenchmarkCacheUpdate_IndexRebuild(b *testing.B) {
	// Create test data using mock users
	users := make([]ldap.User, 1000)
	for i := 0; i < 1000; i++ {
		dn := fmt.Sprintf("CN=user%d,OU=Users,DC=example,DC=com", i)
		users[i] = NewMockUser(dn, fmt.Sprintf("user%d", i), true, []string{})
	}

	cache := NewCached[ldap.User]()
	cache.setAll(users)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Update cache to trigger index rebuild
		cache.update(func(user *ldap.User) {
			// Simulate a minor update
			user.Enabled = !user.Enabled
		})
	}
}

// BenchmarkMemoryOverhead measures memory usage with indexing
func BenchmarkMemoryOverhead(b *testing.B) {
	// This benchmark helps understand memory overhead of indexing
	users := make([]ldap.User, 10000)
	for i := 0; i < 10000; i++ {
		dn := fmt.Sprintf("CN=user%d,OU=Users,DC=example,DC=com", i)
		users[i] = NewMockUser(dn, fmt.Sprintf("user%d", i), true, []string{})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache := NewCached[ldap.User]()
		cache.setAll(users)

		// Perform some lookups to ensure indexes are used
		_, _ = cache.FindBySAMAccountName("user5000")
	}
}

// BenchmarkComputers_FindBySAMAccountName tests computer lookups by SAMAccountName
func BenchmarkComputers_FindBySAMAccountName(b *testing.B) {
	computers := make([]ldap.Computer, 1000)
	for i := 0; i < 1000; i++ {
		dn := fmt.Sprintf("CN=computer%d,OU=Computers,DC=example,DC=com", i)
		computers[i] = NewMockComputer(dn, fmt.Sprintf("computer%d$", i), true, []string{})
	}

	cache := NewCached[ldap.Computer]()
	cache.setAll(computers)

	// Search for computer by SAMAccountName
	targetSAM := "computer500$"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, found := cache.FindBySAMAccountName(targetSAM)
		if !found {
			b.Fatal("Computer not found")
		}
	}
}
