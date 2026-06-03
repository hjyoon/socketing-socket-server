package app

import (
	"context"
	"time"

	"github.com/hjyoon/socketing-socket-server/internal/ws"
)

func (okCache) Order(context.Context, string, string) (ws.OrderCache, bool, error) {
	return ws.OrderCache{}, false, nil
}
func (okCache) CompleteOrder(context.Context, string, string) error { return nil }
func (okCache) DeleteOrder(context.Context, string, string) error   { return nil }
func (okCache) Publish(context.Context, string, any) error          { return nil }
func (okCache) Subscribe(context.Context, string, func(string)) error {
	return nil
}
func (okCache) SubscribeExpired(context.Context, func(string)) error { return nil }
func (okCache) Lock(context.Context, string, time.Duration) (func(context.Context), bool, error) {
	return func(context.Context) {}, true, nil
}
func (okStore) Areas(context.Context, string) ([]ws.Area, error) { return nil, nil }
func (okStore) Seats(context.Context, string, string) ([]ws.Seat, error) {
	return nil, nil
}
func (okStore) CreateOrder(context.Context, ws.OrderRequest) (map[string]any, error) {
	return nil, nil
}
