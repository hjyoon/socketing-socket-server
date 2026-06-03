package ws

import "testing"

func TestFindAdjacent(t *testing.T) {
	seats := []Seat{
		{ID: "s1", Row: 1, Number: 1},
		{ID: "s2", Row: 1, Number: 2},
		{ID: "s3", Row: 2, Number: 1},
	}
	got := FindAdjacent(seats, seats[0], 3)
	if len(got) != 3 || got[1].ID != "s2" {
		t.Fatalf("unexpected adjacent result: %#v", got)
	}
	if !isMain("a_b") || !isArea("a_b_c") || countSep("a_b_c") != 2 {
		t.Fatalf("room helpers failed")
	}
}
