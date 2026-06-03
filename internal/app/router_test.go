package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hjyoon/socketing-socket-server/internal/ws"
)

func TestRouterHealthAndCORS(t *testing.T) {
	s := ws.NewService(ws.Config{}, okCache{}, okStore{})
	r := NewRouter(Config{CORSOrigins: []string{"http://localhost:5173"}}, s)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest("GET", "/liveness", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("liveness got %d", res.Code)
	}
	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	res = httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("preflight got %d", res.Code)
	}
	res = httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest("GET", "/readiness", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("readiness got %d", res.Code)
	}
	res = httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest("GET", "/", nil))
	if res.Code == http.StatusOK {
		t.Fatalf("websocket without upgrade should not be ok")
	}
	r = NewRouter(Config{}, ws.NewService(ws.Config{}, errCache{}, okStore{}))
	res = httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest("GET", "/readiness", nil))
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("readiness error got %d", res.Code)
	}
}

type okCache struct{}
type okStore struct{}
type errCache struct{ okCache }

func (okCache) Ready(context.Context) error  { return nil }
func (okStore) Ready(context.Context) error  { return nil }
func (errCache) Ready(context.Context) error { return errors.New("down") }
func (okCache) ValidateToken(context.Context, string) (bool, error) {
	return true, nil
}
func (okCache) RoomCount(context.Context, string) (int, error)   { return 0, nil }
func (okCache) IncRoom(context.Context, string) (int, error)     { return 0, nil }
func (okCache) DecRoom(context.Context, string) (int, error)     { return 0, nil }
func (okCache) Areas(context.Context, string) ([]ws.Area, error) { return nil, nil }
func (okCache) SetArea(context.Context, string, ws.Area) error   { return nil }
func (okCache) Seats(context.Context, string) ([]ws.Seat, error) { return nil, nil }
func (okCache) SetSeat(context.Context, string, ws.Seat) error   { return nil }
func (okCache) Seat(context.Context, string, string) (ws.Seat, bool, error) {
	return ws.Seat{}, false, nil
}
func (okCache) SetSeatTimer(context.Context, string, string, time.Duration) error { return nil }
func (okCache) DelSeatTimer(context.Context, string, string) error                { return nil }
func (okCache) SeatExpired(context.Context, string, string) (bool, error)         { return true, nil }
func (okCache) CreateOrder(context.Context, string, ws.OrderCache, time.Duration) (string, error) {
	return "", nil
}
