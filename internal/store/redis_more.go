package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hjyoon/socketing-socket-server/internal/ws"
	"github.com/redis/go-redis/v9"
)

func (r *Redis) SetSeatTimer(ctx context.Context, area, seatID string, ttl time.Duration) error {
	return r.c.Set(ctx, "timer:"+area+":"+seatID, "active", ttl).Err()
}

func (r *Redis) DelSeatTimer(ctx context.Context, area, seatID string) error {
	return r.c.Del(ctx, "timer:"+area+":"+seatID).Err()
}

func (r *Redis) SeatExpired(ctx context.Context, area, seatID string) (bool, error) {
	n, err := r.c.Exists(ctx, "timer:"+area+":"+seatID).Result()
	return n == 0, err
}

func (r *Redis) CreateOrder(ctx context.Context, area string, order ws.OrderCache, ttl time.Duration) (string, error) {
	id := newOrderID()
	if order.CreatedAt == "" {
		order.CreatedAt = time.Now().Format(time.RFC3339Nano)
	}
	if order.OrderStatus == "" {
		order.OrderStatus = "pending"
	}
	if err := hset(ctx, r.c, "order:"+area, id, order); err != nil {
		return "", err
	}
	return id, r.c.Set(ctx, "paymentTimer:"+area+":"+id, "active", ttl).Err()
}

func (r *Redis) Order(ctx context.Context, area, id string) (ws.OrderCache, bool, error) {
	raw, err := r.c.HGet(ctx, "order:"+area, id).Result()
	if err == redis.Nil {
		return ws.OrderCache{}, false, nil
	}
	if err != nil {
		return ws.OrderCache{}, false, err
	}
	var order ws.OrderCache
	return order, true, json.Unmarshal([]byte(raw), &order)
}

func (r *Redis) CompleteOrder(ctx context.Context, area, id string) error {
	order, ok, err := r.Order(ctx, area, id)
	if err != nil || !ok {
		return err
	}
	order.OrderStatus = "completed"
	return hset(ctx, r.c, "order:"+area, id, order)
}

func (r *Redis) DeleteOrder(ctx context.Context, area, id string) error {
	return r.c.HDel(ctx, "order:"+area, id).Err()
}

func (r *Redis) Publish(ctx context.Context, channel string, message any) error {
	raw, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return r.c.Publish(ctx, channel, raw).Err()
}

func (r *Redis) Subscribe(ctx context.Context, channel string, cb func(string)) error {
	sub := r.c.Subscribe(ctx, channel)
	go func() {
		for msg := range sub.Channel() {
			cb(msg.Payload)
		}
	}()
	return nil
}

func (r *Redis) SubscribeExpired(ctx context.Context, cb func(string)) error {
	_ = r.c.ConfigSet(ctx, "notify-keyspace-events", "Ex").Err()
	sub := r.c.PSubscribe(ctx, "__keyevent@0__:expired")
	go func() {
		for msg := range sub.Channel() {
			cb(msg.Payload)
		}
	}()
	return nil
}
