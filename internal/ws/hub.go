package ws

import "sync"

type Hub struct {
	mu      sync.Mutex
	clients map[string]*Client
	rooms   map[string]map[string]*Client
}

func NewHub() *Hub {
	return &Hub{clients: map[string]*Client{}, rooms: map[string]map[string]*Client{}}
}

func (h *Hub) Add(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c.ID] = c
}

func (h *Hub) Remove(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for room := range c.Rooms {
		if h.rooms[room] != nil {
			delete(h.rooms[room], c.ID)
		}
	}
	delete(h.clients, c.ID)
}

func (h *Hub) Join(c *Client, room string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.Rooms[room] {
		return false
	}
	c.Rooms[room] = true
	if h.rooms[room] == nil {
		h.rooms[room] = map[string]*Client{}
	}
	h.rooms[room][c.ID] = c
	return true
}

func (h *Hub) Leave(c *Client, room string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !c.Rooms[room] {
		return false
	}
	delete(c.Rooms, room)
	delete(h.rooms[room], c.ID)
	return true
}

func (h *Hub) Broadcast(room, t string, payload any) {
	h.mu.Lock()
	list := make([]*Client, 0, len(h.rooms[room]))
	for _, c := range h.rooms[room] {
		list = append(list, c)
	}
	h.mu.Unlock()
	for _, c := range list {
		c.Send(t, payload)
	}
}

func (h *Hub) BroadcastAll(t string, payload any) {
	h.mu.Lock()
	list := make([]*Client, 0, len(h.clients))
	for _, c := range h.clients {
		list = append(list, c)
	}
	h.mu.Unlock()
	for _, c := range list {
		c.Send(t, payload)
	}
}
