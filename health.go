package wshub

import (
	"fmt"
	"net/http"
	"time"
)

// HealthStatus is a point-in-time snapshot of hub health, suitable for
// liveness and readiness probes.
type HealthStatus struct {
	// Alive is true when the Run() goroutine is executing.
	Alive bool

	// Ready is true when the hub is alive and accepting new connections.
	Ready bool

	// State is the hub's lifecycle state as a human-readable string
	// ("running", "draining", or "stopped").
	State string

	// Uptime is how long the Run() goroutine has been executing.
	// Zero if Run() has not been called.
	Uptime time.Duration

	// Clients is the current number of connected clients.
	Clients int
}

// Alive reports whether the [Hub.Run] goroutine is currently executing.
// Returns false before Run is called or after it exits.
// The read is a single atomic load — safe for concurrent use on hot paths.
func (h *Hub) Alive() bool {
	return h.alive.Load() == 1
}

// Ready reports whether the hub is alive and in the [StateRunning] state,
// meaning it can accept and process new connections. Returns false before
// [Hub.Run] is called, while draining, or after shutdown.
func (h *Hub) Ready() bool {
	return h.Alive() && h.State() == StateRunning
}

// Uptime returns how long the [Hub.Run] goroutine has been executing.
// Returns zero if Run has not been called or has already exited.
func (h *Hub) Uptime() time.Duration {
	if !h.Alive() {
		return 0
	}
	t, ok := h.startedAt.Load().(time.Time)
	if !ok || t.IsZero() {
		return 0
	}
	return time.Since(t)
}

// Health returns a point-in-time [HealthStatus] snapshot.
// All reads are lock-free atomic loads.
func (h *Hub) Health() HealthStatus {
	return HealthStatus{
		Alive:   h.Alive(),
		Ready:   h.Ready(),
		State:   h.State().String(),
		Uptime:  h.Uptime(),
		Clients: h.ClientCount(),
	}
}

// HealthHandler returns an HTTP handler that reports the hub's liveness
// status. Returns 200 OK when the [Hub.Run] goroutine is alive,
// 503 Service Unavailable otherwise. The response body is a JSON object
// with the same fields as [HealthStatus].
//
// Typical usage with Kubernetes:
//
//	http.Handle("/healthz", hub.HealthHandler())
func (h *Hub) HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hs := h.Health()
		w.Header().Set("Content-Type", "application/json")
		if !hs.Alive {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_, _ = fmt.Fprintf(w, `{"alive":%t,"ready":%t,"state":%q,"uptime_ns":%d,"clients":%d}`,
			hs.Alive, hs.Ready, hs.State, hs.Uptime, hs.Clients)
	}
}

// ReadyHandler returns an HTTP handler that reports the hub's readiness
// status. Returns 200 OK when the hub is alive and in [StateRunning]
// (accepting connections), 503 Service Unavailable otherwise.
//
// Typical usage with Kubernetes:
//
//	http.Handle("/readyz", hub.ReadyHandler())
func (h *Hub) ReadyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hs := h.Health()
		w.Header().Set("Content-Type", "application/json")
		if !hs.Ready {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_, _ = fmt.Fprintf(w, `{"alive":%t,"ready":%t,"state":%q,"uptime_ns":%d,"clients":%d}`,
			hs.Alive, hs.Ready, hs.State, hs.Uptime, hs.Clients)
	}
}
