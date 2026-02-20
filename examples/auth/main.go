// Example: JWT authentication with wshub.
//
// This demonstrates using BeforeConnect to validate a token from the
// query string and AfterConnect to set the user ID on the client.
package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	wshub "github.com/KARTIKrocks/wshub"
)

// jwtSecret is the HMAC-SHA256 signing key (in production, load from env).
var jwtSecret = []byte("super-secret-key-change-me")

// claims is a minimal JWT payload.
type claims struct {
	Sub string `json:"sub"` // user ID
	Exp int64  `json:"exp"` // expiry (unix)
}

// validateToken performs a basic HMAC-SHA256 JWT validation.
// In production you would use a proper JWT library.
func validateToken(tokenStr string) (*claims, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return nil, errors.New("malformed token")
	}

	// Verify signature
	mac := hmac.New(sha256.New, jwtSecret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, errors.New("invalid signature")
	}

	// Decode payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	var c claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	if time.Now().Unix() > c.Exp {
		return nil, errors.New("token expired")
	}

	return &c, nil
}

// generateToken creates a signed token for testing.
func generateToken(userID string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

	c := claims{
		Sub: userID,
		Exp: time.Now().Add(time.Hour).Unix(),
	}
	payload, _ := json.Marshal(c)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)

	mac := hmac.New(sha256.New, jwtSecret)
	mac.Write([]byte(header + "." + encodedPayload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return header + "." + encodedPayload + "." + sig
}

// authMiddleware rejects messages from clients without a user ID.
func authMiddleware() wshub.Middleware {
	return func(next wshub.HandlerFunc) wshub.HandlerFunc {
		return func(client *wshub.Client, msg *wshub.Message) error {
			if client.GetUserID() == "" {
				return wshub.ErrUnauthorized
			}
			return next(client, msg)
		}
	}
}

func main() {
	var hub *wshub.Hub
	hub = wshub.NewHub(
		wshub.WithHooks(wshub.Hooks{
			// Validate the token before upgrading the connection.
			BeforeConnect: func(r *http.Request) error {
				token := r.URL.Query().Get("token")
				if token == "" {
					return wshub.ErrAuthenticationFailed
				}
				if _, err := validateToken(token); err != nil {
					log.Printf("auth failed: %v", err)
					return wshub.ErrAuthenticationFailed
				}
				return nil
			},

			// After the connection is established, extract the user ID
			// from the token and set it on the client.
			AfterConnect: func(client *wshub.Client) {
				token := client.Request().URL.Query().Get("token")
				c, _ := validateToken(token) // already validated in BeforeConnect
				if c != nil {
					client.SetUserID(c.Sub)
					log.Printf("user %s connected (client %s)", c.Sub, client.ID)
				}
			},
		}),

		wshub.WithMessageHandler(
			wshub.NewMiddlewareChain(func(client *wshub.Client, msg *wshub.Message) error {
				log.Printf("[%s] says: %s", client.GetUserID(), msg.Text())
				hub.BroadcastText(fmt.Sprintf("[%s]: %s", client.GetUserID(), msg.Text()))
				return nil
			}).
				Use(authMiddleware()).
				Build().
				Execute,
		),
	)

	go hub.Run()

	// Serve a test page that generates a token and connects.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		token := generateToken("demo-user")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html><body>
<h1>Auth Example</h1>
<p>Token: <code>%s</code></p>
<pre id="log"></pre>
<script>
const ws = new WebSocket("ws://"+location.host+"/ws?token=%s");
ws.onopen = () => { document.getElementById("log").textContent += "connected\n"; ws.send("hello!"); };
ws.onmessage = (e) => { document.getElementById("log").textContent += e.data + "\n"; };
ws.onclose = () => { document.getElementById("log").textContent += "disconnected\n"; };
</script>
</body></html>`, token, token)
	})
	http.HandleFunc("/ws", hub.HandleHTTP())

	server := &http.Server{Addr: ":8080"}
	go func() {
		log.Println("Auth example running on :8080")
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
