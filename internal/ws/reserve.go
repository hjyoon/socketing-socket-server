package ws

import (
	"context"
	"time"
)

func (s *Service) reserveSeats(ctx context.Context, c *Client, p map[string]any) error {
	eventID, dateID, areaID := str(p, "eventId"), str(p, "eventDateId"), str(p, "areaId")
	userID := str(p, "userId")
	seatIDs := strSlice(p, "seatIds")
	if eventID == "" || dateID == "" || areaID == "" || userID == "" {
		c.Send("error", map[string]string{"message": "Invalid reserveSeats parameters."})
		return nil
	}
	if len(seatIDs) == 0 {
		c.Send("error", map[string]string{"message": "Invalid seat IDs."})
		return nil
	}
	room, area := roomName(eventID, dateID), areaName(eventID, dateID, areaID)
	seats, updates := []Seat{}, []map[string]any{}
	for _, id := range seatIDs {
		seat, ok, err := s.cache.Seat(ctx, area, id)
		if err != nil {
			return err
		}
		if !ok || !s.canReserve(ctx, c, area, seat) {
			return nil
		}
		now := time.Now().Format(time.RFC3339Nano)
		seat.ReservedUserID, seat.SelectedBy = ptr(userID), nil
		seat.UpdatedAt, seat.ExpirationTime = ptr(now), nil
		_ = s.cache.DelSeatTimer(ctx, area, id)
		_ = s.cache.SetSeat(ctx, area, seat)
		seats = append(seats, seat)
		updates = append(updates, seatUpdate(seat))
	}
	createdAt := time.Now().Format(time.RFC3339Nano)
	order := OrderCache{
		UserID: userID, EventDateID: dateID, SeatIDs: seatIDs,
		OrderStatus: "pending", CreatedAt: createdAt,
	}
	orderID, err := s.cache.CreateOrder(ctx, area, order, paymentTimeout)
	if err != nil {
		return err
	}
	selectedArea := s.areaForOrder(ctx, room, areaID)
	exp := time.Now().Add(paymentTimeout).Format(time.RFC3339Nano)
	c.Send("orderMade", map[string]any{"data": map[string]any{
		"id": orderID, "createdAt": order.CreatedAt, "expirationTime": exp,
		"seats": seats, "area": selectedArea,
	}})
	if len(updates) > 0 {
		return s.cache.Publish(ctx, broadcastChannel, map[string]any{
			"room": area, "type": "seatsSelected", "payload": updates,
		})
	}
	return nil
}

func (s *Service) canReserve(ctx context.Context, c *Client, area string, seat Seat) bool {
	if seat.ID == "" {
		c.Send("error", map[string]string{"message": "Invalid seat ID."})
		return false
	}
	if seat.ReservedUserID != nil {
		c.Send("error", map[string]string{"message": "Seat " + seat.ID + " is reserved and cannot be selected."})
		return false
	}
	expired, _ := s.cache.SeatExpired(ctx, area, seat.ID)
	if seat.SelectedBy != nil && *seat.SelectedBy != c.ID && !expired {
		c.Send("error", map[string]string{"message": "Seat " + seat.ID + " is already selected by another user."})
		return false
	}
	return true
}

func (s *Service) areaForOrder(ctx context.Context, room, areaID string) map[string]any {
	areas, _ := s.cache.Areas(ctx, room)
	for _, area := range areas {
		if area.ID == areaID {
			return map[string]any{"id": area.ID, "label": area.Label, "price": area.Price}
		}
	}
	return map[string]any{"id": areaID}
}
