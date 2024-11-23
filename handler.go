package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"golang.org/x/net/websocket"
)

type empty struct{}

type Handler[T any] struct {
	cfg *Config[T]

	closingCh chan empty

	wsServer *websocket.Server
	rooms    []*Room[T]

	msgDecoder func([]byte) (T, error)
	msgEncoder func([]T) ([]byte, error)
}

func New[T any](cfg *Config[T]) *Handler[T] {
	if cfg == nil {
		cfg = NewConfig[T]()
	}
	h := &Handler[T]{
		cfg: cfg,

		closingCh: make(chan empty),

		wsServer: nil,
		rooms:    nil,
	}
	h.wsServer = &websocket.Server{
		Config: websocket.Config{
			Origin: nil,
		},
		Handshake: func(c *websocket.Config, r *http.Request) error {
			return nil
		},
		Handler: h.serveWS,
	}
	if h.cfg.Decoder != nil {
		h.msgDecoder = h.cfg.Decoder
	}
	if h.cfg.Encoder != nil {
		h.msgEncoder = h.cfg.Encoder
	}
	return h
}

func (h *Handler[T]) Close() error {
	if h.isClosing() {
		return errors.New("ws-handler: handler is closed")
	}
	close(h.closingCh)

	for _, room := range h.rooms {
		room.closingCh <- empty{}
		close(room.closingCh)

		// TODO: Add timeout for how long to wait for the room to close
	}
	return nil
}

func (h *Handler[T]) isClosing() bool {
	select {
	case <-h.closingCh:
		return true
	default:
		return false
	}
}

func (h *Handler[T]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.wsServer.ServeHTTP(w, r)
}

func (h *Handler[T]) serveWS(conn *websocket.Conn) {
}

func defaultMessageDecoder[T any](data []byte) (T, error) {
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		var defaultValue T
		return defaultValue, fmt.Errorf("ws-handler: unable to decode message: %w", err)
	}
	return v, nil
}

func defaultMessageEncoder[T any](msg T) ([]byte, error) {
	b, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("ws-handler: unable to encode message: %w", err)
	}
	return b, nil
}
