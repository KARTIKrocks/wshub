package wshub

import (
	"testing"
	"time"
)

// waitHubReady blocks until the hub will accept upgrades, or fails the test.
//
// Run() sets the alive flag from inside its own goroutine, and
// UpgradeConnection rejects with 503 until it does. A test that starts an
// httptest server immediately after `go hub.Run()` is racing that goroutine,
// and loses it often enough under load to matter: the dial then fails with
// "websocket: bad handshake", and a test asserting on rejection counts sees
// connection_rejected_not_ready instead of the metric it expected, leaving it
// one short.
//
// Call this between starting the hub and serving, not before dialling — the
// point is that the server must not be reachable until the hub behind it is
// ready. Tests that deliberately exercise the pre-Run path
// (TestUpgradeConnection_RejectsBeforeRun) must not call it.
func waitHubReady(t *testing.T, hub *Hub) {
	t.Helper()

	// Ready() is Alive() && StateRunning, which is exactly the pair of guards
	// UpgradeConnection checks before upgrading.
	deadline := time.Now().Add(2 * time.Second)
	for !hub.Ready() {
		if time.Now().After(deadline) {
			t.Fatal("hub did not become ready within 2s of Run()")
		}
		time.Sleep(time.Millisecond)
	}
}
