package ws

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	ID    string
	Rooms map[string]bool
	Conn  *websocket.Conn
	send  func(string, any)
	once  sync.Once
}

func NewTestClient(id string) *Client {
	return &Client{ID: id, Rooms: map[string]bool{}}
}

func (c *Client) Send(t string, payload any) {
	if c.send != nil {
		c.send(t, payload)
		return
	}
	if c.Conn == nil {
		return
	}
	_ = c.Conn.WriteJSON(map[string]any{"type": t, "payload": payload})
}

func sendRaw(conn *websocket.Conn, t string, payload any) {
	if conn == nil {
		return
	}
	_ = conn.WriteJSON(map[string]any{"type": t, "payload": payload})
}

func decodeMessage(raw []byte) (string, map[string]any, bool) {
	var msg struct {
		Type    string         `json:"type"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil || msg.Type == "" {
		return "", nil, false
	}
	if msg.Payload == nil {
		msg.Payload = map[string]any{}
	}
	return msg.Type, msg.Payload, true
}
