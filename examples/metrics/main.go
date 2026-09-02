// Example: Prometheus-style metrics with wshub.
//
// This demonstrates implementing the MetricsCollector interface and exposing
// metrics on a /metrics endpoint. Uses wshub's built-in DebugMetrics for
// simplicity — in production, replace with a real Prometheus collector.
//
// Usage:
//
//	go run ./examples/metrics
//	open http://localhost:8080
//
// Set PORT to run on something other than 8080.
package main

import (
	"context"
	"fmt"
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

	metrics := wshub.NewDebugMetrics()

	var hub *wshub.Hub
	hub = wshub.NewHub(
		wshub.WithMetrics(metrics),
		wshub.WithMessageHandler(func(client *wshub.Client, msg *wshub.Message) error {
			// Echo message to all clients
			hub.BroadcastText(fmt.Sprintf("[%s]: %s", client.ID[:8], msg.Text()))
			return nil
		}),
	)

	go hub.Run()

	// WebSocket endpoint
	http.HandleFunc("/ws", hub.HandleHTTP())

	// Metrics endpoint — returns the human-readable DebugMetrics summary.
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, metrics.String())
	})

	// Metrics JSON endpoint
	http.HandleFunc("/metrics/json", func(w http.ResponseWriter, r *http.Request) {
		stats := metrics.Stats()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"active_connections":%d,"total_connections":%d,"messages_recv":%d,"messages_sent":%d,"messages_dropped":%d,"total_bytes":%d,"active_rooms":%d,"room_joins":%d,"room_leaves":%d,"avg_latency_ns":%d,"avg_broadcast_ns":%d,"uptime_s":%.0f}`,
			stats.ActiveConnections,
			stats.TotalConnections,
			stats.TotalMessagesRecv,
			stats.TotalMessagesSent,
			stats.TotalDropped,
			stats.TotalMessageBytes,
			stats.ActiveRooms,
			stats.TotalRoomJoins,
			stats.TotalRoomLeaves,
			stats.AvgLatency,
			stats.AvgBroadcast,
			stats.Uptime.Seconds(),
		)
	})

	// Simple test page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!DOCTYPE html>
<html><body>
<h1>Metrics Example</h1>
<p>Open <a href="/metrics">/metrics</a> to see stats.</p>
<pre id="log"></pre>
<input id="msg" placeholder="Type a message" />
<button onclick="send()">Send</button>
<script>
const ws = new WebSocket("ws://"+location.host+"/ws");
ws.onopen = () => log("connected");
ws.onmessage = (e) => log(e.data);
ws.onclose = () => log("disconnected");
function send() { const m = document.getElementById("msg"); ws.send(m.value); m.value = ""; }
function log(t) { document.getElementById("log").textContent += t + "\n"; }
</script>
</body></html>`)
	})

	// ReadHeaderTimeout guards against slow-header (Slowloris) connections
	// holding a goroutine open indefinitely.
	server := &http.Server{Addr: addr, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("Metrics example running on %s", addr)
		log.Println("  /ws      - WebSocket endpoint")
		log.Println("  /metrics - Metrics endpoint")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)
	hub.Shutdown(ctx)
}
