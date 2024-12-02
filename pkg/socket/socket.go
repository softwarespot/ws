package socket

import (
	"errors"
	"fmt"
	"io"
	"log"

	"golang.org/x/net/websocket"
)

type empty struct{}

type Packet struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

type Socket struct {
	subscribers map[string][]func(args ...any)

	ws *websocket.Conn
	id string

	onConnect    func()
	onDisconnect func()

	ackID  int
	ackFns map[int]func(...any)

	doneCh chan empty
}

func New(ws *websocket.Conn, id string) *Socket {
	s := &Socket{
		subscribers: map[string][]func(args ...any){},

		ws: ws,
		id: id,

		onConnect:    nil,
		onDisconnect: nil,

		ackID:  0,
		ackFns: map[int]func(...any){},

		doneCh: make(chan empty),
	}

	go s.onPacketReceived()

	return s
}

func (s *Socket) emit(event string, args ...any) {
	for _, fn := range s.subscribers[event] {
		fn(args...)
	}
}

func (s *Socket) on(event string, fn func(args ...any)) {
	s.subscribers[event] = append(s.subscribers[event], fn)
}

func (s *Socket) onPacketReceived() {
	for {
		var pkt Packet
		if err := websocket.JSON.Receive(s.ws, &pkt); err != nil {
			if errors.Is(err, io.EOF) {
				s.disconnect(errors.New("socket client disconnected"))
			} else {
				s.disconnect(fmt.Errorf("socket client disconnected with error: %s", err.Error()))
			}
			break
		}

		switch pkt.Type {
		case "ack":
			id, ok := pkt.Data["id"].(float64)
			if !ok {
				log.Printf("invalid id type: %v", pkt.Data["id"])
				continue
			}
			ackID := int(id)

			args, ok := pkt.Data["args"].([]any)
			if !ok {
				log.Printf("invalid args type: %v", pkt.Data["args"])
				continue
			}
			if ackFn, ok := s.ackFns[ackID]; ok {
				ackFn(args...)
				delete(s.ackFns, ackID)
			}
		case "event":
			event, ok := pkt.Data["event"].(string)
			if !ok {
				log.Printf("invalid event type: %v", pkt.Data["event"])
				continue
			}

			args, ok := pkt.Data["args"].([]any)
			if !ok {
				log.Printf("invalid args type: %v", pkt.Data["args"])
				continue
			}

			id, ok := pkt.Data["ackId"].(float64)
			if !ok {
				log.Printf("invalid ackId type: %v", pkt.Data["ackId"])
				continue
			}
			ackID := int(id)

			if ackID > 0 {
				args = append(args, func(args ...any) {
					s.emitAck(ackID, args...)
				})
			}
			s.emit(event, args...)
		}
	}
}

func (s *Socket) connect() error {
	err := websocket.JSON.Send(s.ws, Packet{
		Type: "connect",
		Data: map[string]any{
			"id": s.id,
		},
	})
	if err != nil {
		return fmt.Errorf("socket: sending connect packet: %w", err)
	}

	s.emit("connect")

	if s.onConnect != nil {
		// Avoid blocking
		go s.onConnect()
	}

	<-s.doneCh
	return nil
}

func (s *Socket) disconnect(err error) error {
	defer close(s.doneCh)

	if s.onDisconnect != nil {
		// Avoid blocking
		go s.onDisconnect()
	}

	reason := err.Error()
	err = websocket.JSON.Send(s.ws, Packet{
		Type: "disconnect",
		Data: map[string]any{
			"reason": reason,
		},
	})
	if err != nil {
		return fmt.Errorf("socket: sending disconnect packet: %w", err)
	}

	s.emit("disconnect", reason)

	if err := s.ws.Close(); err != nil {
		return fmt.Errorf("socket: closing websocket connection: %w", err)
	}
	return nil
}

func (s *Socket) OnConnect(fn func()) {
	s.onConnect = fn
}

func (s *Socket) OnDisconnect(fn func()) {
	s.onDisconnect = fn
}

func (s *Socket) ID() string {
	return s.id
}

func (s *Socket) Emit(event string, args ...any) error {
	var ackID int
	if ackFn, ok := GetAckFunc(args); ok {
		s.ackID += 1
		s.ackFns[s.ackID] = ackFn

		// Remove the "ack" function
		args = argDeleteLast(args)
		ackID = s.ackID
	}

	err := websocket.JSON.Send(s.ws, Packet{
		Type: "event",
		Data: map[string]any{
			"event": event,
			"args":  ensureInitializedArgs(args),
			"ackId": ackID,
		},
	})
	if err != nil {
		return fmt.Errorf("socket: calling emit: %w", err)
	}
	return nil
}

func (s *Socket) emitAck(id int, args ...any) error {
	err := websocket.JSON.Send(s.ws, Packet{
		Type: "ack",
		Data: map[string]any{
			"id":   id,
			"args": ensureInitializedArgs(args),
		},
	})
	if err != nil {
		return fmt.Errorf("socket: calling emitAck: %w", err)
	}
	return nil
}

func (s *Socket) On(event string, fn func(args ...any)) {
	s.on(event, fn)
}

func ensureInitializedArgs(args []any) []any {
	if len(args) == 0 {
		return []any{}
	}
	return args
}
