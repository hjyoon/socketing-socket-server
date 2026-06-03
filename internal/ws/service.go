package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hjyoon/socketing-socket-server/internal/auth"
)

type Service struct {
	cfg    Config
	cache  Cache
	store  Store
	hub    *Hub
	ticker *time.Ticker
	done   chan struct{}
}

func NewService(cfg Config, cache Cache, store Store) *Service {
	return &Service{
		cfg: cfg, cache: cache, store: store, hub: NewHub(), done: make(chan struct{}),
	}
}

func (s *Service) Start(ctx context.Context) {
	s.ticker = time.NewTicker(time.Second)
	go func() {
		for {
			select {
			case t := <-s.ticker.C:
				s.hub.BroadcastAll("serverTime", t.Format(time.RFC3339Nano))
			case <-s.done:
				return
			}
		}
	}()
	_ = s.cache.Subscribe(ctx, broadcastChannel, s.handleBroadcast)
	_ = s.cache.SubscribeExpired(ctx, func(key string) { go s.expired(ctx, key) })
}

func (s *Service) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
	close(s.done)
}

func (s *Service) Ready(ctx context.Context) error {
	if err := s.cache.Ready(ctx); err != nil {
		return err
	}
	return s.store.Ready(ctx)
}

func (s *Service) HandleHTTP(c *gin.Context) {
	s.ServeWebSocket(c.Writer, c.Request)
}

func (s *Service) ServeWebSocket(w http.ResponseWriter, r *http.Request) {
	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	conn, err := up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	token := r.URL.Query().Get("token")
	if err := s.authToken(r.Context(), conn, token); err != nil {
		_ = conn.Close()
		return
	}
	client := &Client{ID: auth.UUID(), Rooms: map[string]bool{}, Conn: conn}
	s.hub.Add(client)
	s.hub.Join(client, client.ID)
	client.Send("connected", map[string]string{"id": client.ID})
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			s.disconnect(r.Context(), client)
			return
		}
		s.HandleMessage(r.Context(), client, raw)
	}
}

func (s *Service) authToken(ctx context.Context, conn *websocket.Conn, token string) error {
	if token == "" {
		sendRaw(conn, "connect_error", map[string]string{"message": "Authentication error"})
		return errors.New("missing token")
	}
	ok, err := s.cache.ValidateToken(ctx, token)
	if err != nil || !ok {
		sendRaw(conn, "connect_error", map[string]string{"message": "Authentication error 2"})
		return errors.New("invalid issued token")
	}
	if _, err := auth.Verify(token, s.cfg.EntranceJWTSecret); err != nil {
		sendRaw(conn, "connect_error", map[string]string{"message": "Authentication error"})
		return err
	}
	return nil
}

func (s *Service) handleBroadcast(raw string) {
	var msg struct {
		Room    string          `json:"room"`
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if json.Unmarshal([]byte(raw), &msg) != nil || msg.Room == "" || msg.Type == "" {
		return
	}
	var payload any
	_ = json.Unmarshal(msg.Payload, &payload)
	s.hub.Broadcast(msg.Room, msg.Type, payload)
}

func (s *Service) expired(ctx context.Context, key string) {
	parts := strings.Split(key, ":")
	if len(parts) < 3 {
		return
	}
	if parts[0] == "timer" {
		_ = s.expireSeat(ctx, parts[1], parts[2])
	}
	if parts[0] == "paymentTimer" {
		_ = s.expireOrder(ctx, parts[1], parts[2])
	}
}
