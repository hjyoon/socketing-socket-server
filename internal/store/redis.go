package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hjyoon/socketing-socket-server/internal/auth"
	"github.com/hjyoon/socketing-socket-server/internal/ws"
	"github.com/redis/go-redis/v9"
)

type Redis struct{ c *redis.Client }

func NewRedis(c *redis.Client) *Redis { return &Redis{c: c} }

func (r *Redis) Ready(ctx context.Context) error { return r.c.Ping(ctx).Err() }

func (r *Redis) ValidateToken(ctx context.Context, token string) (bool, error) {
	key := "token:" + token
	value, err := r.c.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil || value != "issued" {
		return false, err
	}
	return true, r.c.Del(ctx, key).Err()
}

func (r *Redis) RoomCount(ctx context.Context, room string) (int, error) {
	n, err := r.c.Get(ctx, "room:"+room+":count").Int()
	if err == redis.Nil {
		return 0, nil
	}
	return n, err
}

func (r *Redis) IncRoom(ctx context.Context, room string) (int, error) {
	n, err := r.c.Incr(ctx, "room:"+room+":count").Result()
	return int(n), err
}

func (r *Redis) DecRoom(ctx context.Context, room string) (int, error) {
	script := `local v=redis.call("GET",KEYS[1]);if v and tonumber(v)>0 then return redis.call("DECR",KEYS[1]) else return 0 end`
	n, err := r.c.Eval(ctx, script, []string{"room:" + room + ":count"}).Int()
	return n, err
}

func (r *Redis) Areas(ctx context.Context, room string) ([]ws.Area, error) {
	return hvals[ws.Area](ctx, r.c, "areas:"+room)
}

func (r *Redis) SetArea(ctx context.Context, room string, area ws.Area) error {
	return hset(ctx, r.c, "areas:"+room, area.ID, area)
}

func (r *Redis) Seats(ctx context.Context, area string) ([]ws.Seat, error) {
	return hvals[ws.Seat](ctx, r.c, "seats:"+area)
}

func (r *Redis) SetSeat(ctx context.Context, area string, seat ws.Seat) error {
	return hset(ctx, r.c, "seats:"+area, seat.ID, seat)
}

func (r *Redis) Seat(ctx context.Context, area, id string) (ws.Seat, bool, error) {
	raw, err := r.c.HGet(ctx, "seats:"+area, id).Result()
	if err == redis.Nil {
		return ws.Seat{}, false, nil
	}
	if err != nil {
		return ws.Seat{}, false, err
	}
	var seat ws.Seat
	return seat, true, json.Unmarshal([]byte(raw), &seat)
}

func hset(ctx context.Context, c *redis.Client, key, field string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.HSet(ctx, key, field, raw).Err()
}

func hvals[T any](ctx context.Context, c *redis.Client, key string) ([]T, error) {
	raw, err := c.HVals(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	out := make([]T, 0, len(raw))
	for _, item := range raw {
		var value T
		if json.Unmarshal([]byte(item), &value) == nil {
			out = append(out, value)
		}
	}
	return out, nil
}

func (r *Redis) Lock(ctx context.Context, key string, ttl time.Duration) (func(context.Context), bool, error) {
	ok, err := r.c.SetNX(ctx, key, "locked", ttl).Result()
	return func(ctx context.Context) { _ = r.c.Del(ctx, key).Err() }, ok, err
}

func newOrderID() string { return auth.UUID() }
