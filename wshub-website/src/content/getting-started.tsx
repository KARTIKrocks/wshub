import CodeBlock from '../components/CodeBlock';

export default function GettingStarted() {
  return (
    <section id="getting-started" className="py-10 border-b border-border">
      <h2 className="text-2xl font-bold text-text-heading mb-4">Getting Started</h2>

      <h3 className="text-lg font-semibold text-text-heading mt-6 mb-2">Installation</h3>
      <p className="text-text-muted mb-3">Requires <strong>Go 1.21+</strong>.</p>
      <CodeBlock lang="bash" code={`go get github.com/KARTIKrocks/wshub`} />

      <h3 className="text-lg font-semibold text-text-heading mt-8 mb-2">Quick Start</h3>
      <p className="text-text-muted mb-3">
        A minimal WebSocket server with echo handler and graceful shutdown:
      </p>
      <CodeBlock code={`package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/KARTIKrocks/wshub"
)

func main() {
    hub := wshub.NewHub(
        wshub.WithMessageHandler(func(client *wshub.Client, msg *wshub.Message) error {
            log.Printf("Message from %s: %s", client.ID, msg.Text())
            return client.Send(msg.Data) // echo back
        }),
        wshub.WithHooks(wshub.Hooks{
            AfterConnect: func(client *wshub.Client) {
                log.Printf("Client connected: %s", client.ID)
            },
            AfterDisconnect: func(client *wshub.Client) {
                log.Printf("Client disconnected: %s", client.ID)
            },
        }),
    )

    go hub.Run()

    http.HandleFunc("/ws", hub.HandleHTTP())

    srv := &http.Server{Addr: ":8080"}
    go func() {
        log.Println("Listening on :8080")
        if err := srv.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatal(err)
        }
    }()

    // Graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    hub.Shutdown(ctx)
    srv.Shutdown(ctx)
}`} />
    </section>
  );
}
