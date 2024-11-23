package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/websocket"
)

type Client struct {
	ws   *websocket.Conn
	send chan string
}

type Room struct {
	clients   map[*Client]bool
	join      chan *Client
	leave     chan *Client
	broadcast chan string
	mutex     sync.Mutex
	done      chan struct{}
}

var rooms = make(map[string]*Room)
var roomsMutex sync.Mutex

func newRoom() *Room {
	return &Room{
		clients:   make(map[*Client]bool),
		join:      make(chan *Client),
		leave:     make(chan *Client),
		broadcast: make(chan string),
		done:      make(chan struct{}),
	}
}

func (r *Room) run() {
	for {
		select {
		case client := <-r.join:
			r.mutex.Lock()
			r.clients[client] = true
			r.mutex.Unlock()
			log.Printf("Client joined room. Total clients: %d", len(r.clients))
		case client := <-r.leave:
			r.mutex.Lock()
			delete(r.clients, client)
			close(client.send)
			r.mutex.Unlock()
			log.Printf("Client left room. Total clients: %d", len(r.clients))
		case message := <-r.broadcast:
			r.mutex.Lock()
			for client := range r.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(r.clients, client)
					log.Printf("Removed unresponsive client. Total clients: %d", len(r.clients))
				}
			}
			r.mutex.Unlock()
		case <-r.done:
			return
		}
	}
}

func handleWebSocket(ws *websocket.Conn) {
	fmt.Println("HELLO")
	client := &Client{ws: ws, send: make(chan string, 256)}
	go clientWriter(ws, client.send)
	var currentRoom *Room

	log.Printf("New WebSocket connection established from %s", ws.Request().RemoteAddr)

	for {
		var message string
		err := websocket.Message.Receive(ws, &message)
		if err != nil {
			log.Printf("Error receiving message from %s: %v", ws.Request().RemoteAddr, err)
			if currentRoom != nil {
				currentRoom.leave <- client
			}
			break
		}

		log.Printf("Received message from %s: %s", ws.Request().RemoteAddr, message)

		parts := strings.SplitN(message, ":", 2)
		if len(parts) != 2 {
			log.Printf("Invalid message format from %s: %s", ws.Request().RemoteAddr, message)
			continue
		}

		command, payload := parts[0], parts[1]

		switch command {
		case "create", "join":
			roomsMutex.Lock()
			room, exists := rooms[payload]
			if !exists {
				room = newRoom()
				rooms[payload] = room
				go room.run()
				log.Printf("Created new room: %s", payload)
			}
			roomsMutex.Unlock()

			if currentRoom != nil {
				currentRoom.leave <- client
			}
			room.join <- client
			currentRoom = room
			client.send <- "System: Joined room " + payload
			log.Printf("Client %s joined room: %s", ws.Request().RemoteAddr, payload)

		case "leave":
			if currentRoom != nil {
				currentRoom.leave <- client
				currentRoom = nil
				client.send <- "System: Left the room"
				log.Printf("Client %s left the room", ws.Request().RemoteAddr)
			}

		case "message":
			if currentRoom != nil {
				echoMessage := "You: " + payload
				client.send <- echoMessage // Echo back to sender
				broadcastMessage := "User: " + payload
				currentRoom.broadcast <- broadcastMessage
				log.Printf("Broadcast message in room from %s: %s", ws.Request().RemoteAddr, payload)
			}
		}
	}
}

func clientWriter(ws *websocket.Conn, ch <-chan string) {
	for msg := range ch {
		err := websocket.Message.Send(ws, msg)
		if err != nil {
			log.Printf("Error sending message to %s: %v", ws.Request().RemoteAddr, err)
			break
		}
	}
}

var allowedOrigins = map[string]bool{
	"http://localhost:8080": true,
	"http://127.0.0.1:8080": true,
	// Add any other allowed origins here
}

func checkOrigin(config *websocket.Config, req *http.Request) error {
	if config == nil || config.Origin == nil {
		return nil
	}
	origin := config.Origin.String()
	if allowedOrigins[origin] {
		return nil
	}
	return fmt.Errorf("origin not allowed: %s", origin)
}

func main() {
	wsServer := &websocket.Server{
		Handler:   websocket.Handler(handleWebSocket),
		Handshake: checkOrigin,
	}
	http.Handle("/ws", wsServer)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	server := &http.Server{
		Addr: ":8080",
	}

	go func() {
		log.Println("Server starting on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe: %v", err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
