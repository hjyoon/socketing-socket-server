package ws

import (
	"context"
	"testing"
)

func TestJoinSelectReserveOrderAndExit(t *testing.T) {
	ctx := context.Background()
	svc, cache, c, out := testService()
	svc.HandleMessage(ctx, c, []byte(`{"type":"joinRoom","payload":{"eventId":"e","eventDateId":"d"}}`))
	if (*out)[0].t != "roomJoined" || !c.Rooms["e_d"] {
		t.Fatalf("room join failed")
	}
	svc.HandleMessage(ctx, c, []byte(`{"type":"joinArea","payload":{"eventId":"e","eventDateId":"d","areaId":"a"}}`))
	if (*out)[1].t != "areaJoined" || !c.Rooms["e_d_a"] {
		t.Fatalf("area join failed")
	}
	svc.HandleMessage(ctx, c, []byte(`{"type":"selectSeats","payload":{"eventId":"e","eventDateId":"d","areaId":"a","seatId":"s1","numberOfSeats":2}}`))
	if len(cache.pub) == 0 {
		t.Fatalf("selection was not published")
	}
	svc.HandleMessage(ctx, c, []byte(`{"type":"reserveSeats","payload":{"eventId":"e","eventDateId":"d","areaId":"a","seatIds":["s1"],"userId":"u"}}`))
	if last(out).t != "orderMade" {
		t.Fatalf("orderMade not sent")
	}
	svc.HandleMessage(ctx, c, []byte(`{"type":"requestOrder","payload":{"eventId":"e","eventDateId":"d","areaId":"a","orderId":"o1","userId":"u","paymentMethod":"socket_pay"}}`))
	if last(out).t != "orderApproved" {
		t.Fatalf("orderApproved not sent")
	}
	svc.HandleMessage(ctx, c, []byte(`{"type":"exitArea","payload":{"eventId":"e","eventDateId":"d","areaId":"a"}}`))
	svc.HandleMessage(ctx, c, []byte(`{"type":"exitRoom","payload":{"eventId":"e","eventDateId":"d"}}`))
	if c.Rooms["e_d"] || c.Rooms["e_d_a"] {
		t.Fatalf("rooms not left")
	}
}

func TestErrorsAndExpiration(t *testing.T) {
	ctx := context.Background()
	svc, cache, c, out := testService()
	svc.HandleMessage(ctx, c, []byte(`bad`))
	svc.HandleMessage(ctx, c, []byte(`{"type":"missing","payload":{}}`))
	svc.HandleMessage(ctx, c, []byte(`{"type":"selectSeats","payload":{}}`))
	if len(*out) < 3 {
		t.Fatalf("errors not emitted")
	}
	area := "e_d_a"
	_ = cache.SetSeat(ctx, area, Seat{ID: "s1", Row: 1, Number: 1, SelectedBy: ptr("x")})
	if err := svc.expireSeat(ctx, area, "s1"); err != nil {
		t.Fatal(err)
	}
	cache.orders[area] = map[string]OrderCache{"o": {SeatIDs: []string{"s1"}, OrderStatus: "pending"}}
	if err := svc.expireOrder(ctx, area, "o"); err != nil {
		t.Fatal(err)
	}
	svc.handleBroadcast(`{"room":"r","type":"x","payload":{"a":1}}`)
	svc.disconnect(ctx, c)
}

func last(out *[]sent) sent { return (*out)[len(*out)-1] }
