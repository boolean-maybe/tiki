package grid

// PadToFullRows pads a slice to fill complete grid rows.
// This is useful for grid layouts where you want each column to have
// the same height (rowCount), adding empty items as needed.
//
// Example: With rowCount=3 and 7 items, this adds 2 padding items
// to make 9 total (3 complete columns of 3 rows each).
func PadToFullRows[T any](items []T, rowCount int) []T {
	if len(items) == 0 {
		return items
	}
	remainder := len(items) % rowCount
	if remainder != 0 {
		padding := rowCount - remainder
		var emptyItem T
		for range padding {
			items = append(items, emptyItem)
		}
	}
	return items
}
