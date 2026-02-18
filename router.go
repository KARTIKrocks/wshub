package wshub

import "sync"

// Router dispatches incoming messages to per-event handlers based on an
// event name extracted from each message by a user-provided extractor function.
//
// The extractor decouples the router from any specific message format — JSON,
// msgpack, binary with a leading byte, or anything else.
//
// Usage:
//
//	router := wshub.NewRouter(func(msg *wshub.Message) string {
//	    var env struct{ Type string `json:"type"` }
//	    json.Unmarshal(msg.Data, &env)
//	    return env.Type
//	})
//
//	router.
//	    On("chat",  handleChat).
//	    On("join",  handleJoin).
//	    On("leave", handleLeave)
//
//	hub := wshub.NewHub(wshub.WithMessageHandler(router.Handle))
//
// All On/OnNotFound calls should be made before the hub starts running.
type Router struct {
	mu        sync.RWMutex
	extractor func(*Message) string
	handlers  map[string]HandlerFunc
	notFound  HandlerFunc
}

// NewRouter creates a Router. The extractor is called on every incoming
// message to determine which registered handler to dispatch to.
func NewRouter(extractor func(*Message) string) *Router {
	return &Router{
		extractor: extractor,
		handlers:  make(map[string]HandlerFunc),
	}
}

// On registers a handler for the given event name.
// Returns the router for chaining.
func (r *Router) On(event string, handler HandlerFunc) *Router {
	r.mu.Lock()
	r.handlers[event] = handler
	r.mu.Unlock()
	return r
}

// OnNotFound sets a fallback handler called when the extracted event name
// has no registered handler. If not set, unmatched events return ErrInvalidMessage.
func (r *Router) OnNotFound(handler HandlerFunc) *Router {
	r.mu.Lock()
	r.notFound = handler
	r.mu.Unlock()
	return r
}

// Handle dispatches the message to the appropriate handler.
// Pass this method to WithMessageHandler or use it as a HandlerFunc directly.
func (r *Router) Handle(client *Client, msg *Message) error {
	event := r.extractor(msg)

	r.mu.RLock()
	handler, ok := r.handlers[event]
	notFound := r.notFound
	r.mu.RUnlock()

	if !ok {
		if notFound != nil {
			return notFound(client, msg)
		}
		return ErrInvalidMessage
	}

	return handler(client, msg)
}
