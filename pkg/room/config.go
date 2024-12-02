package room

import "time"

// Config defines the configuration settings for the WebSocket handler.
type Config[T any] struct {
	// How long to wait for all connected users to gracefully close. Default is 30s
	CloseTimeout time.Duration
}

// NewRoomConfig initializes a room configuration instance with reasonable defaults.
func NewRoomConfig[T any]() *Config[T] {
	cfg := &Config[T]{
		CloseTimeout: 30 * time.Second,
	}
	return cfg
}
