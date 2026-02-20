package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	wshub "github.com/KARTIKrocks/wshub"
)

// Message types
const (
	MsgTypeChat     = "chat"
	MsgTypeJoin     = "join"
	MsgTypeLeave    = "leave"
	MsgTypeRooms    = "rooms"
	MsgTypeUserList = "users"
)

// ChatMessage represents a chat message
type ChatMessage struct {
	Type    string `json:"type"`
	Room    string `json:"room,omitempty"`
	From    string `json:"from,omitempty"`
	Message string `json:"message,omitempty"`
	Users   []User `json:"users,omitempty"`
	Rooms   []Room `json:"rooms,omitempty"`
}

// User represents a connected user
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// Room represents a chat room
type Room struct {
	Name      string `json:"name"`
	UserCount int    `json:"userCount"`
}

// ChatServer wraps the WebSocket hub with chat-specific logic
type ChatServer struct {
	hub       *wshub.Hub
	usernames map[string]string // clientID -> username
	mu        sync.RWMutex
}

// NewChatServer creates a new chat server
func NewChatServer() *ChatServer {
	server := &ChatServer{
		usernames: make(map[string]string),
	}

	config := wshub.DefaultConfig().
		WithMaxMessageSize(1024 * 1024).
		WithCompression(true).
		WithCheckOrigin(wshub.AllowAllOrigins)

	limits := wshub.DefaultLimits().
		WithMaxConnections(10000).
		WithMaxConnectionsPerUser(5).
		WithMaxRoomsPerClient(10).
		WithMaxClientsPerRoom(100)

	server.hub = wshub.NewHub(
		wshub.WithConfig(config),
		wshub.WithLimits(limits),
		wshub.WithHooks(wshub.Hooks{
			BeforeConnect: func(r *http.Request) error {
				// You could add authentication here
				// For demo, we accept all connections
				return nil
			},

			AfterConnect: func(client *wshub.Client) {
				log.Printf("Client connected: %s", client.ID)

				// Send welcome message
				welcome := ChatMessage{
					Type:    "welcome",
					Message: "Welcome to the chat server!",
				}
				data, _ := json.Marshal(welcome)
				client.Send(data)
			},

			AfterDisconnect: func(client *wshub.Client) {
				log.Printf("Client disconnected: %s", client.ID)

				// Remove username
				server.mu.Lock()
				username := server.usernames[client.ID]
				delete(server.usernames, client.ID)
				server.mu.Unlock()

				// Notify rooms
				for _, room := range client.Rooms() {
					server.notifyRoomUsers(room, fmt.Sprintf("%s left the room", username))
				}
			},

			AfterRoomJoin: func(client *wshub.Client, room string) {
				server.mu.RLock()
				username := server.usernames[client.ID]
				server.mu.RUnlock()

				// Notify room members
				server.notifyRoomUsers(room, fmt.Sprintf("%s joined the room", username))

				// Send room user list to new member
				server.sendRoomUsers(client, room)
			},

			AfterRoomLeave: func(client *wshub.Client, room string) {
				server.mu.RLock()
				username := server.usernames[client.ID]
				server.mu.RUnlock()

				// Notify room members
				server.notifyRoomUsers(room, fmt.Sprintf("%s left the room", username))
			},

			OnError: func(client *wshub.Client, err error) {
				log.Printf("Client error %s: %v", client.ID, err)
			},
		}),
		wshub.WithMessageHandler(server.buildRouter()),
	)

	return server
}

// buildRouter wires up per-event handlers and wraps them with middleware.
func (s *ChatServer) buildRouter() wshub.HandlerFunc {
	logger := &SimpleLogger{}

	// Extractor reads only the "type" field — no assumptions about the rest.
	router := wshub.NewRouter(func(msg *wshub.Message) string {
		var env struct {
			Type string `json:"type"`
		}
		json.Unmarshal(msg.Data, &env)
		return env.Type
	})

	router.
		On("setUsername", s.decode(s.handleSetUsername)).
		On(MsgTypeJoin, s.decode(s.handleJoinRoom)).
		On(MsgTypeLeave, s.decode(s.handleLeaveRoom)).
		On(MsgTypeChat, s.decode(s.handleChatMessage)).
		On(MsgTypeRooms, func(c *wshub.Client, _ *wshub.Message) error {
			return s.handleGetRooms(c)
		})

	// Middleware wraps the entire router — recovery and logging apply to all events.
	chain := wshub.NewMiddlewareChain(router.Handle).
		Use(wshub.RecoveryMiddleware(logger)).
		Use(wshub.LoggingMiddleware(logger)).
		Build()

	return chain.Execute
}

// decode adapts a handler that expects a ChatMessage to the HandlerFunc signature.
func (s *ChatServer) decode(fn func(*wshub.Client, ChatMessage) error) wshub.HandlerFunc {
	return func(client *wshub.Client, msg *wshub.Message) error {
		var chatMsg ChatMessage
		if err := json.Unmarshal(msg.Data, &chatMsg); err != nil {
			return wshub.ErrInvalidMessage
		}
		return fn(client, chatMsg)
	}
}

