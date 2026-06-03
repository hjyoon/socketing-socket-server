package store

import (
	"context"

	"github.com/hjyoon/socketing-socket-server/internal/ws"
)

func (p *Postgres) Areas(ctx context.Context, eventID string) ([]ws.Area, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT id,label,svg,price FROM area
		WHERE "eventId"=$1 AND "deletedAt" IS NULL`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var areas []ws.Area
	for rows.Next() {
		var a ws.Area
		if err := rows.Scan(&a.ID, &a.Label, &a.SVG, &a.Price); err != nil {
			return nil, err
		}
		areas = append(areas, a)
	}
	return areas, rows.Err()
}

func (p *Postgres) Seats(ctx context.Context, eventDateID, areaID string) ([]ws.Seat, error) {
	rows, err := p.db.QueryContext(ctx, seatSQL, areaID, eventDateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var seats []ws.Seat
	for rows.Next() {
		var s ws.Seat
		if err := rows.Scan(&s.ID, &s.CX, &s.CY, &s.Row, &s.Number,
			&s.AreaID, &s.ReservedUserID); err != nil {
			return nil, err
		}
		seats = append(seats, s)
	}
	return seats, rows.Err()
}

const seatSQL = `
SELECT seat.id, seat.cx, seat.cy, seat.row, seat.number,
       seat."areaId", "order"."userId"
FROM seat
LEFT JOIN reservation
  ON reservation."seatId" = seat.id
 AND reservation."canceledAt" IS NULL
 AND reservation."deletedAt" IS NULL
LEFT JOIN event_date AS eventDate
  ON reservation."eventDateId" = eventDate.id
LEFT JOIN "order"
  ON reservation."orderId" = "order".id
 AND "order"."canceledAt" IS NULL
 AND "order"."deletedAt" IS NULL
WHERE seat."areaId" = $1
  AND (eventDate.id = $2 OR eventDate.id IS NULL)`
