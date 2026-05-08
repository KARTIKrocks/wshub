// Command loadtest runs end-to-end load tests against a real wshub server
// with real WebSocket connections to find bottlenecks that micro-benchmarks miss.
//
// Usage:
//
//	go run ./cmd/loadtest -scenario=all -clients=5000 -duration=10s
//	go run ./cmd/loadtest -scenario=fanout -clients=10000 -duration=30s -msg-rate=500
//	go run ./cmd/loadtest -scenario=churn -clients=5000 -churn-rate=100
//	go run ./cmd/loadtest -scenario=fanout -clients=10000 -parallel=100
package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	wshub "github.com/KARTIKrocks/wshub"
	"github.com/gorilla/websocket"
)

var (
	scenario  = flag.String("scenario", "all", "scenario: connect, fanout, rooms, echo, churn, all")
	nClients  = flag.Int("clients", 1000, "number of WebSocket clients")
	dur       = flag.Duration("duration", 10*time.Second, "test duration for sustained scenarios")
	msgSize   = flag.Int("msg-size", 128, "message payload size in bytes")
	msgRate   = flag.Int("msg-rate", 100, "broadcast messages per second")
	nRooms    = flag.Int("rooms", 100, "number of rooms (rooms scenario)")
	churnRate = flag.Int("churn-rate", 50, "connects+disconnects per second (churn scenario)")
	parallel  = flag.Int("parallel", 0, "enable parallel broadcast with given batch size (0=disabled)")
)

// ---------------- main ----------------

func main() {
	flag.Parse()

	printHeader()

	scenarios := []struct {
		name string
		fn   func()
	}{
		{"connect", runConnect},
		{"fanout", runFanout},
		{"rooms", runRooms},
		{"echo", runEcho},
		{"churn", runChurn},
	}

	if *scenario == "all" {
		for _, s := range scenarios {
			s.fn()
			fmt.Println()
			runtime.GC()
			time.Sleep(500 * time.Millisecond)
		}
	} else {
		for _, s := range scenarios {
			if s.name == *scenario {
				s.fn()
				return
			}
		}
		fmt.Fprintf(os.Stderr, "unknown scenario: %s\nvalid: connect, fanout, rooms, echo, churn, all\n", *scenario)
		os.Exit(1)
	}
}

func printHeader() {
	fmt.Println("wshub load test")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("system         : %s/%s, %d cores, Go %s\n",
		runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.Version())
	if *parallel > 0 {
		fmt.Printf("broadcast      : parallel (batch=%d)\n", *parallel)
	} else {
		fmt.Printf("broadcast      : serial\n")
	}
	fmt.Printf("goroutines     : %d (baseline)\n", runtime.NumGoroutine())
	fmt.Println()
}

// ---------------- test server ----------------

type testServer struct {
	hub     *wshub.Hub
	metrics *wshub.DebugMetrics
	server  *httptest.Server
	wsURL   string
}

func newServer(opts ...wshub.Option) *testServer {
	m := wshub.NewDebugMetrics()
	defaults := []wshub.Option{
		wshub.WithMetrics(m),
		wshub.WithConfig(wshub.Config{
			CheckOrigin:     func(r *http.Request) bool { return true },
			SendChannelSize: 256,
		}),
	}
	if *parallel > 0 {
		// The -parallel flag exists to verify and compare the deprecated path.
		defaults = append(defaults, wshub.WithParallelBroadcast(*parallel)) //nolint:staticcheck // intentional: load test exercises the deprecated option
	}
	hub := wshub.NewHub(append(defaults, opts...)...)
	go hub.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.HandleHTTP())
	srv := httptest.NewServer(mux)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	return &testServer{hub: hub, metrics: m, server: srv, wsURL: wsURL}
}

func (ts *testServer) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = ts.hub.Shutdown(ctx)
	ts.server.Close()
}

// ---------------- client helpers ----------------

var dialer = &websocket.Dialer{
	HandshakeTimeout: 30 * time.Second,
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
}

// connectN opens n WebSocket connections with bounded concurrency.
// Returns the connections and the time taken.
func connectN(url string, n int) ([]*websocket.Conn, time.Duration, int64) {
	conns := make([]*websocket.Conn, n)
	var mu sync.Mutex
	var errors atomic.Int64
	sem := make(chan struct{}, 200) // max 200 concurrent handshakes
	var wg sync.WaitGroup

	start := time.Now()
	for i := range n {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			conn, _, err := dialer.Dial(url, nil)
			if err != nil {
				errors.Add(1)
				return
			}
			mu.Lock()
			conns[idx] = conn
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)
	return conns, elapsed, errors.Load()
}

