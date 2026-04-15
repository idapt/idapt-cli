package listener

import "context"

type contextKey string

const listenerPortKey contextKey = "listenerPort"

func WithListenerPort(ctx context.Context, port int) context.Context {
	return context.WithValue(ctx, listenerPortKey, port)
}

func ListenerPortFromContext(ctx context.Context) int {
	port, _ := ctx.Value(listenerPortKey).(int)
	return port
}
