package main

type Room[T any] struct {
	closingCh  chan empty
	completeCh chan empty

	clientRegisterCh   chan chan T
	clientUnregisterCh chan chan T
	clientMsgChs       map[chan T]empty

	msgCh chan T
}

func NewRoom[T any]() *Room[T] {
	r := &Room[T]{
		closingCh:  make(chan empty),
		completeCh: make(chan empty),

		clientRegisterCh:   make(chan chan T),
		clientUnregisterCh: make(chan chan T),
		clientMsgChs:       make(map[chan T]empty),

		msgCh: make(chan T),
	}

	go r.start()

	return r
}

func (r *Room[T]) start() {
	var (
		isClosing bool
		cleanup   = func() bool {
			isCleanable := isClosing && len(r.clientMsgChs) == 0
			if !isCleanable {
				return false
			}

			close(r.clientRegisterCh)
			close(r.clientUnregisterCh)
			close(r.msgCh)
			close(r.completeCh)
			return true
		}
	)
	for {
		select {
		case <-r.closingCh:
			isClosing = true
			if cleanup() {
				return
			}
		case clientMsgCh := <-r.clientRegisterCh:
			r.clientMsgChs[clientMsgCh] = empty{}
		case clientMsgCh := <-r.clientUnregisterCh:
			close(clientMsgCh)
			delete(r.clientMsgChs, clientMsgCh)

			if cleanup() {
				return
			}
		case msg := <-r.msgCh:
			for clientMsgCh := range r.clientMsgChs {
				clientMsgCh <- msg
			}
		}
	}
}

func (r *Room[T]) isClosing() bool {
	select {
	case <-r.closingCh:
		return true
	case <-r.completeCh:
		return true
	default:
		return false
	}
}