func closeAll(conns []*websocket.Conn) {
	var wg sync.WaitGroup
	for _, c := range conns {
		if c == nil {
			continue
		}
		wg.Add(1)
		go func(c *websocket.Conn) {
			defer wg.Done()
			_ = c.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			_ = c.Close()
		}(c)
	}
	wg.Wait()
}

// readLoop reads messages from a connection until it's closed.
// For each message, it extracts the embedded timestamp and records the latency.
func readLoop(conn *websocket.Conn, latencies *latencyCollector, count *atomic.Int64) {
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		count.Add(1)
		if len(msg) >= 8 {
			sent := int64(binary.LittleEndian.Uint64(msg[:8]))
			lat := time.Duration(time.Now().UnixNano() - sent)
			latencies.add(lat)
		}
	}
}

// startReaders starts a readLoop goroutine for each non-nil connection.
func startReaders(conns []*websocket.Conn, lc *latencyCollector, count *atomic.Int64) {
	for _, c := range conns {
		if c != nil {
			go readLoop(c, lc, count)
		}
	}
}

// firstConn returns the first non-nil connection, or nil.
func firstConn(conns []*websocket.Conn) *websocket.Conn {
	for _, c := range conns {
		if c != nil {
			return c
		}
	}
	return nil
}

// startBroadcaster sends messages at the given rate over a single connection
// until ctx is cancelled. Returns a counter of messages sent.
func startBroadcaster(ctx context.Context, conn *websocket.Conn, rate, size int) *atomic.Int64 {
	var sentCount atomic.Int64
	go func() {
		payload := makePayload(size)
		ticker := time.NewTicker(time.Second / time.Duration(rate))
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stampPayload(payload)
				if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
					return
				}
				sentCount.Add(1)
			}
		}
	}()
	return &sentCount
}

// makePayload creates a message of the given size with a timestamp in the first 8 bytes.
func makePayload(size int) []byte {
	if size < 8 {
		size = 8
	}
	buf := make([]byte, size)
	binary.LittleEndian.PutUint64(buf[:8], uint64(time.Now().UnixNano()))
	return buf
}

// stampPayload updates the timestamp in an existing buffer.
func stampPayload(buf []byte) {
	binary.LittleEndian.PutUint64(buf[:8], uint64(time.Now().UnixNano()))
}

// ---------------- latency collector ----------------

type latencyCollector struct {
	mu      sync.Mutex
	samples []time.Duration
}

func newLatencyCollector(cap int) *latencyCollector {
	return &latencyCollector{samples: make([]time.Duration, 0, cap)}
}

func (lc *latencyCollector) add(d time.Duration) {
	lc.mu.Lock()
	lc.samples = append(lc.samples, d)
	lc.mu.Unlock()
}

func (lc *latencyCollector) percentiles() (p50, p95, p99, max time.Duration) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if len(lc.samples) == 0 {
		return
	}
	slices.Sort(lc.samples)
	n := len(lc.samples)
	p50 = lc.samples[n*50/100]
	p95 = lc.samples[n*95/100]
	p99 = lc.samples[n*99/100]
	max = lc.samples[n-1]
	return
}

// ---------------- scenario: connect ----------------

func runConnect() {
	n := *nClients
	fmt.Printf("--- CONNECT (%d clients) ---\n", n)

	ts := newServer()
	defer ts.close()

	// Wait for hub to start.
	time.Sleep(50 * time.Millisecond)

	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)
	grBefore := runtime.NumGoroutine()

	conns, elapsed, errs := connectN(ts.wsURL, n)
	connected := int64(n) - errs

	// Let goroutines settle.
	time.Sleep(200 * time.Millisecond)

	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)
	grAfter := runtime.NumGoroutine()

	stats := ts.metrics.Stats()
	heapGrowth := int64(memAfter.HeapAlloc) - int64(memBefore.HeapAlloc)
	grGrowth := grAfter - grBefore

	var memPerConn float64
	var grPerConn float64
	if connected > 0 {
		memPerConn = float64(heapGrowth) / float64(connected)
		grPerConn = float64(grGrowth) / float64(connected)
	}

	fmt.Printf("  connected      : %d / %d", connected, n)
	if errs > 0 {
		fmt.Printf(" (%d errors)", errs)
	}
	fmt.Println()
	fmt.Printf("  time           : %s\n", elapsed.Round(time.Millisecond))
	if elapsed > 0 {
		fmt.Printf("  rate           : %.0f conn/s\n", float64(connected)/elapsed.Seconds())
	}
	fmt.Printf("  mem/conn       : %s\n", formatBytes(memPerConn))
	fmt.Printf("  goroutines     : %d (+%.1f/conn)\n", grAfter, grPerConn)
	fmt.Printf("  hub active     : %d connections\n", stats.ActiveConnections)

	closeAll(conns)
}

