// Example: Multi-node WebSocket hub with Redis adapter.
//
// This demonstrates horizontal scaling by running two hub instances on
// different ports, connected through Redis Pub/Sub. A message sent to
// one node is automatically relayed to clients on the other.
//
// Prerequisites:
//
//	Redis running on localhost:6379
//
// Usage:
//
//	go run ./examples/multinode
//
// Then open two browser tabs:
//
//	Tab 1 → http://localhost:8081   (connects to Node A)
//	Tab 2 → http://localhost:8082   (connects to Node B)
//
// Send a message from either tab — both tabs receive it.
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
	wshubredis "github.com/KARTIKrocks/wshub/adapter/redis"
	goredis "github.com/redis/go-redis/v9"
)

func main() {
	// Connect to Redis.
	redisAddr := "localhost:6379"
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		redisAddr = addr
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: redisAddr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis: %v (set REDIS_ADDR to override)", err)
	}
	defer rdb.Close()

	// Create two adapters on the same Redis instance.
	adapterA := wshubredis.New(rdb)
	adapterB := wshubredis.New(rdb)

	// Create hub A on :8081.
	hubA := newNode("node-A", ":8081", adapterA)

	// Create hub B on :8082.
	hubB := newNode("node-B", ":8082", adapterB)

	go hubA.hub.Run()
	go hubB.hub.Run()

	serverA := startHTTP(hubA)
	serverB := startHTTP(hubB)

	log.Println("Multi-node example running:")
	log.Println("  Node A → http://localhost:8081")
	log.Println("  Node B → http://localhost:8082")
	log.Println()
	log.Println("Open both URLs, send a message from one — it appears on both.")

	// Wait for interrupt.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	serverA.Shutdown(ctx)
	serverB.Shutdown(ctx)
	hubA.hub.Shutdown(ctx)
	hubB.hub.Shutdown(ctx)
}

type node struct {
	hub  *wshub.Hub
	addr string
}

func newNode(id, addr string, adapter wshub.Adapter) *node {
	var hub *wshub.Hub
	hub = wshub.NewHub(
		wshub.WithAdapter(adapter),
		wshub.WithNodeID(id),
		wshub.WithPresence(5*time.Second),
		wshub.WithMessageHandler(func(client *wshub.Client, msg *wshub.Message) error {
			text := fmt.Sprintf("[%s@%s]: %s", client.ID[:8], id, msg.Text())
			hub.BroadcastText(text)
			return nil
		}),
	)
	return &node{hub: hub, addr: addr}
}

func startHTTP(n *node) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", n.hub.HandleHTTP())
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "node: %s\nlocal clients: %d\nglobal clients: %d\n",
			n.hub.NodeID(), n.hub.ClientCount(), n.hub.GlobalClientCount())
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, page, n.hub.NodeID(), n.addr)
	})

	server := &http.Server{Addr: n.addr, Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http %s: %v", n.addr, err)
		}
	}()
	return server
}

const page = `<!DOCTYPE html>
<html><body>
<h1>wshub — Multi-Node Demo</h1>
<p>Node: <strong>%s</strong> | Port: <strong>%s</strong></p>
<p>Open the other node's URL in another tab and send messages.</p>
<pre id="log" style="background:#111;color:#0f0;padding:1em;height:300px;overflow:auto"></pre>
<input id="msg" placeholder="Type a message" style="width:300px" />
<button onclick="send()">Send</button>
<p><a href="/stats">/stats</a> — node stats</p>
<script>
const ws = new WebSocket("ws://"+location.host+"/ws");
ws.onopen = () => log("connected");
ws.onmessage = (e) => log(e.data);
ws.onclose = () => log("disconnected");
function send() { const m = document.getElementById("msg"); ws.send(m.value); m.value = ""; }
function log(t) { const el = document.getElementById("log"); el.textContent += t + "\n"; el.scrollTop = el.scrollHeight; }
document.getElementById("msg").addEventListener("keydown", (e) => { if (e.key === "Enter") send(); });
</script>
</body></html>`
