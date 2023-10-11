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

func (i *Cache[T]) setAll(v []T) {
	i.m.Lock()
	defer i.m.Unlock()

	i.items = v
}

func (i *Cache[T]) update(fn func(*T)) {
	i.m.Lock()
	defer i.m.Unlock()

	for idx, item := range i.items {
		fn(&item)
		i.items[idx] = item
	}
}

func (i *Cache[T]) Get() []T {
	i.m.RLock()
	defer i.m.RUnlock()

	return i.items
}

func (i *Cache[T]) Find(fn func(T) bool) (v *T, found bool) {
	i.m.RLock()
	defer i.m.RUnlock()

	for _, item := range i.items {
		if fn(item) {
			return &item, true
		}
	}

	return nil, false
}

func (i *Cache[T]) FindByDN(dn string) (v *T, found bool) {
	return i.Find(func(v T) bool {
		return v.DN() == dn
	})
}

func (i *Cache[T]) Filter(fn func(T) bool) (v []T) {
	i.m.RLock()
	defer i.m.RUnlock()

	for _, item := range i.items {
		if fn(item) {
			v = append(v, item)
		}
	}

	return v
}

func (i *Cache[T]) Count() int {
	i.m.RLock()
	defer i.m.RUnlock()

	return len(i.items)
}