// ---------------- scenario: fanout ----------------

func runFanout() {
	n := *nClients
	rate := *msgRate
	d := *dur
	size := *msgSize
	fmt.Printf("--- FANOUT (%d clients, %dB msg, %d msg/s, %s) ---\n", n, size, rate, d)

	var hub *wshub.Hub
	ts := newServer(wshub.WithMessageHandler(func(client *wshub.Client, msg *wshub.Message) error {
		hub.Broadcast(msg.Data)
		return nil
	}))
	hub = ts.hub
	defer ts.close()

	time.Sleep(50 * time.Millisecond)

	conns, _, errs := connectN(ts.wsURL, n)
	if errs > 0 {
		fmt.Printf("  !! %d connection errors\n", errs)
	}

	latencies := newLatencyCollector(rate * int(d.Seconds()) * n)
	var recvCount atomic.Int64
	startReaders(conns, latencies, &recvCount)

	broadcaster := firstConn(conns)
	if broadcaster == nil {
		fmt.Println("  !! no connections established")
		return
	}

	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	sentCount := startBroadcaster(ctx, broadcaster, rate, size)

	<-ctx.Done()
	time.Sleep(500 * time.Millisecond)

	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	sent := sentCount.Load()
	recv := recvCount.Load()
	p50, p95, p99, pMax := latencies.percentiles()
	stats := ts.metrics.Stats()

	fmt.Printf("  sent           : %d broadcasts\n", sent)
	fmt.Printf("  received       : %d messages\n", recv)
	if d.Seconds() > 0 {
		fmt.Printf("  throughput     : %.0f msg/s delivered\n", float64(recv)/d.Seconds())
	}
	fmt.Printf("  latency p50    : %s\n", latStr(p50))
	fmt.Printf("  latency p95    : %s\n", latStr(p95))
	fmt.Printf("  latency p99    : %s\n", latStr(p99))
	fmt.Printf("  latency max    : %s\n", latStr(pMax))
	fmt.Printf("  dropped        : %d\n", stats.TotalDropped)
	fmt.Printf("  heap alloc     : %s\n", formatBytes(float64(memAfter.HeapAlloc)))
	fmt.Printf("  GC cycles      : %d\n", memAfter.NumGC-memBefore.NumGC)

	closeAll(conns)
}

// ---------------- scenario: rooms ----------------

// assignRooms distributes hub clients across numRooms rooms and returns
// the time taken and the number of clients assigned.
func assignRooms(hub *wshub.Hub, numRooms int) (time.Duration, int) {
	clients := hub.Clients()
	start := time.Now()
	for i, c := range clients {
		_ = hub.JoinRoom(c, fmt.Sprintf("room-%d", i%numRooms))
	}
	return time.Since(start), len(clients)
}

// pickRoomSenders selects one sender connection per room from conns.
func pickRoomSenders(conns []*websocket.Conn, numRooms, perRoom int) []*websocket.Conn {
	senders := make([]*websocket.Conn, 0, numRooms)
	for i := 0; i < numRooms && i < len(conns); i++ {
		idx := i * perRoom
		if idx < len(conns) && conns[idx] != nil {
			senders = append(senders, conns[idx])
		}
	}
	return senders
}

// startMultiSenders starts a sender goroutine per connection at the given rate.
func startMultiSenders(ctx context.Context, senders []*websocket.Conn, rate, size int) *atomic.Int64 {
	var sentCount atomic.Int64
	for _, sender := range senders {
		go func(conn *websocket.Conn) {
			payload := makePayload(size)
			ticker := time.NewTicker(time.Second / time.Duration(rate))
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					stampPayload(payload)
					if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
						return
					}
					sentCount.Add(1)
				}
			}
		}(sender)
	}
	return &sentCount
}

