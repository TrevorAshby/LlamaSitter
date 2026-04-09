package analytics

import (
	"strings"
	"time"
)

type RangeSpec struct {
	Name           string
	BucketCount    int
	BucketDuration time.Duration
}

type BucketWindow struct {
	Start time.Time
	End   time.Time
}

func ResolveRange(name string) RangeSpec {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "week":
		return RangeSpec{
			Name:           "week",
			BucketCount:    7,
			BucketDuration: 24 * time.Hour,
		}
	case "month":
		return RangeSpec{
			Name:           "month",
			BucketCount:    5,
			BucketDuration: 7 * 24 * time.Hour,
		}
	default:
		return RangeSpec{
			Name:           "day",
			BucketCount:    24,
			BucketDuration: time.Hour,
		}
	}
}

func DefaultWindow(rangeName string, now time.Time) (time.Time, time.Time) {
	spec := ResolveRange(rangeName)
	end := now.UTC()
	start := end.Add(-time.Duration(spec.BucketCount) * spec.BucketDuration)
	return start, end
}

func NormalizeWindow(rangeName string, now, start, end time.Time) (time.Time, time.Time) {
	if start.IsZero() || end.IsZero() || !start.Before(end) {
		return DefaultWindow(rangeName, now)
	}
	return start.UTC(), end.UTC()
}

func PreviousWindow(start, end time.Time) (time.Time, time.Time) {
	if start.IsZero() || end.IsZero() || !start.Before(end) {
		return time.Time{}, time.Time{}
	}

	duration := end.Sub(start)
	return start.Add(-duration), start
}

func BucketWindows(rangeName string, start, end time.Time) []BucketWindow {
	spec := ResolveRange(rangeName)
	start, end = NormalizeWindow(rangeName, end, start, end)

	windows := make([]BucketWindow, 0, spec.BucketCount)
	cursor := start
	for i := 0; i < spec.BucketCount; i++ {
		next := cursor.Add(spec.BucketDuration)
		if i == spec.BucketCount-1 || next.After(end) {
			next = end
		}
		windows = append(windows, BucketWindow{
			Start: cursor,
			End:   next,
		})
		cursor = next
	}
	return windows
}

func BucketLabel(rangeName string, start time.Time) string {
	switch ResolveRange(rangeName).Name {
	case "week":
		return start.Format("Mon")
	case "month":
		return start.Format("Jan 2")
	default:
		return start.Format("3 PM")
	}
}
