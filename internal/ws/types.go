package ws

import (
	"context"
	"time"
)

const (
	selectionTimeout = 10 * time.Second
	paymentTimeout   = 60 * time.Second
	broadcastChannel = "socketing:reservation:broadcast"
)

type Config struct {
	JWTSecret         string
	EntranceJWTSecret string
	SchedulingURL     string
}

type Area struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	SVG   string `json:"svg,omitempty"`
	Price int    `json:"price"`
}

type Seat struct {
	ID             string  `json:"id"`
	CX             float64 `json:"cx"`
	CY             float64 `json:"cy"`
	Row            int     `json:"row"`
	Number         int     `json:"number"`
	AreaID         string  `json:"area_id"`
	SelectedBy     *string `json:"selectedBy"`
	ReservedUserID *string `json:"reservedUserId"`
	UpdatedAt      *string `json:"updatedAt"`
	ExpirationTime *string `json:"expirationTime"`
}

type OrderCache struct {
	UserID      string   `json:"userId"`
	EventDateID string   `json:"eventDateId"`
	SeatIDs     []string `json:"seatIds"`
	OrderStatus string   `json:"orderStatus"`
	CreatedAt   string   `json:"createdAt"`
}

type Cache interface {
	Ready(context.Context) error
	ValidateToken(context.Context, string) (bool, error)
	RoomCount(context.Context, string) (int, error)
	IncRoom(context.Context, string) (int, error)
	DecRoom(context.Context, string) (int, error)
	Areas(context.Context, string) ([]Area, error)
	SetArea(context.Context, string, Area) error
	Seats(context.Context, string) ([]Seat, error)
	SetSeat(context.Context, string, Seat) error
	Seat(context.Context, string, string) (Seat, bool, error)
	SetSeatTimer(context.Context, string, string, time.Duration) error
	DelSeatTimer(context.Context, string, string) error
	SeatExpired(context.Context, string, string) (bool, error)
	CreateOrder(context.Context, string, OrderCache, time.Duration) (string, error)
	Order(context.Context, string, string) (OrderCache, bool, error)
	CompleteOrder(context.Context, string, string) error
	DeleteOrder(context.Context, string, string) error
	Publish(context.Context, string, any) error
	Subscribe(context.Context, string, func(string)) error
	SubscribeExpired(context.Context, func(string)) error
	Lock(context.Context, string, time.Duration) (func(context.Context), bool, error)
}

type Store interface {
	Ready(context.Context) error
	Areas(context.Context, string) ([]Area, error)
	Seats(context.Context, string, string) ([]Seat, error)
	CreateOrder(context.Context, OrderRequest) (map[string]any, error)
}

type OrderRequest struct {
	UserID        string
	OrderID       string
	PaymentMethod string
	EventDateID   string
	AreaName      string
	SeatIDs       []string
}
