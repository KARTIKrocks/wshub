// Example: minimal echo/broadcast server with wshub.
//
// This is the smallest useful wshub server: every message a client sends is
// broadcast back to all connected clients, including the sender.
//
// Usage:
//
//	go run ./examples/simple
//	open http://localhost:8080
//
// Set PORT to run on something other than 8080.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	wshub "github.com/KARTIKrocks/wshub"
)

func main() {
	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}

	// Create hub with functional options
	var hub *wshub.Hub
	hub = wshub.NewHub(
		wshub.WithMessageHandler(func(client *wshub.Client, msg *wshub.Message) error {
			log.Printf("Message from client %s: %s", client.ID, msg.Text())

			// Echo message back to all clients
			hub.Broadcast(msg.Data)
			return nil
		}),
	)

	// Start the hub
	go hub.Run()

	// Set up HTTP handler
	http.HandleFunc("/ws", hub.HandleHTTP())

	// Serve static files
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	// Start server. ReadHeaderTimeout guards against slow-header (Slowloris)
	// connections holding a goroutine open indefinitely.
	server := &http.Server{Addr: addr, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("Server starting on %s", addr)
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

	// Shutdown WebSocket hub
	if err := hub.Shutdown(ctx); err != nil {
		log.Printf("WebSocket hub shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