func runRooms() {
	n := *nClients
	rooms := *nRooms
	rate := *msgRate
	d := *dur
	size := *msgSize
	perRoom := max(n/rooms, 1)
	fmt.Printf("--- ROOMS (%d clients, %d rooms, %d/room, %d msg/s, %s) ---\n",
		n, rooms, perRoom, rate, d)

	var hub *wshub.Hub
	ts := newServer(wshub.WithMessageHandler(func(client *wshub.Client, msg *wshub.Message) error {
		clientRooms := client.Rooms()
		if len(clientRooms) > 0 {
			_ = hub.BroadcastToRoom(clientRooms[0], msg.Data)
		}
		return nil
	}))
	hub = ts.hub
	defer ts.close()

	time.Sleep(50 * time.Millisecond)

	conns, _, errs := connectN(ts.wsURL, n)
	if errs > 0 {
		fmt.Printf("  !! %d connection errors\n", errs)
	}

	joinTime, nJoined := assignRooms(hub, rooms)

	latencies := newLatencyCollector(rate * int(d.Seconds()) * perRoom)
	var recvCount atomic.Int64
	startReaders(conns, latencies, &recvCount)

	senders := pickRoomSenders(conns, rooms, perRoom)
	ratePerRoom := max(rate/rooms, 1)

	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	sentCount := startMultiSenders(ctx, senders, ratePerRoom, size)

	<-ctx.Done()
	time.Sleep(500 * time.Millisecond)

	sent := sentCount.Load()
	recv := recvCount.Load()
	p50, p95, p99, pMax := latencies.percentiles()
	stats := ts.metrics.Stats()

	fmt.Printf("  room joins     : %d in %s (%.0f/s)\n",
		nJoined, joinTime.Round(time.Millisecond),
		float64(nJoined)/joinTime.Seconds())
	fmt.Printf("  sent           : %d messages\n", sent)
	fmt.Printf("  received       : %d messages\n", recv)
	if d.Seconds() > 0 {
		fmt.Printf("  throughput     : %.0f msg/s delivered\n", float64(recv)/d.Seconds())
	}
	fmt.Printf("  latency p50    : %s\n", latStr(p50))
	fmt.Printf("  latency p95    : %s\n", latStr(p95))
	fmt.Printf("  latency p99    : %s\n", latStr(p99))
	fmt.Printf("  latency max    : %s\n", latStr(pMax))
	fmt.Printf("  dropped        : %d\n", stats.TotalDropped)
	fmt.Printf("  active rooms   : %d\n", stats.ActiveRooms)

	closeAll(conns)
}

// ---------------- scenario: echo ----------------

func runEcho() {
	n := *nClients
	d := *dur
	size := *msgSize
	fmt.Printf("--- ECHO (%d clients, %dB msg, %s) ---\n", n, size, d)

	ts := newServer(wshub.WithMessageHandler(func(client *wshub.Client, msg *wshub.Message) error {
		return client.Send(msg.Data)
	}))
	defer ts.close()

	time.Sleep(50 * time.Millisecond)

	conns, _, errs := connectN(ts.wsURL, n)
	if errs > 0 {
		fmt.Printf("  !! %d connection errors\n", errs)
	}

	// Each client sends a message, waits for echo, records RTT.
	latencies := newLatencyCollector(n * int(d.Seconds()))
	var roundTrips atomic.Int64
	var sendErrors atomic.Int64

	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	// Distribute sending across clients. Each client sends once per second.
	var wg sync.WaitGroup
	for _, conn := range conns {
		if conn == nil {
			continue
		}
		wg.Add(1)
		go func(c *websocket.Conn) {
			defer wg.Done()
			payload := makePayload(size)
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				stampPayload(payload)
				sendTime := time.Now()
				if err := c.WriteMessage(websocket.TextMessage, payload); err != nil {
					sendErrors.Add(1)
					return
				}

				_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
				_, _, err := c.ReadMessage()
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					sendErrors.Add(1)
					return
				}

				rtt := time.Since(sendTime)
				latencies.add(rtt)
				roundTrips.Add(1)
			}
		}(conn)
	}

	<-ctx.Done()
	// Give a moment for in-flight messages.
	time.Sleep(200 * time.Millisecond)
	cancel()
	wg.Wait()

	trips := roundTrips.Load()
	p50, p95, p99, max := latencies.percentiles()

	fmt.Printf("  round-trips    : %d\n", trips)
	if d.Seconds() > 0 {
		fmt.Printf("  throughput     : %.0f echo/s\n", float64(trips)/d.Seconds())
	}
	fmt.Printf("  RTT p50        : %s\n", latStr(p50))
	fmt.Printf("  RTT p95        : %s\n", latStr(p95))
	fmt.Printf("  RTT p99        : %s\n", latStr(p99))
	fmt.Printf("  RTT max        : %s\n", latStr(max))
	if e := sendErrors.Load(); e > 0 {
		fmt.Printf("  send errors    : %d\n", e)
	}

	closeAll(conns)
}

