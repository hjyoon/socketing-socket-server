package ws

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/hjyoon/socketing-socket-server/internal/auth"
)

func (s *Service) joinRoom(ctx context.Context, c *Client, p map[string]any) error {
	eventID, eventDateID := str(p, "eventId"), str(p, "eventDateId")
	if eventID == "" || eventDateID == "" {
		c.Send("error", map[string]string{"message": "Invalid room parameters."})
		return nil
	}
	room := roomName(eventID, eventDateID)
	joined := s.hub.Join(c, room)
	count, _ := s.cache.RoomCount(ctx, room)
	if joined {
		count, _ = s.cache.IncRoom(ctx, room)
	}
	_ = count
	areas, err := s.cache.Areas(ctx, room)
	if err != nil {
		return err
	}
	if len(areas) == 0 {
		areas, err = s.store.Areas(ctx, eventID)
		if err != nil {
			return err
		}
		for _, area := range areas {
			_ = s.cache.SetArea(ctx, room, area)
		}
	}
	c.Send("roomJoined", map[string]any{"message": "You have joined the room: " + room, "areas": areas})
	go s.notifyScheduling(eventID, eventDateID)
	return nil
}

func (s *Service) joinArea(ctx context.Context, c *Client, p map[string]any) error {
	eventID, eventDateID, areaID := str(p, "eventId"), str(p, "eventDateId"), str(p, "areaId")
	if eventID == "" || eventDateID == "" || areaID == "" {
		c.Send("error", map[string]string{"message": "Invalid area parameters."})
		return nil
	}
	area := areaName(eventID, eventDateID, areaID)
	s.hub.Join(c, area)
	seats, err := s.cache.Seats(ctx, area)
	if err != nil {
		return err
	}
	if len(seats) == 0 {
		seats, err = s.store.Seats(ctx, eventDateID, areaID)
		if err != nil {
			return err
		}
		for _, seat := range seats {
			_ = s.cache.SetSeat(ctx, area, seat)
		}
	}
	c.Send("areaJoined", map[string]any{"message": "You have joined the area: " + area, "seats": seats})
	return nil
}

func (s *Service) notifyScheduling(eventID, eventDateID string) {
	if s.cfg.SchedulingURL == "" {
		return
	}
	tok, _ := auth.Sign(map[string]any{
		"sub": "scheduling", "eventId": eventID, "eventDateId": eventDateID,
	}, s.cfg.JWTSecret, 10*time.Minute)
	u, _ := url.JoinPath(s.cfg.SchedulingURL, "scheduling/seat/reservation/statistic")
	req, _ := http.NewRequest(http.MethodPost, u, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err == nil && resp != nil {
		_ = resp.Body.Close()
	}
}
