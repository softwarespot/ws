package room

import (
	"sync"
)

type Manager[T any] struct {
	rooms map[string]*Room[T]
	mu    sync.Mutex
}

func NewManager[T any]() *Manager[T] {
	return &Manager[T]{
		rooms: map[string]*Room[T]{},
	}
}

func (m *Manager[T]) Load(id string, cfg *Config[T]) *Room[T] {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[id]
	if ok {
		return room
	}

	room = New(id, cfg)
	m.rooms[id] = room
	return room
}

func (m *Manager[T]) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, room := range m.rooms {
		// Ignore the error
		room.Close()
	}
	clear(m.rooms)
	return nil
}
