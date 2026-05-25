package trends

import "time"

// DefaultBucketDuration is the width of each time window produced by Aggregate.
// Daily buckets give a human-readable "day-by-day" trend view without
// excessive cardinality (30 buckets = 30 days lookback at this size).
const DefaultBucketDuration = 24 * time.Hour

// MinBucketsForDisplay is the minimum number of non-empty buckets required
// before the Trends tab renders sparklines. Fewer buckets means there is
// not enough longitudinal contrast to show meaningful trends.
const MinBucketsForDisplay = 2

// DefaultLookbackDays is how many calendar days back the Trends tab queries
// from the signal store by default. 30 days = one rolling month of history.
const DefaultLookbackDays = 30

// RoundedNow returns the current UTC time truncated to midnight (start of day).
// Using a rounded "now" as the upper bound means bucket boundaries are
// stable across calls on the same day — the last bucket always starts at
// today's midnight regardless of what time the user presses '6'.
func RoundedNow() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
}
