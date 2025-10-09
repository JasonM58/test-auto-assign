package timegen

import "time"

type (
	TimeService struct{}
)

func NewTimeService() TimeService {
	return TimeService{}
}

func (r *TimeService) GenerateTimeNow() time.Time {
	return time.Now()
}
