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

	// Start server
	server := &http.Server{Addr: ":8080"}
	go func() {
		log.Println("Server starting on :8080")
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
