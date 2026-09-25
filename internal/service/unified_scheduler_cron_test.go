package service

import (
	"testing"
	"time"
)

func TestNormalizeCronExprPrependsSecondsForFiveField(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"*/30 * * * *", "0 */30 * * * *"},
		{"0 * * * *", "0 0 * * * *"},
		{"0 */6 * * *", "0 0 */6 * * *"},
		{"0 0 * * *", "0 0 0 * * *"},
		{"0 0 * * 1", "0 0 0 * * 1"},
		// already 6-field
		{"0 0 * * * *", "0 0 * * * *"},
		{"0 */30 * * * *", "0 */30 * * * *"},
		// descriptors
		{"@every 30m", "@every 30m"},
		{"@hourly", "@hourly"},
		{"  0 */6 * * *  ", "0 0 */6 * * *"},
	}
	for _, tc := range cases {
		got := normalizeCronExpr(tc.in)
		if got != tc.want {
			t.Fatalf("normalizeCronExpr(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAddScheduleAcceptsFiveFieldCronPresets(t *testing.T) {
	sched := NewUnifiedScheduler(nil, nil)
	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	// Historical UI presets were 5-field; WithSeconds requires 6.
	if err := sched.AddSchedule(ScheduleTypeSpeedTest, "preset-5", "5-field preset", "*/30 * * * *", func() {}); err != nil {
		t.Fatalf("AddSchedule 5-field: %v", err)
	}
	entry := sched.GetEntry("speed_test:preset-5")
	if entry == nil {
		t.Fatal("expected schedule entry")
	}
	if entry.CronExpr != "0 */30 * * * *" {
		t.Fatalf("stored cron = %q, want normalized 6-field", entry.CronExpr)
	}
	if entry.NextRun == nil || entry.NextRun.Before(time.Now()) {
		t.Fatalf("NextRun = %v, want a future time", entry.NextRun)
	}
}
