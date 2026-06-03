package ws

import "sort"

func FindAdjacent(seats []Seat, selected Seat, count int) []Seat {
	available := make([]Seat, 0, len(seats))
	for _, s := range seats {
		if s.ReservedUserID == nil && s.SelectedBy == nil {
			available = append(available, s)
		}
	}
	result := []Seat{selected}
	add := func(row, num int) bool {
		if len(result) >= count {
			return false
		}
		for _, s := range available {
			if s.Row == row && s.Number == num && !contains(result, s.ID) {
				result = append(result, s)
				return true
			}
		}
		return false
	}
	for offset := 1; len(result) < count; offset++ {
		found := add(selected.Row, selected.Number+offset)
		found = add(selected.Row, selected.Number-offset) || found
		if !found {
			break
		}
	}
	rows := rowsByDistance(available, selected.Row)
	for _, row := range rows {
		for offset := 0; len(result) < count; offset++ {
			found := add(row, selected.Number+offset)
			found = add(row, selected.Number-offset) || found
			if !found {
				break
			}
		}
	}
	sort.Slice(available, func(i, j int) bool {
		di := abs(available[i].Row-selected.Row) + abs(available[i].Number-selected.Number)
		dj := abs(available[j].Row-selected.Row) + abs(available[j].Number-selected.Number)
		return di < dj
	})
	for _, s := range available {
		if len(result) >= count {
			break
		}
		if !contains(result, s.ID) {
			result = append(result, s)
		}
	}
	return result
}

func rowsByDistance(seats []Seat, row int) []int {
	seen := map[int]bool{}
	rows := []int{}
	for _, s := range seats {
		if s.Row != row && !seen[s.Row] {
			seen[s.Row] = true
			rows = append(rows, s.Row)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return abs(rows[i]-row) < abs(rows[j]-row) })
	return rows
}

func contains(seats []Seat, id string) bool {
	for _, s := range seats {
		if s.ID == id {
			return true
		}
	}
	return false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
