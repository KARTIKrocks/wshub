// Example: per-user server-push notifications with wshub.
//
// The other examples are client-driven: a browser sends a message and the
// server reacts. Notifications are the opposite — the server decides when to
// push, and it addresses a *user* rather than a connection. wshub keeps a user
// index for exactly this, so one call reaches every tab and device that user
// has open:
//
//	hub.SendToUser(userID, payload)
//
// This demonstrates:
//
//   - Identifying connections with client.SetUserID, so the hub can route by user
//   - hub.SendToUser fanning one payload out to all of a user's connections
//   - Pushing from outside the WebSocket path (an HTTP endpoint, a background job)
//   - Marshaling a payload once and reusing the bytes for every recipient
//
// Usage:
//
//	go run ./examples/notifications
//	open http://localhost:8080
//
// Open the page twice as the same user to watch a single send arrive in both
// tabs. To push from outside the browser entirely:
//
//	curl -X POST localhost:8080/notify \
//	  -d '{"user":"alice","title":"Deploy finished","body":"build #412 is live"}'
//
// Set PORT to run on something other than 8080.
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	wshub "github.com/KARTIKrocks/wshub"
)

// notificationsHTML is embedded rather than served from disk with
// http.ServeFile: go run resolves relative paths against the caller's working
// directory, not the package directory, so "go run ./examples/notifications"
// from the repo root would 404 against a bare "notifications.html".
//
//go:embed notifications.html
var notificationsHTML []byte

// Notification is the payload pushed to the browser. Level drives how the page
// renders it; nothing here is required by wshub, which only moves bytes.
type Notification struct {
	Level string `json:"level"` // info | success | warning
	Title string `json:"title"`
	Body  string `json:"body"`
	At    string `json:"at"`
}

