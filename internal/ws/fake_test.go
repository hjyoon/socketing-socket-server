package ws

import (
	"context"
	"encoding/json"
	"time"
)

type sent struct {
	t string
	p any
}

type fakeCache struct {
	areas  map[string][]Area
	seats  map[string]map[string]Seat
	orders map[string]map[string]OrderCache
	pub    []string
}

func newFakeCache() *fakeCache {
	return &fakeCache{areas: map[string][]Area{}, seats: map[string]map[string]Seat{}, orders: map[string]map[string]OrderCache{}}
}

func (f *fakeCache) Ready(context.Context) error { return nil }
func (f *fakeCache) ValidateToken(context.Context, string) (bool, error) {
	return true, nil
}
func (f *fakeCache) RoomCount(context.Context, string) (int, error) { return 0, nil }
func (f *fakeCache) IncRoom(context.Context, string) (int, error)   { return 1, nil }
func (f *fakeCache) DecRoom(context.Context, string) (int, error)   { return 0, nil }
func (f *fakeCache) Areas(_ context.Context, room string) ([]Area, error) {
	return f.areas[room], nil
}
func (f *fakeCache) SetArea(_ context.Context, room string, a Area) error {
	f.areas[room] = append(f.areas[room], a)
	return nil
}
func (f *fakeCache) Seats(_ context.Context, area string) ([]Seat, error) {
	out := []Seat{}
	for _, s := range f.seats[area] {
		out = append(out, s)
	}
	return out, nil
}
func (f *fakeCache) SetSeat(_ context.Context, area string, s Seat) error {
	if f.seats[area] == nil {
		f.seats[area] = map[string]Seat{}
	}
	f.seats[area][s.ID] = s
	return nil
}
func (f *fakeCache) Seat(_ context.Context, area, id string) (Seat, bool, error) {
	s, ok := f.seats[area][id]
	return s, ok, nil
}
func (f *fakeCache) SetSeatTimer(context.Context, string, string, time.Duration) error {
	return nil
}
func (f *fakeCache) DelSeatTimer(context.Context, string, string) error { return nil }
func (f *fakeCache) SeatExpired(context.Context, string, string) (bool, error) {
	return false, nil
}
func (f *fakeCache) Publish(_ context.Context, _ string, m any) error {
	raw, _ := json.Marshal(m)
	f.pub = append(f.pub, string(raw))
	return nil
}
