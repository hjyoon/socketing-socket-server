package ws

import "context"

func (s *Service) requestOrder(ctx context.Context, c *Client, p map[string]any) error {
	eventID, dateID, areaID := str(p, "eventId"), str(p, "eventDateId"), str(p, "areaId")
	userID, orderID, method := str(p, "userId"), str(p, "orderId"), str(p, "paymentMethod")
	if eventID == "" || dateID == "" || areaID == "" || userID == "" || orderID == "" || method == "" {
		c.Send("error", map[string]string{"message": "Invalid requestOrder parameters."})
		return nil
	}
	area := areaName(eventID, dateID, areaID)
	order, ok, err := s.cache.Order(ctx, area, orderID)
	if err != nil {
		return err
	}
	if !ok {
		c.Send("error", map[string]string{"message": "Invalid cache requestOrderData"})
		return nil
	}
	data, err := s.store.CreateOrder(ctx, OrderRequest{
		UserID: userID, OrderID: orderID, PaymentMethod: method,
		EventDateID: dateID, AreaName: area, SeatIDs: order.SeatIDs,
	})
	if err != nil {
		c.Send("error", map[string]any{"error": "UNKNOWN_ERROR", "message": err.Error()})
		return nil
	}
	_ = s.cache.CompleteOrder(ctx, area, orderID)
	c.Send("orderApproved", map[string]any{"success": true, "data": data})
	return nil
}
