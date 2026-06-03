package ws

import "context"

func (s *Service) HandleMessage(ctx context.Context, c *Client, raw []byte) {
	t, p, ok := decodeMessage(raw)
	if !ok {
		c.Send("error", map[string]string{"message": "Invalid message format."})
		return
	}
	var err error
	switch t {
	case "joinRoom":
		err = s.joinRoom(ctx, c, p)
	case "joinArea":
		err = s.joinArea(ctx, c, p)
	case "selectSeats":
		err = s.selectSeats(ctx, c, p)
	case "reserveSeats":
		err = s.reserveSeats(ctx, c, p)
	case "requestOrder":
		err = s.requestOrder(ctx, c, p)
	case "exitArea":
		err = s.exitArea(ctx, c, p)
	case "exitRoom":
		err = s.exitRoom(ctx, c, p)
	default:
		c.Send("error", map[string]string{"message": "Unknown message type."})
	}
	if err != nil {
		c.Send("error", map[string]string{"message": err.Error()})
	}
}

func (s *Service) disconnect(ctx context.Context, c *Client) {
	for room := range c.Rooms {
		if isArea(room) {
			s.leaveArea(ctx, c, room)
		}
	}
	for room := range c.Rooms {
		if isMain(room) {
			s.leaveRoom(ctx, c, room)
		}
	}
	s.hub.Remove(c)
}

func isMain(room string) bool { return countSep(room) == 1 }
func isArea(room string) bool { return countSep(room) == 2 }

func countSep(value string) int {
	n := 0
	for _, ch := range value {
		if ch == '_' {
			n++
		}
	}
	return n
}
