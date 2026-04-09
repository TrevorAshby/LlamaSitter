package analytics

import (
	"testing"
	"time"
)

func TestDefaultWindowDurations(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 8, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		rangeKey string
		want     time.Duration
	}{
		{name: "day", rangeKey: "day", want: 24 * time.Hour},
		{name: "week", rangeKey: "week", want: 7 * 24 * time.Hour},
		{name: "month", rangeKey: "month", want: 35 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := DefaultWindow(tt.rangeKey, now)
			if end.Sub(start) != tt.want {
				t.Fatalf("expected %s window duration %s, got %s", tt.rangeKey, tt.want, end.Sub(start))
			}
			if !end.Equal(now) {
				t.Fatalf("expected end %s, got %s", now, end)
			}
		})
	}
}

func TestPreviousWindow(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.April, 7, 12, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.April, 8, 12, 0, 0, 0, time.UTC)

	prevStart, prevEnd := PreviousWindow(start, end)
	if want := time.Date(2026, time.April, 6, 12, 0, 0, 0, time.UTC); !prevStart.Equal(want) {
		t.Fatalf("expected previous start %s, got %s", want, prevStart)
	}
	if !prevEnd.Equal(start) {
		t.Fatalf("expected previous end %s, got %s", start, prevEnd)
	}
}

func TestBucketWindowsForMonth(t *testing.T) {
	t.Parallel()

	end := time.Date(2026, time.April, 8, 12, 0, 0, 0, time.UTC)
	start := end.Add(-35 * 24 * time.Hour)

	windows := BucketWindows("month", start, end)
	if len(windows) != 5 {
		t.Fatalf("expected 5 weekly windows, got %d", len(windows))
	}

	for i, window := range windows {
		if window.End.Sub(window.Start) != 7*24*time.Hour {
			t.Fatalf("window %d duration = %s, want 7d", i, window.End.Sub(window.Start))
		}
		if i == 0 && !window.Start.Equal(start) {
			t.Fatalf("first window start = %s, want %s", window.Start, start)
		}
		if i == len(windows)-1 && !window.End.Equal(end) {
			t.Fatalf("last window end = %s, want %s", window.End, end)
		}
	}
}
