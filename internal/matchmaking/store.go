package matchmaking

import (
	"maps"
	"sync"
)

type ID interface {
	GetID() string
}

type Store[T ID] struct {
	items map[string]T
	mu    sync.Mutex
}

func NewStore[T ID]() *Store[T] {
	return &Store[T]{
		items: map[string]T{},
	}
}

func (s *Store[T]) Add(item T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.GetID()] = item
}

func (s *Store[T]) Get(id string) *T {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.items[id]
	if !ok {
		return nil
	}
	return &t
}

func (s *Store[T]) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
}

func (s *Store[T]) View() map[string]T {
	s.mu.Lock()
	defer s.mu.Unlock()

	cpy := make(map[string]T, len(s.items))
	maps.Copy(cpy, s.items)

	return cpy
}

func (s *Store[T]) Update(item T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.GetID()] = item
}

func (s *Store[T]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make(map[string]T)
}
