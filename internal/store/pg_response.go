package store

import (
	"context"
	"database/sql"
	"encoding/json"
)

func orderResponse(ctx context.Context, tx *sql.Tx, orderID string) (map[string]any, error) {
	var raw []byte
	err := tx.QueryRowContext(ctx, orderResponseSQL, orderID).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	return out, json.Unmarshal(raw, &out)
}

const orderResponseSQL = `SELECT json_build_object(
'orderId',o.id,'orderCreatedAt',o."createdAt",'orderUpdatedAt',o."updatedAt",
'orderCanceledAt',o."canceledAt",'orderDeletedAt',o."deletedAt",
'paymentMethod',o."paymentMethod",'userId',u.id,'userNickname',u.nickname,
'userEmail',u.email,'userProfileImage',u."profileImage",'userRole',u.role,
'eventDateId',ed.id,'eventDate',ed.date,'eventId',e.id,'eventTitle',e.title,
'eventThumbnail',e.thumbnail,'eventPlace',e.place,'eventCast',e."cast",
'eventAgeLimit',e."ageLimit",'eventSvg',e.svg,'reservations',json_agg(
json_build_object('reservationId',r.id,'seatId',s.id,'seatCx',s.cx,'seatCy',
s.cy,'seatRow',s."row",'seatNumber',s.number,'seatAreaId',a.id,
'seatAreaLabel',a.label,'seatAreaPrice',a.price)))
FROM "order" o JOIN "user" u ON u.id=o."userId"
JOIN reservation r ON r."orderId"=o.id
JOIN event_date ed ON ed.id=r."eventDateId" JOIN event e ON e.id=ed."eventId"
JOIN seat s ON s.id=r."seatId" JOIN area a ON a.id=s."areaId"
WHERE o.id=$1 GROUP BY o.id,u.id,ed.id,e.id`
