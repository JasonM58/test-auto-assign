package autoassign

func CalculateScore(openPR, recentReviews int) int {
	return 100 - (openPR * 15) - (recentReviews * 5)
}
