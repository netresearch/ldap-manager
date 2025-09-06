package ldap_cache

import (
	"sync"
	"testing"
)

// mockCacheable is a test implementation of the cacheable interface
type mockCacheable struct {
	dn   string
	data string
}

func (m mockCacheable) DN() string {
	return m.dn
}

func TestNewCached(t *testing.T) {
	cache := NewCached[mockCacheable]()
	
	if cache.items == nil {
		t.Error("Expected items to be initialized")
	}
	
	if len(cache.items) != 0 {
		t.Errorf("Expected empty cache, got %d items", len(cache.items))
	}
	
	if cache.Count() != 0 {
		t.Errorf("Expected count 0, got %d", cache.Count())
	}
}

func TestCacheSetAll(t *testing.T) {
	cache := NewCached[mockCacheable]()
	
	items := []mockCacheable{
		{dn: "cn=user1,dc=example,dc=com", data: "user1"},
		{dn: "cn=user2,dc=example,dc=com", data: "user2"},
	}
	
	cache.setAll(items)
	
	if cache.Count() != 2 {
		t.Errorf("Expected count 2, got %d", cache.Count())
	}
	
	retrieved := cache.Get()
	if len(retrieved) != 2 {
		t.Errorf("Expected 2 items, got %d", len(retrieved))
	}
	
	if retrieved[0].dn != "cn=user1,dc=example,dc=com" {
		t.Errorf("Expected first item DN to be 'cn=user1,dc=example,dc=com', got '%s'", retrieved[0].dn)
	}
}

func TestCacheUpdate(t *testing.T) {
	cache := NewCached[mockCacheable]()
	
	items := []mockCacheable{
		{dn: "cn=user1,dc=example,dc=com", data: "original"},
		{dn: "cn=user2,dc=example,dc=com", data: "original"},
	}
	cache.setAll(items)
	
	// Update all items
	cache.update(func(item *mockCacheable) {
		item.data = "updated"
	})
	
	retrieved := cache.Get()
	for i, item := range retrieved {
		if item.data != "updated" {
			t.Errorf("Item %d was not updated, got '%s'", i, item.data)
		}
	}
}

func TestCacheGet(t *testing.T) {
	cache := NewCached[mockCacheable]()
	
	t.Run("empty cache", func(t *testing.T) {
		items := cache.Get()
		if len(items) != 0 {
			t.Errorf("Expected empty slice, got %d items", len(items))
		}
	})
	
	t.Run("populated cache", func(t *testing.T) {
		testItems := []mockCacheable{
			{dn: "cn=user1,dc=example,dc=com", data: "user1"},
		}
		cache.setAll(testItems)
		
		items := cache.Get()
		if len(items) != 1 {
			t.Errorf("Expected 1 item, got %d", len(items))
		}
		
		if items[0].dn != "cn=user1,dc=example,dc=com" {
			t.Errorf("Expected DN 'cn=user1,dc=example,dc=com', got '%s'", items[0].dn)
		}
	})
}

func TestCacheFind(t *testing.T) {
	cache := NewCached[mockCacheable]()
	
	items := []mockCacheable{
		{dn: "cn=user1,dc=example,dc=com", data: "user1"},
		{dn: "cn=user2,dc=example,dc=com", data: "user2"},
		{dn: "cn=admin,dc=example,dc=com", data: "admin"},
	}
	cache.setAll(items)
	
	t.Run("find existing item", func(t *testing.T) {
		item, found := cache.Find(func(m mockCacheable) bool {
			return m.data == "user2"
		})
		
		if !found {
			t.Error("Expected to find item")
		}
		
		if item.dn != "cn=user2,dc=example,dc=com" {
			t.Errorf("Expected DN 'cn=user2,dc=example,dc=com', got '%s'", item.dn)
		}
	})
	
	t.Run("find non-existent item", func(t *testing.T) {
		item, found := cache.Find(func(m mockCacheable) bool {
			return m.data == "nonexistent"
		})
		
		if found {
			t.Error("Expected not to find item")
		}
		
		if item != nil {
			t.Error("Expected nil item when not found")
		}
	})
	
	t.Run("find first match", func(t *testing.T) {
		// Add duplicate data to test first match behavior
		duplicateItems := []mockCacheable{
			{dn: "cn=test1,dc=example,dc=com", data: "duplicate"},
			{dn: "cn=test2,dc=example,dc=com", data: "duplicate"},
		}
		cache.setAll(duplicateItems)
		
		item, found := cache.Find(func(m mockCacheable) bool {
			return m.data == "duplicate"
		})
		
		if !found {
			t.Error("Expected to find item")
		}
		
		// Should return the first match
		if item.dn != "cn=test1,dc=example,dc=com" {
			t.Errorf("Expected first match DN 'cn=test1,dc=example,dc=com', got '%s'", item.dn)
		}
	})
}

