package ldap_cache

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// fuzzCacheable is a test implementation for fuzz testing with SAMAccountName support
type fuzzCacheable struct {
	dn             string
	data           string
	SAMAccountName string
}

func (m fuzzCacheable) DN() string { return m.dn }

// FuzzCacheFindByDN tests cache operations with fuzzed DN values
func FuzzCacheFindByDN(f *testing.F) {
	// Seed with known edge cases
	f.Add("cn=admin,dc=example,dc=com")
	f.Add("cn=user with spaces,ou=users,dc=example,dc=com")
	f.Add("cn=user\\,with\\,commas,dc=example,dc=com")
	f.Add("cn=user+uid=123,dc=example,dc=com")
	f.Add("cn=\"quoted user\",dc=example,dc=com")
	f.Add("cn=,dc=example,dc=com") // Empty CN
	f.Add("")                       // Empty string
	f.Add("cn=user,")               // Trailing comma
	f.Add(",cn=user")               // Leading comma
	f.Add("cn=user;;dc=example")    // Invalid separator
	f.Add(strings.Repeat("a", 1000)) // Long string
	f.Add("cn=用户,dc=example,dc=com")           // Unicode

	f.Fuzz(func(t *testing.T, dn string) {
		if !utf8.ValidString(dn) {
			return // Skip invalid UTF-8
		}

		cache := NewCached[fuzzCacheable]()

		// Test FindByDN doesn't panic on empty cache
		result, found := cache.FindByDN(dn)
		if found {
			t.Error("Found item in empty cache")
		}
		if result != nil {
			t.Error("Result should be nil for empty cache")
		}

		// Add an item and test lookup
		item := fuzzCacheable{dn: dn, data: "test"}
		cache.setAll([]fuzzCacheable{item})

		result, found = cache.FindByDN(dn)
		if dn != "" {
			if !found {
				t.Errorf("Should find item with DN: %s", dn)
			}
			if result != nil && result.DN() != dn {
				t.Errorf("DN mismatch: got %s, want %s", result.DN(), dn)
			}
		}

		// Verify cache count
		if cache.Count() != 1 {
			t.Errorf("Expected count 1, got %d", cache.Count())
		}
	})
}

// FuzzCacheFindBySAMAccountName tests cache with fuzzed SAMAccountName values
func FuzzCacheFindBySAMAccountName(f *testing.F) {
	// Seed with known edge cases
	f.Add("admin")
	f.Add("user.name")
	f.Add("user_name")
	f.Add("user-name")
	f.Add("USER123")
	f.Add("")                       // Empty
	f.Add("a")                      // Single char
	f.Add(strings.Repeat("x", 256)) // Max length
	f.Add(strings.Repeat("y", 257)) // Over max length
	f.Add("user name")              // Space
	f.Add("user@domain")            // At sign
	f.Add("用户名")                    // Unicode

	f.Fuzz(func(t *testing.T, sam string) {
		if !utf8.ValidString(sam) {
			return // Skip invalid UTF-8
		}

		cache := NewCached[fuzzCacheable]()

		// Test FindBySAMAccountName doesn't panic on empty cache
		result, found := cache.FindBySAMAccountName(sam)
		if found {
			t.Error("Found item in empty cache")
		}
		if result != nil {
			t.Error("Result should be nil for empty cache")
		}

		// Add an item with SAMAccountName and test lookup
		item := fuzzCacheable{
			dn:             "cn=test,dc=example,dc=com",
			data:           "test",
			SAMAccountName: sam,
		}
		cache.setAll([]fuzzCacheable{item})

		_, found = cache.FindBySAMAccountName(sam)
		if sam != "" {
			if !found {
				t.Errorf("Should find item with SAMAccountName: %s", sam)
			}
		}
	})
}

// FuzzCacheFilter tests filter operations with fuzzed predicates
func FuzzCacheFilter(f *testing.F) {
	// Seed with sample data for filter matching
	f.Add("test", true)
	f.Add("admin", false)
	f.Add("", true)
	f.Add(strings.Repeat("a", 100), false)
	f.Add("user*", true)
	f.Add("user?", false)

	f.Fuzz(func(t *testing.T, pattern string, matchIfContains bool) {
		if !utf8.ValidString(pattern) {
			return
		}

		cache := NewCached[fuzzCacheable]()

		// Add some test items
		testItems := []fuzzCacheable{
			{dn: "cn=test,dc=example,dc=com", data: "test"},
			{dn: "cn=admin,dc=example,dc=com", data: "admin"},
			{dn: "cn=user,dc=example,dc=com", data: "user"},
		}
		cache.setAll(testItems)

		// Create a predicate based on fuzz input
		predicate := func(item fuzzCacheable) bool {
			if matchIfContains {
				return strings.Contains(item.data, pattern)
			}

			return item.data == pattern
		}

		// Test Filter doesn't panic
		results := cache.Filter(predicate)

		// Verify results match the predicate
		for _, result := range results {
			if !predicate(result) {
				t.Errorf("Filter returned item that doesn't match predicate: %v", result)
			}
		}
	})
}

