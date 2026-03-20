package wshub_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/KARTIKrocks/wshub"
)

func ExampleNewHub() {
	hub := wshub.NewHub()
	fmt.Println("clients:", hub.ClientCount())
	// Output:
	// clients: 0
}

func ExampleNewHub_withOptions() {
	hub := wshub.NewHub(
		wshub.WithConfig(wshub.DefaultConfig().WithMaxMessageSize(4096)),
		wshub.WithLimits(wshub.DefaultLimits().WithMaxConnections(1000)),
		wshub.WithLogger(&wshub.NoOpLogger{}),
		wshub.WithMetrics(wshub.NewDebugMetrics()),
	)
	fmt.Println("clients:", hub.ClientCount())
	// Output:
	// clients: 0
}

func ExampleHub_HandleHTTP() {
	hub := wshub.NewHub()

	handler := hub.HandleHTTP()
	fmt.Println("handler is nil:", handler == nil)
	// Output:
	// handler is nil: false
}

func ExampleNewMessage() {
	msg := wshub.NewMessage([]byte("hello world"))
	fmt.Println("text:", msg.Text())
	fmt.Println("type:", msg.Type)
	// Output:
	// text: hello world
	// type: 1
}

func ExampleNewJSONMessage() {
	payload := map[string]string{"event": "chat", "text": "hello"}
	msg, err := wshub.NewJSONMessage(payload)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("has data:", len(msg.Data) > 0)

	var decoded map[string]string
	msg.JSON(&decoded)
	fmt.Println("event:", decoded["event"])
	// Output:
	// has data: true
	// event: chat
}

func ExampleNewRouter() {
	router := wshub.NewRouter(func(m *wshub.Message) string {
		var envelope struct {
			Event string `json:"event"`
		}
		json.Unmarshal(m.Data, &envelope)
		return envelope.Event
	})

	router.On("ping", func(c *wshub.Client, m *wshub.Message) error {
		return c.SendText("pong")
	})

	router.OnNotFound(func(c *wshub.Client, m *wshub.Message) error {
		return c.SendText("unknown event")
	})

	fmt.Println("router created")
	// Output:
	// router created
}

func ExampleNewMiddlewareChain() {
	handler := func(c *wshub.Client, m *wshub.Message) error {
		return nil
	}

	chain := wshub.NewMiddlewareChain(handler).
		Use(wshub.RecoveryMiddleware(&wshub.NoOpLogger{})).
		Build()

	fmt.Println("chain built:", chain != nil)
	// Output:
	// chain built: true
}

func ExampleDefaultConfig() {
	config := wshub.DefaultConfig()
	fmt.Println("compression:", config.EnableCompression)

	custom := config.
		WithMaxMessageSize(1024).
		WithCompression(true)
	fmt.Println("custom compression:", custom.EnableCompression)
	// Output:
	// compression: false
	// custom compression: true
}

func ExampleDefaultLimits() {
	limits := wshub.DefaultLimits()
	fmt.Println("max connections:", limits.MaxConnections)

	custom := limits.
		WithMaxConnections(5000).
		WithMaxRoomsPerClient(10)
	fmt.Println("custom max connections:", custom.MaxConnections)
	fmt.Println("custom max rooms:", custom.MaxRoomsPerClient)
	// Output:
	// max connections: 0
	// custom max connections: 5000
	// custom max rooms: 10
}

func ExampleNewDebugMetrics() {
	metrics := wshub.NewDebugMetrics()
	metrics.IncrementConnections()
	metrics.IncrementMessages()
	metrics.RecordMessageSize(128)

	stats := metrics.Stats()
	fmt.Println("connections:", stats.ActiveConnections)
	fmt.Println("messages:", stats.TotalMessages)
	// Output:
	// connections: 1
	// messages: 1
}

func ExampleWithHooks() {
	hub := wshub.NewHub(
		wshub.WithHooks(wshub.Hooks{
			BeforeConnect: func(r *http.Request) error {
				token := r.URL.Query().Get("token")
				if token == "" {
					return fmt.Errorf("missing token")
				}
				return nil
			},
			AfterConnect: func(c *wshub.Client) {
				fmt.Println("client connected:", c.ID)
			},
		}),
	)
	fmt.Println("hub with hooks:", hub.ClientCount())
	// Output:
	// hub with hooks: 0
}

func ExampleWithParallelBroadcast() {
	hub := wshub.NewHub(
		wshub.WithParallelBroadcast(100),
	)
	fmt.Println("parallel hub created:", hub.ClientCount())
	// Output:
	// parallel hub created: 0
}
