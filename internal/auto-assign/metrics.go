package autoassign

import "context"

// =====================
// INTERFACE
// =====================

// MetricsProvider adalah abstraction layer untuk metrics source.
// Service tidak boleh tahu apakah data berasal dari Prometheus, GitHub, dll.
type MetricsProvider interface {

	// Workload reviewer berdasarkan PR load
	GetReviewWorkload(ctx context.Context) (map[string]int, error)

	// Activity reviewer (jumlah review terbaru)
	GetRecentReviewCount(ctx context.Context) (map[string]int, error)
}

// =====================
// DOMAIN MODEL (OPTIONAL BUT RECOMMENDED)
// =====================

// ReviewerMetrics menyatukan semua metric untuk satu reviewer
type ReviewerMetrics struct {
	Workload      int
	RecentReviews int
}

// =====================
// HELPER (OPTIONAL)
// =====================

// MergeMetrics menggabungkan workload + recent review menjadi 1 struct
func MergeMetrics(
	workloads map[string]int,
	recents map[string]int,
) map[string]ReviewerMetrics {

	out := make(map[string]ReviewerMetrics)

	// dari workload
	for reviewer, w := range workloads {
		out[reviewer] = ReviewerMetrics{
			Workload:      w,
			RecentReviews: recents[reviewer],
		}
	}

	// handle reviewer yang hanya ada di recents
	for reviewer, r := range recents {
		if _, exists := out[reviewer]; !exists {
			out[reviewer] = ReviewerMetrics{
				Workload:      0,
				RecentReviews: r,
			}
		}
	}

	return out
}
