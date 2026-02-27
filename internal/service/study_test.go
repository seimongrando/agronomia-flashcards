package service

import (
	"testing"
	"time"
)

func TestSchedule(t *testing.T) {
	now := time.Date(2026, 2, 27, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		result     int
		streak     int
		wantDue    time.Time
		wantStreak int
		wantResult int16
	}{
		{
			name:       "wrong → +1d, streak reset",
			result:     0,
			streak:     5,
			wantDue:    now.Add(24 * time.Hour),
			wantStreak: 0,
			wantResult: 0,
		},
		{
			name:       "hard → +3d, streak reset",
			result:     1,
			streak:     3,
			wantDue:    now.Add(3 * 24 * time.Hour),
			wantStreak: 0,
			wantResult: 1,
		},
		{
			name:       "correct → +7d, streak incremented",
			result:     2,
			streak:     2,
			wantDue:    now.Add(7 * 24 * time.Hour),
			wantStreak: 3,
			wantResult: 2,
		},
		{
			name:       "correct from zero streak",
			result:     2,
			streak:     0,
			wantDue:    now.Add(7 * 24 * time.Hour),
			wantStreak: 1,
			wantResult: 2,
		},
		{
			name:       "invalid result defaults to wrong",
			result:     99,
			streak:     4,
			wantDue:    now.Add(24 * time.Hour),
			wantStreak: 0,
			wantResult: 0,
		},
		{
			name:       "negative result defaults to wrong",
			result:     -1,
			streak:     2,
			wantDue:    now.Add(24 * time.Hour),
			wantStreak: 0,
			wantResult: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Schedule(tt.result, tt.streak, now)
			if !got.NextDue.Equal(tt.wantDue) {
				t.Errorf("NextDue = %v, want %v", got.NextDue, tt.wantDue)
			}
			if got.Streak != tt.wantStreak {
				t.Errorf("Streak = %d, want %d", got.Streak, tt.wantStreak)
			}
			if got.LastResult != tt.wantResult {
				t.Errorf("LastResult = %d, want %d", got.LastResult, tt.wantResult)
			}
		})
	}
}
