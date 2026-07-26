package service

import (
	"time"
)

// SubscriptionCacheData represents cached subscription data
type SubscriptionCacheData struct {
	Status       string
	ExpiresAt    time.Time
	TermVersion  int64
	DailyUsage   float64
	WeeklyUsage  float64
	MonthlyUsage float64
	TotalUsage   float64
	Version      int64
}
