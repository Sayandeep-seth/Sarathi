package websocket

type Room struct {
    ID      string
    Clients  map[*Client]bool
}

func NewRoom(id string) *Room {
    return &Room{
        ID:      id,
        Clients: make(map[*Client]bool),
    }
}

func (r *Room) AddClient(client *Client) {
    r.Clients[client] = true
}

func (r *Room) RemoveClient(client *Client) {
    delete(r.Clients, client)
}