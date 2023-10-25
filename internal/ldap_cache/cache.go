package ldap_cache

import (
	"sync"
)

type cacheable interface {
	DN() string
}

type Cache[T cacheable] struct {
	m     sync.RWMutex
	items []T
}

func NewCached[T cacheable]() Cache[T] {
	return Cache[T]{
		items: make([]T, 0),
	}
}

func (c *Cache[T]) setAll(v []T) {
	c.m.Lock()
	defer c.m.Unlock()

	c.items = v
}

func (c *Cache[T]) update(fn func(*T)) {
	c.m.Lock()
	defer c.m.Unlock()

	for idx, item := range c.items {
		fn(&item)
		c.items[idx] = item
	}
}

func (c *Cache[T]) Get() []T {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.items
}

func (c *Cache[T]) Find(fn func(T) bool) (v *T, found bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	for _, item := range c.items {
		if fn(item) {
			return &item, true
		}
	}

	return nil, false
}

func (c *Cache[T]) FindByDN(dn string) (v *T, found bool) {
	return c.Find(func(v T) bool {
		return v.DN() == dn
	})
}

func (c *Cache[T]) Filter(fn func(T) bool) (v []T) {
	c.m.RLock()
	defer c.m.RUnlock()

	for _, item := range c.items {
		if fn(item) {
			v = append(v, item)
		}
	}

	return v
}

func (c *Cache[T]) Count() int {
	c.m.RLock()
	defer c.m.RUnlock()

	return len(c.items)
}
