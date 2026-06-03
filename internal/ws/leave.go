package ws

import "context"

func (s *Service) exitArea(ctx context.Context, c *Client, p map[string]any) error {
	eventID, eventDateID, areaID := str(p, "eventId"), str(p, "eventDateId"), str(p, "areaId")
	if eventID == "" || eventDateID == "" || areaID == "" {
		c.Send("error", map[string]string{"message": "Invalid area parameters."})
		return nil
	}
	area := areaName(eventID, eventDateID, areaID)
	if err := s.leaveArea(ctx, c, area); err != nil {
		return err
	}
	c.Send("areaExited", map[string]string{"message": "You have left the area: " + area})
	return nil
}

func (s *Service) exitRoom(ctx context.Context, c *Client, p map[string]any) error {
	eventID, eventDateID := str(p, "eventId"), str(p, "eventDateId")
	if eventID == "" || eventDateID == "" {
		c.Send("error", map[string]string{"message": "Invalid room parameters."})
		return nil
	}
	room := roomName(eventID, eventDateID)
	s.leaveRoom(ctx, c, room)
	c.Send("roomExited", map[string]string{"message": "You have left the room: " + room})
	return nil
}

func (s *Service) leaveRoom(ctx context.Context, c *Client, room string) {
	if s.hub.Leave(c, room) {
		_, _ = s.cache.DecRoom(ctx, room)
	}
}

func (s *Service) leaveArea(ctx context.Context, c *Client, area string) error {
	if !s.hub.Leave(c, area) {
		return nil
	}
	seats, err := s.cache.Seats(ctx, area)
	if err != nil {
		return err
	}
	return s.releaseSeats(ctx, c.ID, seats, area)
}
