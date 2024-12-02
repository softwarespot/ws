package room

import (
	"errors"
	"sync"
	"sync/atomic"
)

var ErrUserClosed = errors.New("room-user: user is closed")

type User[T any] struct {
	id string

	closed    atomic.Bool
	closeOnce sync.Once

	msgCh chan T
}

func NewUser[T any](id string) *User[T] {
	return &User[T]{
		id:    id,
		msgCh: make(chan T),
	}
}

func (u *User[T]) ID() string {
	return u.id
}

func (u *User[T]) Send(msg T) error {
	if u.closed.Load() {
		return ErrUserClosed
	}

	u.msgCh <- msg
	return nil
}

func (u *User[T]) Messages() <-chan T {
	return u.msgCh
}

func (u *User[T]) Close() error {
	if u.closed.Load() {
		return ErrUserClosed
	}

	u.closeOnce.Do(func() {
		u.closed.Store(true)
		close(u.msgCh)
	})
	return nil
}
