package todo

import (
	"errors"
	"sync"
	"time"
)

// ErrNotFound is returned when a todo with the given ID doesn't exist.
var ErrNotFound = errors.New("todo not found")

// Store is a simple in-memory, thread-safe storage for Todos.
// In a real app you'd swap this for a database (Postgres, SQLite, etc.),
// but the handler code wouldn't need to change much since it only talks
// to this interface-like struct.
type Store struct {
	mu     sync.Mutex
	nextID int
	items  map[int]Todo
}

// NewStore creates an empty Store.
func NewStore() *Store {
	return &Store{
		nextID: 1,
		items:  make(map[int]Todo),
	}
}

// Create adds a new todo and returns it (with ID + CreatedAt populated).
func (s *Store) Create(title string) Todo {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := Todo{
		ID:        s.nextID,
		Title:     title,
		Done:      false,
		CreatedAt: time.Now(),
	}
	s.items[t.ID] = t
	s.nextID++
	return t
}

// All returns every todo, sorted by ID.
func (s *Store) All() []Todo {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]Todo, 0, len(s.items))
	for _, t := range s.items {
		result = append(result, t)
	}
	// simple insertion sort by ID (fine for small lists)
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j].ID < result[j-1].ID; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	return result
}

// Get returns a single todo by ID.
func (s *Store) Get(id int) (Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.items[id]
	if !ok {
		return Todo{}, ErrNotFound
	}
	return t, nil
}

// Update replaces the title/done fields of an existing todo.
func (s *Store) Update(id int, title string, done bool) (Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.items[id]
	if !ok {
		return Todo{}, ErrNotFound
	}
	t.Title = title
	t.Done = done
	s.items[id] = t
	return t, nil
}

// Delete removes a todo by ID.
func (s *Store) Delete(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.items[id]; !ok {
		return ErrNotFound
	}
	delete(s.items, id)
	return nil
}
