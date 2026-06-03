package ws

import (
	"context"
	"time"
)

func (s *Service) expireSeat(ctx context.Context, area, seatID string) error {
	unlock, ok, err := s.cache.Lock(ctx, "lock:seat:"+area+":"+seatID, 10*time.Second)
	if err != nil || !ok {
		return err
	}
	defer unlock(ctx)
	seat, ok, err := s.cache.Seat(ctx, area, seatID)
	if err != nil || !ok {
		return err
	}
	now := time.Now().Format(time.RFC3339Nano)
	seat.SelectedBy, seat.ReservedUserID = nil, nil
	seat.UpdatedAt, seat.ExpirationTime = ptr(now), nil
	_ = s.cache.SetSeat(ctx, area, seat)
	return s.cache.Publish(ctx, broadcastChannel, map[string]any{
		"room": area, "type": "seatsSelected", "payload": []map[string]any{seatUpdate(seat)},
	})
}

func (s *Service) expireOrder(ctx context.Context, area, orderID string) error {
	order, ok, err := s.cache.Order(ctx, area, orderID)
	if err != nil || !ok {
		return err
	}
	if order.OrderStatus == "pending" {
		for _, seatID := range order.SeatIDs {
			_ = s.expireSeat(ctx, area, seatID)
		}
	}
	return s.cache.DeleteOrder(ctx, area, orderID)
}

func seatUpdate(seat Seat) map[string]any {
	return map[string]any{
		"seatId": seat.ID, "selectedBy": seat.SelectedBy,
		"updatedAt": seat.UpdatedAt, "expirationTime": seat.ExpirationTime,
		"reservedUserId": seat.ReservedUserID,
	}
}
