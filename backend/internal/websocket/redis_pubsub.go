package websocket

import (
    "context"
    "encoding/json"
    "log"

    "github.com/redis/go-redis/v9"
)

type RedisPubSub struct {
    Client *redis.Client
    Hub   *Hub
}

func NewRedisPubSub(client *redis.Client, hub *Hub) *RedisPubSub {
    return &RedisPubSub{
        Client: client,
        Hub:   hub,
    }
}

func (r *RedisPubSub) Subscribe(ctx context.Context) {
    pubsub := r.Client.Subscribe(
        ctx,
        "sarathi:ws:location",
        "sarathi:ws:booking",
        "sarathi:ws:dispatch",
        "sarathi:ws:shared",
        "sarathi:ws:bus",
        "sarathi:ws:commuter",
        "sarathi:ws:station",
        "sarathi:ws:notification",
    )

    defer pubsub.Close()

    _, err := pubsub.Receive(ctx)
    if err != nil {
        log.Printf("Redis subscription failed: %v", err)
        return
    }

    channel := pubsub.Channel()

    for {
        select {
        case <-ctx.Done():
            return

        case msg, ok := <-channel:
            if !ok {
                return
            }

            r.routeEvent(msg.Channel, []byte(msg.Payload))
        }
    }
}

func (r *RedisPubSub) routeEvent(channel string, payload []byte) {
    var event Message

    if err := json.Unmarshal(payload, &event); err != nil {
        log.Printf("Invalid Redis event: %v", err)
        return
    }

    switch channel {

    case "sarathi:ws:location",
         "sarathi:ws:booking",
         "sarathi:ws:shared",
         "sarathi:ws:bus":

        if event.RideID != "" {
            roomID := "ride:" + event.RideID
            r.Hub.SendToRoom(roomID, payload)
        }

    case "sarathi:ws:dispatch",
         "sarathi:ws:notification":

        if event.UserID != "" {
            r.Hub.SendToUser(event.UserID, payload)
        }

    case "sarathi:ws:commuter",
         "sarathi:ws:station":

        if event.RoomID != "" {
            r.Hub.SendToRoom(event.RoomID, payload)
        }
    }
}