// FuzzCacheSetAll tests setting items with fuzzed data
func FuzzCacheSetAll(f *testing.F) {
	f.Add("dn1", "data1", 1)
	f.Add("", "", 0)
	f.Add("dn", "data", 100)
	f.Add(strings.Repeat("x", 1000), strings.Repeat("y", 500), 10)

	f.Fuzz(func(t *testing.T, dn, data string, count int) {
		if !utf8.ValidString(dn) || !utf8.ValidString(data) {
			return
		}

		cache := NewCached[fuzzCacheable]()

		// Limit count to prevent OOM
		if count < 0 {
			count = 0
		}
		if count > 1000 {
			count = 1000
		}

		// Create items with unique DNs
		items := make([]fuzzCacheable, count)
		for i := range count {
			items[i] = fuzzCacheable{
				dn:   dn + "_" + string(rune('A'+i%26)),
				data: data,
			}
		}

		// setAll shouldn't panic
		cache.setAll(items)

		// Verify count
		if cache.Count() != count {
			t.Errorf("Expected %d items, got %d", count, cache.Count())
		}

		// Get should return all items
		result := cache.Get()
		if len(result) != count {
			t.Errorf("Expected %d items from Get, got %d", count, len(result))
		}
	})
}

// FuzzCacheFind tests Find with fuzzed predicates
func FuzzCacheFind(f *testing.F) {
	f.Add("test")
	f.Add("")
	f.Add("nonexistent")
	f.Add(strings.Repeat("x", 1000))

	f.Fuzz(func(t *testing.T, searchData string) {
		if !utf8.ValidString(searchData) {
			return
		}

		cache := NewCached[fuzzCacheable]()

		// Add test items
		testItems := []fuzzCacheable{
			{dn: "cn=item1,dc=example,dc=com", data: "alpha"},
			{dn: "cn=item2,dc=example,dc=com", data: "beta"},
			{dn: "cn=item3,dc=example,dc=com", data: "gamma"},
		}
		cache.setAll(testItems)

		// Test Find doesn't panic
		result, found := cache.Find(func(item fuzzCacheable) bool {
			return item.data == searchData
		})

		// Verify result consistency
		if found && result == nil {
			t.Error("Found is true but result is nil")
		}
		if !found && result != nil {
			t.Error("Found is false but result is not nil")
		}
		if found && result.data != searchData {
			t.Errorf("Found item with wrong data: got %s, want %s", result.data, searchData)
		}
	})
}

// FuzzCacheConcurrent tests concurrent cache operations
func FuzzCacheConcurrent(f *testing.F) {
	f.Add("key", "value", 5, 5)
	f.Add("k", "v", 10, 10)

	f.Fuzz(func(t *testing.T, dn, data string, numWriters, numReaders int) {
		if !utf8.ValidString(dn) || !utf8.ValidString(data) {
			return
		}

		// Limit goroutine count
		if numWriters < 1 {
			numWriters = 1
		}
		if numWriters > 10 {
			numWriters = 10
		}
		if numReaders < 1 {
			numReaders = 1
		}
		if numReaders > 10 {
			numReaders = 10
		}

		cache := NewCached[fuzzCacheable]()
		done := make(chan bool)

		// Writers
		for i := range numWriters {
			go func(id int) {
				for j := range 5 {
					items := []fuzzCacheable{
						{dn: dn + "_" + string(rune('A'+id)) + string(rune('0'+j)), data: data},
					}
					cache.setAll(items)
				}
				done <- true
			}(i)
		}

		// Readers
		for range numReaders {
			go func() {
				for range 5 {
					cache.Get()
					cache.Count()
					cache.FindByDN(dn)
				}
				done <- true
			}()
		}

		// Wait for all goroutines
		for range numWriters + numReaders {
			<-done
		}

		// Cache should still be functional
		cache.setAll([]fuzzCacheable{{dn: "final", data: "test"}})
		if cache.Count() != 1 {
			t.Error("Cache not functional after concurrent access")
		}
	})
}
