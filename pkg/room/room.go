package room

import (
	"errors"
	"sync/atomic"
	"time"
)

var (
	ErrRoomClosed       = errors.New("room: room is closed")
	ErrRoomCloseTimeout = errors.New("room: timeout waiting for the users to close")
	ErrRoomUserNil      = errors.New("room: user cannot be nil")
)

type empty struct{}

type roomRegistration[T any] struct {
	user *User[T]
	ack  *acknowledgement
}

type roomMessage[T any] struct {
	msg T

	// Sender is optional.
	// When not defined with a user, the message is sent to ALL users
	sender *User[T]

	// ACK is optional
	ack *acknowledgement
}

type roomClose struct {
	ack *acknowledgement
}

type Room[T any] struct {
	id  string
	cfg *Config[T]

	closed       atomic.Bool
	closeCh      chan roomClose
	registerCh   chan roomRegistration[T]
	unregisterCh chan roomRegistration[T]

	users map[*User[T]]empty
	size  atomic.Int64

	msgCh chan roomMessage[T]
}

func New[T any](id string, cfg *Config[T]) *Room[T] {
	if cfg == nil {
		cfg = NewRoomConfig[T]()
	}
	r := &Room[T]{
		id:  id,
		cfg: cfg,

		closeCh:      make(chan roomClose),
		registerCh:   make(chan roomRegistration[T]),
		unregisterCh: make(chan roomRegistration[T]),

		users: map[*User[T]]empty{},

		msgCh: make(chan roomMessage[T]),
	}

	go r.start()

	return r
}

func (r *Room[T]) start() {
	for {
		select {
		case rc, ok := <-r.closeCh:
			if !ok {
				rc.ack.done(ErrRoomClosed)
				return
			}

			r.closed.Store(true)
			for user := range r.users {
				// Ignore the error
				user.Close()
			}
			clear(r.users)
			r.updateSize()

			close(r.closeCh)
			close(r.registerCh)
			close(r.unregisterCh)
			close(r.msgCh)

			rc.ack.done(nil)
			return
		case rr, ok := <-r.registerCh:
			if !ok {
				rr.ack.done(ErrRoomClosed)
				return
			}

			r.users[rr.user] = empty{}
			r.updateSize()

			rr.ack.done(nil)
		case rr, ok := <-r.unregisterCh:
			if !ok {
				rr.ack.done(ErrRoomClosed)
				return
			}

			delete(r.users, rr.user)
			r.updateSize()

			rr.ack.done(nil)
		case rm, ok := <-r.msgCh:
			if !ok {
				rm.ack.done(ErrRoomClosed)
				return
			}

			for user := range r.users {
				if rm.sender != user {
					// Ignore the error
					user.Send(rm.msg)
				}
			}
			rm.ack.done(nil)
		}
	}
}

func (r *Room[T]) ID() string {
	return r.id
}

func (r *Room[T]) Size() int {
	return int(r.size.Load())
}

func (r *Room[T]) updateSize() {
	size := int64(len(r.users))
	r.size.Store(size)
}

func (r *Room[T]) Register(user *User[T]) error {
	if r.closed.Load() {
		return ErrRoomClosed
	}
	if user == nil {
		return ErrRoomUserNil
	}

	req := roomRegistration[T]{
		user: user,
		ack:  newAcknowledgement(),
	}
	r.registerCh <- req

	return req.ack.wait()
}

func (r *Room[T]) Unregister(user *User[T]) error {
	if r.closed.Load() {
		return ErrRoomClosed
	}
	if user == nil {
		return ErrRoomUserNil
	}

	req := roomRegistration[T]{
		user: user,
		ack:  newAcknowledgement(),
	}
	r.unregisterCh <- req

	return req.ack.wait()
}

func (r *Room[T]) SendSync(sender *User[T], msg T) error {
	return r.send(sender, msg, true)
}

func (r *Room[T]) BroadcastSync(msg T) error {
	return r.SendSync(nil, msg)
}

func (r *Room[T]) Send(sender *User[T], msg T) error {
	return r.send(sender, msg, false)
}

func (r *Room[T]) Broadcast(msg T) error {
	return r.Send(nil, msg)
}

func (r *Room[T]) send(sender *User[T], msg T, isSync bool) error {
	if r.closed.Load() {
		return ErrRoomClosed
	}

	var ack *acknowledgement
	if isSync {
		ack = newAcknowledgement()
	}

	req := roomMessage[T]{
		msg:    msg,
		sender: sender,
		ack:    ack,
	}
	r.msgCh <- req

	return req.ack.wait()
}

func (r *Room[T]) Close() error {
	if r.closed.Load() {
		return ErrRoomClosed
	}

	req := roomClose{
		ack: newAcknowledgement(),
	}
	r.closeCh <- req

	// Wait for all the users to close or on timeout
	select {
	case err := <-req.ack.waiter():
		req.ack.close()
		return err
	case <-time.After(r.cfg.CloseTimeout):
		return ErrRoomCloseTimeout
	}
}
