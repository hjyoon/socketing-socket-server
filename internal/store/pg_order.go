package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/hjyoon/socketing-socket-server/internal/ws"
)

func (p *Postgres) CreateOrder(ctx context.Context, req ws.OrderRequest) (map[string]any, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	user, total, err := validateOrder(ctx, tx, req.UserID, req.EventDateID, req.SeatIDs)
	if err != nil {
		return nil, err
	}
	if user.Point < total {
		return nil, errors.New("Insufficient balance.")
	}
	id, err := insertOrder(ctx, tx, req.UserID, req.PaymentMethod, req.EventDateID, req.SeatIDs)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE "user" SET point=point-$1 WHERE id=$2`, total, req.UserID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"orderId": id, "useId": user.ID, "reservations": req.SeatIDs}, nil
}

type dbUser struct {
	ID    string
	Point int
}

func validateOrder(ctx context.Context, tx *sql.Tx, userID, dateID string, seats []string) (dbUser, int, error) {
	var user dbUser
	if err := tx.QueryRowContext(ctx, `SELECT id,point FROM "user" WHERE id=$1`, userID).Scan(&user.ID, &user.Point); err != nil {
		return user, 0, err
	}
	for _, seatID := range seats {
		var exists bool
		err := tx.QueryRowContext(ctx, reservationExistsSQL, dateID, seatID).Scan(&exists)
		if err != nil || exists {
			return user, 0, errors.New("Seat is already reserved.")
		}
	}
	total := 0
	for _, seatID := range seats {
		var price int
		if err := tx.QueryRowContext(ctx, `SELECT area.price FROM seat INNER JOIN area ON seat."areaId"=area.id WHERE seat.id=$1`, seatID).Scan(&price); err != nil {
			return user, 0, err
		}
		total += price
	}
	return user, total, nil
}

const reservationExistsSQL = `
SELECT EXISTS (
 SELECT 1 FROM reservation r
 LEFT JOIN "order" o ON r."orderId" = o.id
 WHERE r."eventDateId"=$1 AND r."seatId"=$2
   AND r."canceledAt" IS NULL AND r."deletedAt" IS NULL
   AND o."canceledAt" IS NULL)`
