package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/softwarespot/ws/pkg/room"
	"github.com/softwarespot/ws/pkg/socket"
	"golang.org/x/net/websocket"
)

type responseArgs []any

var rm = room.NewManager[responseArgs]()

func handler(ws *websocket.Conn) {
	log.Printf("connection established for %s", ws.Request().RemoteAddr)
	defer log.Printf("connection disconnected for %s", ws.Request().RemoteAddr)

	u := room.NewUser[responseArgs](getID())

	err := socket.IO(ws, u.ID(), func(s *socket.Socket) error {
		var currRoom *room.Room[responseArgs]

		leaveRoomFn := func() {
			if currRoom == nil {
				return
			}

			currRoom.Unregister(u)

			u.Send(responseArgs{
				"System",
				fmt.Sprintf("Left the room %s", currRoom.ID()),
			})
			currRoom.Send(u, responseArgs{
				"System",
				fmt.Sprintf("%s left the room %s", u.ID(), currRoom.ID()),
			})

			log.Printf("user %s left the room %s", u.ID(), currRoom.ID())

			currRoom = nil
		}

		s.OnConnect(func() {
			log.Printf("user %s opened the connection", u.ID())

			s.Emit("from-server", func(args ...any) {
				fmt.Println("acknowledge by client", args)
			})
			s.On("from-client", func(args ...any) {
				if ackFn, ok := socket.GetAckFunc(args); ok {
					ackFn()
				}
				fmt.Println("received from client")
			})

			for m := range u.Messages() {
				if err := s.Emit("message", m...); err != nil {
					log.Printf("error sending message to user %s: %v", u.ID(), err)
					break
				}
			}
		})
		s.OnDisconnect(func() {
			log.Printf("user %s closed the connection", u.ID())

			leaveRoomFn()
		})

		s.On("ping", func(args ...any) {
			if ackFn, ok := socket.GetAckFunc(args); ok {
				ackFn()
			}
		})

		s.On("join", func(args ...any) {
			leaveRoomFn()

			roomName, err := socket.ArgAt[string](args, 0)
			if err != nil {
				log.Printf("user %s encountered error: %v", u.ID(), err)
				return
			}

			currRoom = rm.Load(roomName, nil)
			log.Printf("user %s loaded the room %s", u.ID(), currRoom.ID())

			currRoom.Register(u)

			u.Send(responseArgs{
				"System",
				fmt.Sprintf("Joined the room %s", currRoom.ID()),
			})
			currRoom.Send(u, responseArgs{
				"System",
				fmt.Sprintf("%s joined the room %s", u.ID(), currRoom.ID()),
			})
			log.Printf("user %s joined the room %s", u.ID(), currRoom.ID())
		})
		s.On("leave", func(_ ...any) {
			leaveRoomFn()
		})
		s.On("message", func(args ...any) {
			if currRoom == nil {
				return
			}

			msg, err := socket.ArgAt[string](args, 0)
			if err != nil {
				log.Printf("user %s encountered error: %v", u.ID(), err)
				return
			}

			u.Send(responseArgs{
				"You",
				msg,
			})
			currRoom.Send(u, responseArgs{
				"User",
				msg,
			})
			log.Printf("user %s broadcast message %q to the room %s", u.ID(), msg, currRoom.ID())
		})

		return nil
	})

	log.Printf("user %s completed: %v", u.ID(), err)
}

func getID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("creating a new ID: %w", err))
	}
	return fmt.Sprintf("%x-%d", b, time.Now().UnixMilli())
}

func main() {
	wsServer := &websocket.Server{
		Config: websocket.Config{
			Origin: nil,
		},
		Handshake: func(_ *websocket.Config, _ *http.Request) error {
			return nil
		},
		Handler: handler,
	}
	http.Handle("/ws", wsServer)

	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		f, err := os.Open("./index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()

		w.Header().Set("Content-Type", "text/html")
		_, err = io.Copy(w, f)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	http.ListenAndServe(":8080", nil)
}
