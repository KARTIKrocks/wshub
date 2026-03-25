import CodeBlock from '../components/CodeBlock';
import ModuleSection from '../components/ModuleSection';

export default function PresenceDocs() {
  return (
    <ModuleSection
      id="presence"
      title="Presence"
      description="Periodic presence gossip for cluster-wide client and room counts in multi-node deployments."
      importPath="github.com/KARTIKrocks/wshub"
      features={[
        'Periodic heartbeat publishing of local client/room counts',
        'Cluster-wide totals via GlobalClientCount and GlobalRoomCount',
        'Automatic eviction of stale nodes (3 missed heartbeats)',
        'O(1) change detection avoids re-gathering unchanged stats',
        'Requires an adapter — no-op in single-node mode',
      ]}
    >
      {/* ── Enabling Presence ── */}
      <h3 id="presence-enabling" className="text-lg font-semibold text-text-heading mt-8 mb-2">Enabling Presence</h3>
      <p className="text-text-muted mb-3">
        Enable presence with <code className="text-accent">WithPresence</code> alongside an adapter.
        Each hub publishes its local stats at the given interval:
      </p>
      <CodeBlock code={`hub := wshub.NewHub(
    wshub.WithAdapter(adapter),       // required for presence
    wshub.WithPresence(5*time.Second), // heartbeat interval (default: 5s)
    wshub.WithNodeID("node-1"),        // stable ID for debugging
)
go hub.Run()`} />
      <p className="text-text-muted mt-3">
        When the interval is zero, the default of 5 seconds is used. Nodes that miss 3
        consecutive heartbeats are automatically evicted from the totals.
      </p>

      {/* ── Global Counts ── */}
      <h3 id="presence-global" className="text-lg font-semibold text-text-heading mt-8 mb-2">Global Counts</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Method</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">GlobalClientCount()</td><td className="py-2 text-text-muted">Total connected clients across all nodes</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">GlobalRoomCount(room)</td><td className="py-2 text-text-muted">Total clients in a room across all nodes</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`// Get cluster-wide totals
totalClients := hub.GlobalClientCount()
totalInRoom := hub.GlobalRoomCount("chat-general")

// In single-node mode (no adapter/presence), these return local counts
localClients := hub.ClientCount()       // always local
globalClients := hub.GlobalClientCount() // local + remote when presence enabled`} />

      {/* ── Full Example ── */}
      <h3 id="presence-example" className="text-lg font-semibold text-text-heading mt-8 mb-2">Full Example</h3>
      <p className="text-text-muted mb-3">
        A multi-node setup with Redis adapter and presence:
      </p>
      <CodeBlock code={`package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/KARTIKrocks/wshub"
    wshubredis "github.com/KARTIKrocks/wshub/adapter/redis"
    goredis "github.com/redis/go-redis/v9"
)

func main() {
    rdb := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
    adapter := wshubredis.New(rdb)

    nodeID, _ := os.Hostname()
    hub := wshub.NewHub(
        wshub.WithAdapter(adapter),
        wshub.WithPresence(5*time.Second),
        wshub.WithNodeID(nodeID),
        wshub.WithMessageHandler(func(c *wshub.Client, msg *wshub.Message) error {
            hub.Broadcast(msg.Data)
            return nil
        }),
    )
    go hub.Run()

    // Expose cluster stats
    http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "local: %d, global: %d\\n",
            hub.ClientCount(), hub.GlobalClientCount())
    })
    http.HandleFunc("/ws", hub.HandleHTTP())

    srv := &http.Server{Addr: ":8080"}
    go srv.ListenAndServe()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    hub.Shutdown(ctx)
    srv.Shutdown(ctx)
}`} />
    </ModuleSection>
  );
}
