package room

import "sync"

type acknowledgement struct {
	closeOnce sync.Once
	closed    bool
	ch        chan error
}

func newAcknowledgement() *acknowledgement {
	return &acknowledgement{
		ch: make(chan error),
	}
}

func (a *acknowledgement) done(err error) {
	if a == nil {
		return
	}
	a.ch <- err
}

func (a *acknowledgement) waiter() <-chan error {
	return a.ch
}

func (a *acknowledgement) wait() error {
	if a == nil {
		return nil
	}

	err := <-a.waiter()
	a.close()

	return err
}

func (a *acknowledgement) close() {
	if a == nil {
		return
	}

	a.closeOnce.Do(func() {
		close(a.ch)
		a.closed = true
	})
}