func TestCacheFindByDN(t *testing.T) {
	cache := NewCached[mockCacheable]()
	
	items := []mockCacheable{
		{dn: "cn=user1,dc=example,dc=com", data: "user1"},
		{dn: "cn=user2,dc=example,dc=com", data: "user2"},
	}
	cache.setAll(items)
	
	t.Run("find by existing DN", func(t *testing.T) {
		item, found := cache.FindByDN("cn=user1,dc=example,dc=com")
		
		if !found {
			t.Error("Expected to find item by DN")
		}
		
		if item.data != "user1" {
			t.Errorf("Expected data 'user1', got '%s'", item.data)
		}
	})
	
	t.Run("find by non-existent DN", func(t *testing.T) {
		item, found := cache.FindByDN("cn=nonexistent,dc=example,dc=com")
		
		if found {
			t.Error("Expected not to find item by DN")
		}
		
		if item != nil {
			t.Error("Expected nil item when not found")
		}
	})
}

func TestCacheFilter(t *testing.T) {
	cache := NewCached[mockCacheable]()
	
	items := []mockCacheable{
		{dn: "cn=user1,dc=example,dc=com", data: "user"},
		{dn: "cn=user2,dc=example,dc=com", data: "user"},
		{dn: "cn=admin1,dc=example,dc=com", data: "admin"},
		{dn: "cn=admin2,dc=example,dc=com", data: "admin"},
	}
	cache.setAll(items)
	
	t.Run("filter matching items", func(t *testing.T) {
		users := cache.Filter(func(m mockCacheable) bool {
			return m.data == "user"
		})
		
		if len(users) != 2 {
			t.Errorf("Expected 2 users, got %d", len(users))
		}
		
		for _, user := range users {
			if user.data != "user" {
				t.Errorf("Expected data 'user', got '%s'", user.data)
			}
		}
	})
	
	t.Run("filter with no matches", func(t *testing.T) {
		results := cache.Filter(func(m mockCacheable) bool {
			return m.data == "nonexistent"
		})
		
		if len(results) != 0 {
			t.Errorf("Expected 0 results, got %d", len(results))
		}
	})
	
	t.Run("filter all items", func(t *testing.T) {
		all := cache.Filter(func(m mockCacheable) bool {
			return true
		})
		
		if len(all) != 4 {
			t.Errorf("Expected 4 items, got %d", len(all))
		}
	})
}

func TestCacheCount(t *testing.T) {
	cache := NewCached[mockCacheable]()
	
	if cache.Count() != 0 {
		t.Errorf("Expected count 0 for empty cache, got %d", cache.Count())
	}
	
	items := []mockCacheable{
		{dn: "cn=user1,dc=example,dc=com", data: "user1"},
		{dn: "cn=user2,dc=example,dc=com", data: "user2"},
		{dn: "cn=user3,dc=example,dc=com", data: "user3"},
	}
	cache.setAll(items)
	
	if cache.Count() != 3 {
		t.Errorf("Expected count 3, got %d", cache.Count())
	}
}

func TestCacheConcurrentAccess(t *testing.T) {
	cache := NewCached[mockCacheable]()
	
	// Initialize with some data
	initialItems := []mockCacheable{
		{dn: "cn=user1,dc=example,dc=com", data: "user1"},
		{dn: "cn=user2,dc=example,dc=com", data: "user2"},
	}
	cache.setAll(initialItems)
	
	var wg sync.WaitGroup
	
	// Test concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			// Concurrent Get operations
			items := cache.Get()
			if len(items) < 0 {
				t.Errorf("Unexpected items length: %d", len(items))
			}
			
			// Concurrent Find operations
			_, _ = cache.FindByDN("cn=user1,dc=example,dc=com")
			
			// Concurrent Filter operations
			cache.Filter(func(m mockCacheable) bool {
				return m.data == "user1"
			})
			
			// Concurrent Count operations
			cache.Count()
		}()
	}
	
	// Test concurrent writes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			
			newItems := []mockCacheable{
				{dn: "cn=concurrent1,dc=example,dc=com", data: "concurrent1"},
				{dn: "cn=concurrent2,dc=example,dc=com", data: "concurrent2"},
			}
			cache.setAll(newItems)
		}(i)
	}
	
	// Test concurrent updates
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			
			cache.update(func(item *mockCacheable) {
				item.data = "updated"
			})
		}(i)
	}
	
	wg.Wait()
	
	// Verify cache is still functional
	count := cache.Count()
	if count < 0 {
		t.Errorf("Invalid count after concurrent operations: %d", count)
	}
}