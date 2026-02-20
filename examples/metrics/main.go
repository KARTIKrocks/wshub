// Example: Prometheus-style metrics with wshub.
//
// This demonstrates implementing the MetricsCollector interface and exposing
// metrics on a /metrics endpoint. Uses wshub's built-in DebugMetrics for
// simplicity — in production, replace with a real Prometheus collector.
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
		fmt.Fprintf(w, `{"active_connections":%d,"total_connections":%d,"total_messages":%d,"total_bytes":%d,"room_joins":%d,"room_leaves":%d,"avg_latency_ns":%d,"uptime_s":%.0f}`,
			stats.ActiveConnections,
			stats.TotalConnections,
			stats.TotalMessages,
			stats.TotalMessageBytes,
			stats.TotalRoomJoins,
			stats.TotalRoomLeaves,
			stats.AvgLatency,
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

	server := &http.Server{Addr: ":8080"}
	go func() {
		log.Println("Metrics example running on :8080")
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
