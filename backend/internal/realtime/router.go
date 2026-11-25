package realtime

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// HandlerFunc handles one inbound client message. Return nil for success
// (the router sends an ack if the client supplied a ref), a *ClientError for
// a problem the client caused, or any other error for an internal failure.
type HandlerFunc func(ctx context.Context, c *Client, env Envelope) error

// Router maps message types to handlers. subscribe / unsubscribe / ping are
// built in; feature packages register the rest with On().
type Router struct {
	hub      *Hub
	authz    *Authorizer
	handlers map[string]HandlerFunc
	log      *slog.Logger
}

func NewRouter(hub *Hub, authz *Authorizer, log *slog.Logger) *Router {
	return &Router{hub: hub, authz: authz, handlers: make(map[string]HandlerFunc), log: log.With("component", "router")}
}

// On registers a handler. Call it at startup only (not concurrency-safe).
func (r *Router) On(msgType string, h HandlerFunc) { r.handlers[msgType] = h }

func (r *Router) Handle(ctx context.Context, c *Client, env Envelope) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var err error
	switch env.Type {
	case TypePing:
		// App-level ping: browsers cannot see protocol pings, so the client
		// sends {"type":"ping"} to measure latency / detect a dead socket.
		c.SendEnvelope(Envelope{Type: TypePong, Ref: env.Ref})
		return
	case TypeSubscribe:
		err = r.handleSubscribe(ctx, c, env)
	case TypeUnsubscribe:
		r.hub.unsubscribe(c, env.Topic)
	default:
		h, ok := r.handlers[env.Type]
		if !ok {
			c.sendError(env.Ref, "unknown_type", "unknown message type "+env.Type)
			return
		}
		err = h(ctx, c, env)
	}

	if err == nil {
		c.sendAck(env.Ref)
		return
	}
	var ce *ClientError
	if errors.As(err, &ce) {
		c.sendError(env.Ref, ce.Code, ce.Msg)
		return
	}
	c.log.Error("handler failed", "type", env.Type, "err", err)
	c.sendError(env.Ref, "internal", "something went wrong, please retry")
}

func (r *Router) handleSubscribe(ctx context.Context, c *Client, env Envelope) error {
	ok, err := r.authz.CanSubscribe(ctx, c.ID, env.Topic)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return r.hub.subscribe(c, env.Topic)
}
