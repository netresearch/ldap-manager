package ldap_cache

import (
	"sync"
)

type cacheable interface {
	DN() string
}

type Cache[T cacheable] struct {
	m sync.RWMutex
	v []T
}

func NewCached[T cacheable]() Cache[T] {
	return Cache[T]{
		v: make([]T, 0),
	}
}

func (i *Cache[T]) set(v []T) {
	i.m.Lock()
	i.v = v
	i.m.Unlock()
}

func (i *Cache[T]) Get() []T {
	i.m.RLock()
	v := i.v
	i.m.RUnlock()

	return v
}

func (i *Cache[T]) Find(fn func(T) bool) (v *T, found bool) {
	i.m.RLock()
	defer i.m.RUnlock()

	for _, v := range i.v {
		if fn(v) {
			return &v, true
		}
	}

	return nil, false
}

func (i *Cache[T]) FindByDN(dn string) (v *T, found bool) {
	return i.Find(func(v T) bool {
		return v.DN() == dn
	})
}

func (i *Cache[T]) Count() int {
	i.m.RLock()
	defer i.m.RUnlock()

	return len(i.v)
}
