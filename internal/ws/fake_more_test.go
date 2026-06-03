package ws

import (
	"context"
	"time"
)

func (f *fakeCache) CreateOrder(_ context.Context, area string, o OrderCache, _ time.Duration) (string, error) {
	if f.orders[area] == nil {
		f.orders[area] = map[string]OrderCache{}
	}
	o.CreatedAt = "now"
	f.orders[area]["o1"] = o
	return "o1", nil
}
func (f *fakeCache) Order(_ context.Context, area, id string) (OrderCache, bool, error) {
	o, ok := f.orders[area][id]
	return o, ok, nil
}
func (f *fakeCache) CompleteOrder(_ context.Context, area, id string) error {
	o := f.orders[area][id]
	o.OrderStatus = "completed"
	f.orders[area][id] = o
	return nil
}
func (f *fakeCache) DeleteOrder(_ context.Context, area, id string) error {
	delete(f.orders[area], id)
	return nil
}
func (f *fakeCache) Subscribe(context.Context, string, func(string)) error { return nil }
func (f *fakeCache) SubscribeExpired(context.Context, func(string)) error  { return nil }
func (f *fakeCache) Lock(context.Context, string, time.Duration) (func(context.Context), bool, error) {
	return func(context.Context) {}, true, nil
}

type fakeStore struct {
	areas []Area
	seats []Seat
}

func (f fakeStore) Ready(context.Context) error { return nil }
func (f fakeStore) Areas(context.Context, string) ([]Area, error) {
	return f.areas, nil
}
func (f fakeStore) Seats(context.Context, string, string) ([]Seat, error) {
	return f.seats, nil
}
func (f fakeStore) CreateOrder(context.Context, OrderRequest) (map[string]any, error) {
	return map[string]any{"orderId": "pg1"}, nil
}

func testService() (*Service, *fakeCache, *Client, *[]sent) {
	cache := newFakeCache()
	store := fakeStore{
		areas: []Area{{ID: "a", Label: "A", Price: 10}},
		seats: []Seat{{ID: "s1", Row: 1, Number: 1, AreaID: "a"}, {ID: "s2", Row: 1, Number: 2, AreaID: "a"}},
	}
	svc := NewService(Config{JWTSecret: "s", EntranceJWTSecret: "s"}, cache, store)
	out := []sent{}
	c := NewTestClient("c1")
	c.send = func(t string, p any) { out = append(out, sent{t, p}) }
	svc.hub.Add(c)
	return svc, cache, c, &out
}
