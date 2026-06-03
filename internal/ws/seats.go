package ws

import (
	"context"
	"time"
)

func (s *Service) selectSeats(ctx context.Context, c *Client, p map[string]any) error {
	eventID, dateID, areaID := str(p, "eventId"), str(p, "eventDateId"), str(p, "areaId")
	seatID := str(p, "seatId")
	if eventID == "" || dateID == "" || areaID == "" || seatID == "" {
		c.Send("error", map[string]string{"message": "Invalid selectSeats parameters."})
		return nil
	}
	area := areaName(eventID, dateID, areaID)
	seats, err := s.cache.Seats(ctx, area)
	if err != nil {
		return err
	}
	_ = s.releaseSeats(ctx, c.ID, seats, area)
	selected, ok := findSeat(seats, seatID)
	if !ok {
		c.Send("error", map[string]string{"message": "Invalid seat ID."})
		return nil
	}
	count := intVal(p, "numberOfSeats", 1)
	targets, ok := s.selectTargets(ctx, c, area, selected, seats, count)
	if !ok {
		return nil
	}
	now := time.Now()
	updates := make([]map[string]any, 0, len(targets))
	for _, seat := range targets {
		ts := now.Format(time.RFC3339Nano)
		exp := now.Add(selectionTimeout).Format(time.RFC3339Nano)
		seat.SelectedBy, seat.UpdatedAt, seat.ExpirationTime = ptr(c.ID), ptr(ts), ptr(exp)
		_ = s.cache.SetSeat(ctx, area, seat)
		_ = s.cache.SetSeatTimer(ctx, area, seat.ID, selectionTimeout)
		updates = append(updates, seatUpdate(seat))
	}
	return s.cache.Publish(ctx, broadcastChannel, map[string]any{
		"room": area, "type": "seatsSelected", "payload": updates,
	})
}

func (s *Service) selectTargets(ctx context.Context, c *Client, area string, selected Seat, seats []Seat, count int) ([]Seat, bool) {
	if count <= 1 {
		if selected.ReservedUserID != nil {
			c.Send("error", map[string]string{"message": "Seat " + selected.ID + " is reserved and cannot be selected."})
			return nil, false
		}
		expired, _ := s.cache.SeatExpired(ctx, area, selected.ID)
		if selected.SelectedBy != nil && !expired {
			c.Send("error", map[string]string{"message": "Seat " + selected.ID + " is already selected by another user."})
			return nil, false
		}
		return []Seat{selected}, true
	}
	targets := FindAdjacent(seats, selected, count)
	if len(targets) < count {
		c.Send("error", map[string]string{"message": "Not enough adjacent seats available"})
		return nil, false
	}
	return targets, true
}

func (s *Service) releaseSeats(ctx context.Context, socketID string, seats []Seat, area string) error {
	now := time.Now().Format(time.RFC3339Nano)
	updates := []map[string]any{}
	for _, seat := range seats {
		if seat.SelectedBy != nil && *seat.SelectedBy == socketID {
			seat.SelectedBy, seat.ReservedUserID = nil, nil
			seat.UpdatedAt, seat.ExpirationTime = ptr(now), nil
			_ = s.cache.DelSeatTimer(ctx, area, seat.ID)
			_ = s.cache.SetSeat(ctx, area, seat)
			updates = append(updates, seatUpdate(seat))
		}
	}
	if len(updates) == 0 {
		return nil
	}
	return s.cache.Publish(ctx, broadcastChannel, map[string]any{
		"room": area, "type": "seatsSelected", "payload": updates,
	})
}

func findSeat(seats []Seat, id string) (Seat, bool) {
	for _, seat := range seats {
		if seat.ID == id {
			return seat, true
		}
	}
	return Seat{}, false
}
