package store

import (
	"context"
	"database/sql"
)

func insertOrder(ctx context.Context, tx *sql.Tx, userID, method, dateID string, seats []string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx,
		`INSERT INTO "order" ("userId","paymentMethod") VALUES ($1,$2) RETURNING id`,
		userID, method).Scan(&id)
	if err != nil {
		return "", err
	}
	for _, seatID := range seats {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO reservation ("orderId","eventDateId","seatId") VALUES ($1,$2,$3)`,
			id, dateID, seatID)
		if err != nil {
			return "", err
		}
	}
	return id, nil
}
