package cmd

// paginateResults applies pagination to a result set and returns the page slice,
// total count, and the effective page number.
func paginateResults(results [][]interface{}, page, limit int) ([][]interface{}, int, int) {
	total := len(results)
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		return results, total, page
	}

	start := (page - 1) * limit
	end := start + limit

	if start >= total {
		return [][]interface{}{}, total, page
	}
	if end > total {
		end = total
	}

	return results[start:end], total, page
}
