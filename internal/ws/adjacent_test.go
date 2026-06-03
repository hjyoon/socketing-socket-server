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

func TestFindAdjacentDoesNotExceedCount(t *testing.T) {
	seats := []Seat{
		{ID: "s1", Row: 1, Number: 1},
		{ID: "s2", Row: 1, Number: 2},
		{ID: "s3", Row: 1, Number: 3},
		{ID: "s4", Row: 1, Number: 4},
		{ID: "s5", Row: 1, Number: 5},
	}
	got := FindAdjacent(seats, seats[2], 4)
	if len(got) != 4 {
		t.Fatalf("expected 4 seats, got %d: %#v", len(got), got)
	}
}