// notifyRequest is the body accepted by POST /notify. An empty User means
// "everyone" and routes to Broadcast instead of SendToUser.
type notifyRequest struct {
	User  string `json:"user"`
	Level string `json:"level"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// clientCommand is the only thing the browser sends us: a request to kick off
// the simulated background job.
type clientCommand struct {
	Type string `json:"type"`
}

// maxNotifyBody caps POST /notify so an oversized body cannot be read into
// memory unbounded.
const maxNotifyBody = 4 << 10 // 4 KiB

// jobDelay is how long the simulated background job "runs" before it reports
// back. Long enough to switch tabs and watch the result land in both.
const jobDelay = 3 * time.Second

type server struct {
	hub *wshub.Hub
}

func newServer() *server {
	s := &server{}

	// A user is allowed several concurrent connections — that is the point of
	// this example, since SendToUser fans out across all of them. The limit
	// only stops one user from opening an unbounded number.
	limits := wshub.DefaultLimits().WithMaxConnectionsPerUser(10)

	s.hub = wshub.NewHub(
		wshub.WithLimits(limits),
		wshub.WithHooks(wshub.Hooks{
			// Identity is taken from ?user= purely to keep this example about
			// notifications. See examples/auth for validating a real token
			// before the upgrade.
			BeforeConnect: func(r *http.Request) error {
				if r.URL.Query().Get("user") == "" {
					return wshub.ErrAuthenticationFailed
				}
				return nil
			},

			// SetUserID is what puts the connection into the hub's user index.
			// Without it SendToUser has nothing to route to.
			AfterConnect: func(client *wshub.Client) {
				userID := client.Request().URL.Query().Get("user")
				if err := client.SetUserID(userID); err != nil {
					// The only expected failure is MaxConnectionsPerUser.
					log.Printf("set user id %q: %v", userID, err)
					client.Close()
					return
				}

				open := len(s.hub.GetClientsByUserID(userID))
				log.Printf("user %s connected (%d connection(s), client %s)",
					userID, open, client.ID)

				// A welcome belongs to this connection alone, so it goes via
				// client.Send. Using SendToUser here would push it into the
				// user's *other* tabs too — the distinction this example is
				// about, and easy to get wrong.
				s.notifyClient(client, Notification{
					Level: "success",
					Title: "Connected",
					Body: fmt.Sprintf("Signed in as %s — %d tab(s) open. Anything sent to %s arrives in all of them.",
						userID, open, userID),
				})
			},

			AfterDisconnect: func(client *wshub.Client) {
				log.Printf("user %s disconnected (client %s)", client.GetUserID(), client.ID)
			},

			OnError: func(client *wshub.Client, err error) {
				log.Printf("client %s error: %v", client.ID, err)
			},
		}),
		wshub.WithMessageHandler(s.handleMessage),
	)

	return s
}

// handleMessage is deliberately thin: in a notification system the interesting
// traffic goes server to client, not the other way round.
func (s *server) handleMessage(client *wshub.Client, msg *wshub.Message) error {
	var cmd clientCommand
	if err := json.Unmarshal(msg.Data, &cmd); err != nil {
		return wshub.ErrInvalidMessage
	}

	if cmd.Type != "startJob" {
		return wshub.ErrInvalidMessage
	}

	// The acknowledgement is for the tab that clicked, so it goes to the one
	// connection...
	userID := client.GetUserID()
	s.notifyClient(client, Notification{
		Level: "info",
		Title: "Job queued",
		Body:  "Export started. Close this tab if you like — the result follows you.",
	})

	// ...but the result is addressed to the user. This is the pattern the
	// example exists to show: work finishes on some other goroutine, long
	// after the request that started it, and by then the connection that
	// asked may be gone while the user is still around on another tab.
	go func() {
		time.Sleep(jobDelay)
		s.notifyUser(userID, Notification{
			Level: "success",
			Title: "Export ready",
			Body:  "report.csv finished processing and is ready to download.",
		})
	}()

	return nil
}

// notifyUser delivers one notification to every connection belonging to userID.
// The payload is marshaled once here; wshub reuses the same bytes for each
// recipient rather than re-encoding per connection.
func (s *server) notifyUser(userID string, n Notification) {
	data, err := s.encode(n)
	if err != nil {
		log.Printf("encode notification: %v", err)
		return
	}
	s.hub.SendToUser(userID, data)
}

// notifyClient delivers one notification to a single connection, leaving the
// user's other connections untouched.
func (s *server) notifyClient(client *wshub.Client, n Notification) {
	data, err := s.encode(n)
	if err != nil {
		log.Printf("encode notification: %v", err)
		return
	}
	if err := client.Send(data); err != nil {
		log.Printf("send to client %s: %v", client.ID, err)
	}
}

// notifyAll delivers one notification to every connected client, regardless of
// user.
func (s *server) notifyAll(n Notification) {
	data, err := s.encode(n)
	if err != nil {
		log.Printf("encode notification: %v", err)
		return
	}
	s.hub.Broadcast(data)
}

func (s *server) encode(n Notification) ([]byte, error) {
	if n.At == "" {
		n.At = time.Now().Format("15:04:05")
	}
	if n.Level == "" {
		n.Level = "info"
	}
	return json.Marshal(n)
}

// handleNotify lets anything that can make an HTTP request push a
// notification — a cron job, a webhook receiver, another service. This is the
// realistic entry point; the browser is only ever the recipient.
//
// It is deliberately unauthenticated so the example stays about notifications,
// which also means anyone who can reach the port can push to any user. Put it
// behind authentication, or on an interface that is not publicly reachable,
// before doing anything like this outside a demo.
func (s *server) handleNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req notifyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxNotifyBody)).Decode(&req); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	n := Notification{Level: req.Level, Title: req.Title, Body: req.Body}

	// No user means "announce to everyone" — the contrast between targeted
	// delivery and a plain broadcast is the whole lesson of this endpoint.
	if req.User == "" {
		s.notifyAll(n)
		log.Printf("announced %q to %d client(s)", req.Title, s.hub.ClientCount())
		s.writeJSON(w, map[string]any{
			"delivered": "broadcast",
			"clients":   s.hub.ClientCount(),
		})
		return
	}

	// GetClientsByUserID is only used here to report how many connections the
	// notification reached; SendToUser does its own lookup.
	delivered := len(s.hub.GetClientsByUserID(req.User))
	s.notifyUser(req.User, n)
	log.Printf("sent %q to user %s (%d connection(s))", req.Title, req.User, delivered)

	// Zero connections is not an error: the user is simply offline right now.
	// A real system would also persist the notification for their next login.
	s.writeJSON(w, map[string]any{
		"delivered": req.User,
		"clients":   delivered,
	})
}

// handleOnline reports which users are currently connected, so you can see the
// user index the hub maintains. Unauthenticated for the same reason as
// handleNotify, and it discloses who is signed in — gate it before exposing
// this anywhere real.
func (s *server) handleOnline(w http.ResponseWriter, _ *http.Request) {
	counts := make(map[string]int)
	for _, client := range s.hub.Clients() {
		if userID := client.GetUserID(); userID != "" {
			counts[userID]++
		}
	}
	s.writeJSON(w, map[string]any{
		"users":       counts,
		"connections": s.hub.ClientCount(),
	})
}

func (s *server) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func main() {
	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}

	s := newServer()
	go s.hub.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.hub.HandleHTTP())
	mux.HandleFunc("/notify", s.handleNotify)
	mux.HandleFunc("/online", s.handleOnline)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(notificationsHTML)
	})

	// ReadHeaderTimeout guards against slow-header (Slowloris) connections
	// holding a goroutine open indefinitely; ReadTimeout bounds the whole
	// request read. Neither applies once a connection is hijacked for the
	// WebSocket, so long-lived sockets are unaffected.
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
	}

	go func() {
		log.Printf("Notifications example running on %s", addr)
		log.Println("  /ws      - WebSocket endpoint (requires ?user=<id>)")
		log.Println("  /notify  - POST a notification to one user, or to everyone")
		log.Println("  /online  - Currently connected users")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	if err := s.hub.Shutdown(ctx); err != nil {
		log.Printf("Hub shutdown error: %v", err)
	}
}
