package socket

import (
	"fmt"

	"golang.org/x/net/websocket"
)

func IO(ws *websocket.Conn, id string, socketHandler func(s *Socket) error) error {
	s := New(ws, id)
	if err := socketHandler(s); err != nil {
		return fmt.Errorf("socket: handling socket: %w", err)
	}
	if err := s.connect(); err != nil {
		return err
	}
	return nil
}
