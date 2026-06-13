/*
 A basic matchmaking service for the Chunk Explorer.
 Copyright (C) 2026 Yannic Rieger <oss@76k.io>

 This program is free software: you can redistribute it and/or modify
 it under the terms of the GNU Affero General Public License as published by
 the Free Software Foundation, either version 3 of the License, or
 (at your option) any later version.

 This program is distributed in the hope that it will be useful,
 but WITHOUT ANY WARRANTY; without even the implied warranty of
 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 GNU Affero General Public License for more details.

 You should have received a copy of the GNU Affero General Public License
 along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

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
