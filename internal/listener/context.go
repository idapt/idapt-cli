package listener

import "context"

type contextKey string

const listenerPortKey contextKey = "listenerPort"

// WithListenerPort returns a context with the listener port value attached.
// Used by dynamic TLS listeners to propagate the port to the proxy and auth middleware.
func WithListenerPort(ctx context.Context, port int) context.Context {
	return context.WithValue(ctx, listenerPortKey, port)
}

// ListenerPortFromContext extracts the listener port from a context.
// Returns 0 if no port was set (request came via the main :443 listener).
func ListenerPortFromContext(ctx context.Context) int {
	port, _ := ctx.Value(listenerPortKey).(int)
	return port
}
