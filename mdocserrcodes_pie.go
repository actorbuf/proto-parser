package proto_parser

import (
	"sort"
)

// SortStableUsing works similar to sort.SliceStable. However, unlike sort.SliceStable the
// slice returned will be reallocated as to not modify the input slice.
func (ss MDocsErrCodes) SortStableUsing(less func(a, b MDocsErrCodeField) bool) MDocsErrCodes {
	// Avoid the allocation. If there is one element or less it is already
	// sorted.
	if len(ss) < 2 {
		return ss
	}

	sorted := make(MDocsErrCodes, len(ss))
	copy(sorted, ss)
	sort.SliceStable(sorted, func(i, j int) bool {
		return less(sorted[i], sorted[j])
	})

	return sorted
}
