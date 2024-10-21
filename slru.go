package slru

// A prototype of SLRU.

import (
	"sync"

	"github.com/hey-kong/slru/list"
)

const (
	DefaultProbationRatio = 0.2
)

// entry holds the key and value of a cache entry.
type entry[K comparable, V any] struct {
	key   K
	value V
}

type SLRU[K comparable, V any] struct {
	lock          sync.RWMutex
	size          int
	items         map[K]*list.Element
	probation     *list.List
	protected     *list.List
	probationSize int
	protectedSize int
}

func New[K comparable, V any](size int) Cache[K, V] {
	return &SLRU[K, V]{
		size:          size,
		items:         make(map[K]*list.Element),
		probation:     list.New(),
		protected:     list.New(),
		probationSize: int(DefaultProbationRatio * float64(size)),
		protectedSize: size - int(DefaultProbationRatio*float64(size)),
	}
}

func (s *SLRU[K, V]) Set(key K, value V) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if e, ok := s.items[key]; ok {
		if e.List() == s.protected {
			s.protected.MoveToFront(e)
		}
		if e.List() == s.probation {
			s.items[e.Value.(*entry[K, V]).key] = s.protected.PushFront(e.Value)
			s.probation.Remove(e)
			if s.protected.Len() > s.protectedSize {
				s.evict(s.protected)
			}
		}
		e.Value.(*entry[K, V]).value = value
		return
	}

	if s.probation.Len() >= s.probationSize {
		s.evict(s.probation)
	}
	e := &entry[K, V]{key: key, value: value}
	s.items[key] = s.probation.PushFront(e)
}

func (s *SLRU[K, V]) Get(key K) (value V, ok bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if e, ok := s.items[key]; ok {
		if e.List() == s.protected {
			s.protected.MoveToFront(e)
		}
		if e.List() == s.probation {
			s.items[e.Value.(*entry[K, V]).key] = s.protected.PushFront(e.Value)
			s.probation.Remove(e)
			if s.protected.Len() > s.protectedSize {
				s.evict(s.protected)
			}
		}
		return e.Value.(*entry[K, V]).value, true
	}

	return
}

func (s *SLRU[K, V]) Contains(key K) (ok bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	_, ok = s.items[key]
	return
}

func (s *SLRU[K, V]) Peek(key K) (value V, ok bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if e, ok := s.items[key]; ok {
		return e.Value.(*entry[K, V]).value, true
	}

	return
}

func (s *SLRU[K, V]) Len() int {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.probation.Len() + s.protected.Len()
}

func (s *SLRU[K, V]) Purge() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.items = make(map[K]*list.Element)
	s.probation = list.New()
	s.protected = list.New()
}

func (s *SLRU[K, V]) evict(l *list.List) {
	o := l.Back()
	key := o.Value.(*entry[K, V]).key
	delete(s.items, key)
	l.Remove(o)
}