// ---------------- scenario: churn ----------------

// churnStats tracks connection churn metrics.
type churnStats struct {
	connects    atomic.Int64
	disconnects atomic.Int64
	errors      atomic.Int64
}

// startChurner spawns ephemeral connections at the given rate: connect, read
// one message, then disconnect.
func startChurner(ctx context.Context, wsURL string, rate int) *churnStats {
	cs := &churnStats{}
	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(rate))
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				go func() {
					conn, _, err := dialer.Dial(wsURL, nil)
					if err != nil {
						cs.errors.Add(1)
						return
					}
					cs.connects.Add(1)

					_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
					_, _, _ = conn.ReadMessage()

					_ = conn.WriteMessage(websocket.CloseMessage,
						websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
					_ = conn.Close()
					cs.disconnects.Add(1)
				}()
			}
		}
	}()
	return cs
}

func runChurn() {
	n := *nClients
	rate := *msgRate
	cRate := *churnRate
	d := *dur
	size := *msgSize
	fmt.Printf("--- CHURN (%d base clients, %d churn/s, %d msg/s, %s) ---\n",
		n, cRate, rate, d)

	var hub *wshub.Hub
	ts := newServer(wshub.WithMessageHandler(func(client *wshub.Client, msg *wshub.Message) error {
		hub.Broadcast(msg.Data)
		return nil
	}))
	hub = ts.hub
	defer ts.close()

	time.Sleep(50 * time.Millisecond)

	conns, _, errs := connectN(ts.wsURL, n)
	if errs > 0 {
		fmt.Printf("  !! %d connection errors\n", errs)
	}

	latencies := newLatencyCollector(rate * int(d.Seconds()) * n)
	var recvCount atomic.Int64
	startReaders(conns, latencies, &recvCount)

	broadcaster := firstConn(conns)
	if broadcaster == nil {
		fmt.Println("  !! no connections established")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	sentCount := startBroadcaster(ctx, broadcaster, rate, size)
	cs := startChurner(ctx, ts.wsURL, cRate)

	<-ctx.Done()
	time.Sleep(500 * time.Millisecond)

	sent := sentCount.Load()
	recv := recvCount.Load()
	p50, p95, p99, pMax := latencies.percentiles()
	stats := ts.metrics.Stats()

	fmt.Printf("  broadcasts     : %d\n", sent)
	fmt.Printf("  received       : %d messages\n", recv)
	if d.Seconds() > 0 {
		fmt.Printf("  throughput     : %.0f msg/s delivered\n", float64(recv)/d.Seconds())
	}
	fmt.Printf("  latency p50    : %s\n", latStr(p50))
	fmt.Printf("  latency p95    : %s\n", latStr(p95))
	fmt.Printf("  latency p99    : %s\n", latStr(p99))
	fmt.Printf("  latency max    : %s\n", latStr(pMax))
	fmt.Printf("  churn          : %d connects, %d disconnects\n",
		cs.connects.Load(), cs.disconnects.Load())
	if ce := cs.errors.Load(); ce > 0 {
		fmt.Printf("  churn errors   : %d\n", ce)
	}
	fmt.Printf("  dropped        : %d\n", stats.TotalDropped)
	fmt.Printf("  total conns    : %d\n", stats.TotalConnections)

	closeAll(conns)
}

// ---------------- formatting helpers ----------------

func latStr(d time.Duration) string {
	if d == 0 {
		return "n/a"
	}
	us := float64(d) / float64(time.Microsecond)
	if us < 1000 {
		return fmt.Sprintf("%.0fus", us)
	}
	ms := float64(d) / float64(time.Millisecond)
	if ms < 1000 {
		return fmt.Sprintf("%.2fms", ms)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func formatBytes(b float64) string {
	abs := math.Abs(b)
	switch {
	case abs >= 1<<30:
		return fmt.Sprintf("%.2f GB", b/float64(int64(1)<<30))
	case abs >= 1<<20:
		return fmt.Sprintf("%.2f MB", b/float64(int64(1)<<20))
	case abs >= 1<<10:
		return fmt.Sprintf("%.2f KB", b/float64(int64(1)<<10))
	default:
		return fmt.Sprintf("%.0f B", b)
	}
}