func (s *ChatServer) handleSetUsername(client *wshub.Client, msg ChatMessage) error {
	s.mu.Lock()
	s.usernames[client.ID] = msg.Message
	s.mu.Unlock()

	client.SetMetadata("username", msg.Message)

	response := ChatMessage{
		Type:    "usernameSet",
		Message: msg.Message,
	}
	data, _ := json.Marshal(response)
	return client.Send(data)
}

func (s *ChatServer) handleJoinRoom(client *wshub.Client, msg ChatMessage) error {
	if err := s.hub.JoinRoom(client, msg.Room); err != nil {
		return err
	}

	response := ChatMessage{
		Type:    "joined",
		Room:    msg.Room,
		Message: fmt.Sprintf("Joined room: %s", msg.Room),
	}
	data, _ := json.Marshal(response)
	return client.Send(data)
}

func (s *ChatServer) handleLeaveRoom(client *wshub.Client, msg ChatMessage) error {
	if err := s.hub.LeaveRoom(client, msg.Room); err != nil {
		return err
	}

	response := ChatMessage{
		Type:    "left",
		Room:    msg.Room,
		Message: fmt.Sprintf("Left room: %s", msg.Room),
	}
	data, _ := json.Marshal(response)
	return client.Send(data)
}

func (s *ChatServer) handleChatMessage(client *wshub.Client, msg ChatMessage) error {
	s.mu.RLock()
	username := s.usernames[client.ID]
	s.mu.RUnlock()

	broadcast := ChatMessage{
		Type:    MsgTypeChat,
		Room:    msg.Room,
		From:    username,
		Message: msg.Message,
	}

	data, _ := json.Marshal(broadcast)

	if msg.Room != "" {
		return s.hub.BroadcastToRoom(msg.Room, data)
	}

	s.hub.Broadcast(data)
	return nil
}

func (s *ChatServer) handleGetRooms(client *wshub.Client) error {
	roomNames := s.hub.RoomNames()
	rooms := make([]Room, 0, len(roomNames))

	for _, name := range roomNames {
		rooms = append(rooms, Room{
			Name:      name,
			UserCount: s.hub.RoomCount(name),
		})
	}

	response := ChatMessage{
		Type:  MsgTypeRooms,
		Rooms: rooms,
	}

	data, _ := json.Marshal(response)
	return client.Send(data)
}

func (s *ChatServer) notifyRoomUsers(room, message string) {
	notification := ChatMessage{
		Type:    "notification",
		Room:    room,
		Message: message,
	}
	data, _ := json.Marshal(notification)
	s.hub.BroadcastToRoom(room, data)
}

func (s *ChatServer) sendRoomUsers(client *wshub.Client, room string) {
	clients := s.hub.RoomClients(room)
	users := make([]User, 0, len(clients))

	s.mu.RLock()
	for _, c := range clients {
		users = append(users, User{
			ID:       c.ID,
			Username: s.usernames[c.ID],
		})
	}
	s.mu.RUnlock()

	msg := ChatMessage{
		Type:  MsgTypeUserList,
		Room:  room,
		Users: users,
	}

	data, _ := json.Marshal(msg)
	client.Send(data)
}

func (s *ChatServer) Start() {
	go s.hub.Run()
}

func (s *ChatServer) Shutdown(ctx context.Context) error {
	return s.hub.Shutdown(ctx)
}

func (s *ChatServer) HandleHTTP() http.HandlerFunc {
	return s.hub.HandleHTTP()
}

// SimpleLogger implements wshub.Logger interface
type SimpleLogger struct{}

func (l *SimpleLogger) Debug(msg string, args ...any) {
	log.Printf("[DEBUG] %s %s", msg, formatLogArgs(args))
}

func (l *SimpleLogger) Info(msg string, args ...any) {
	log.Printf("[INFO] %s %s", msg, formatLogArgs(args))
}

func (l *SimpleLogger) Warn(msg string, args ...any) {
	log.Printf("[WARN] %s %s", msg, formatLogArgs(args))
}

func (l *SimpleLogger) Error(msg string, args ...any) {
	log.Printf("[ERROR] %s %s", msg, formatLogArgs(args))
}

// formatLogArgs formats structured key-value pairs for log output.
func formatLogArgs(args []any) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args)/2)
	for i := 0; i+1 < len(args); i += 2 {
		parts = append(parts, fmt.Sprintf("%v=%v", args[i], args[i+1]))
	}
	if len(args)%2 != 0 {
		parts = append(parts, fmt.Sprintf("%v", args[len(args)-1]))
	}
	return strings.Join(parts, " ")
}

func main() {
	// Create chat server
	chatServer := NewChatServer()
	chatServer.Start()

	// Set up HTTP routes
	http.HandleFunc("/ws", chatServer.HandleHTTP())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "chat.html")
	})

	// Start HTTP server
	server := &http.Server{Addr: ":8080"}
	go func() {
		log.Println("Chat server starting on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Shutdown chat server
	if err := chatServer.Shutdown(ctx); err != nil {
		log.Printf("Chat server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
