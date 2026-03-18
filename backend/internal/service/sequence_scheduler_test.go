package service

import (
	"testing"
	"time"

	"github.com/fastcrm/backend/internal/entity"
)

// makeSeq is a helper for building a minimal Sequence with business hours fields.
func makeSeq(tz, bhStart, bhEnd string) *entity.Sequence {
	s := &entity.Sequence{
		ID:       "seq1",
		OrgID:    "org1",
		Timezone: tz,
	}
	if bhStart != "" {
		s.BusinessHoursStart = &bhStart
	}
	if bhEnd != "" {
		s.BusinessHoursEnd = &bhEnd
	}
	return s
}

// fixedTime constructs a time.Time in the given location at the given hour and minute.
// weekday: time.Tuesday, time.Saturday, etc.
func fixedTime(loc *time.Location, weekday time.Weekday, hour, min int) time.Time {
	// Find a concrete date that matches the target weekday.
	// Start from a known Monday: 2024-01-01.
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, loc) // Monday
	daysAhead := (int(weekday) - int(base.Weekday()) + 7) % 7
	target := base.AddDate(0, 0, daysAhead)
	return time.Date(target.Year(), target.Month(), target.Day(), hour, min, 0, 0, loc)
}

func TestIsWithinBusinessHours_WithinWindow(t *testing.T) {
	// 10:30 AM on Tuesday with 09:00-17:00 window => true
	loc := time.UTC
	seq := makeSeq("UTC", "09:00", "17:00")
	ts := fixedTime(loc, time.Tuesday, 10, 30)

	got := isWithinBusinessHoursAt(seq, ts)
	if !got {
		t.Errorf("expected true for 10:30 Tuesday, got false")
	}
}

func TestIsWithinBusinessHours_OutsideWindow(t *testing.T) {
	// 20:00 on Tuesday with 09:00-17:00 window => false
	loc := time.UTC
	seq := makeSeq("UTC", "09:00", "17:00")
	ts := fixedTime(loc, time.Tuesday, 20, 0)

	got := isWithinBusinessHoursAt(seq, ts)
	if got {
		t.Errorf("expected false for 20:00 Tuesday, got true")
	}
}

func TestIsWithinBusinessHours_Weekend(t *testing.T) {
	// 10:00 on Saturday with 09:00-17:00 window => false
	loc := time.UTC
	seq := makeSeq("UTC", "09:00", "17:00")
	ts := fixedTime(loc, time.Saturday, 10, 0)

	got := isWithinBusinessHoursAt(seq, ts)
	if got {
		t.Errorf("expected false for 10:00 Saturday, got true")
	}
}

func TestIsWithinBusinessHours_TimezoneConversion(t *testing.T) {
	// 14:00 UTC with America/New_York 09:00-17:00 => true (10:00 ET)
	// Note: America/New_York is UTC-4 (EDT) or UTC-5 (EST).
	// In January the offset is EST = UTC-5, so 14:00 UTC = 09:00 EST.
	// We use a date in January to ensure it's 09:00 ET, which is exactly the
	// start of business hours. Since our check is >= start, this is true.
	nyLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("could not load America/New_York: %v", err)
	}
	seq := makeSeq("America/New_York", "09:00", "17:00")

	// 14:00 UTC on a Tuesday in January (no DST, EST = UTC-5)
	// 14:00 UTC = 09:00 EST
	ts := time.Date(2024, 1, 2, 14, 0, 0, 0, time.UTC) // 2024-01-02 is a Tuesday
	_ = nyLoc

	got := isWithinBusinessHoursAt(seq, ts)
	if !got {
		t.Errorf("expected true for 14:00 UTC (09:00 New York) on Tuesday, got false")
	}
}

func TestNextBusinessHoursStart_SameDay(t *testing.T) {
	// 07:00 on Tuesday with 09:00 start => returns 09:00 same day
	loc := time.UTC
	seq := makeSeq("UTC", "09:00", "17:00")
	from := fixedTime(loc, time.Tuesday, 7, 0)

	got := nextBusinessHoursStartAt(seq, from)
	if got.Weekday() != time.Tuesday {
		t.Errorf("expected Tuesday, got %v", got.Weekday())
	}
	if got.Hour() != 9 || got.Minute() != 0 {
		t.Errorf("expected 09:00, got %02d:%02d", got.Hour(), got.Minute())
	}
}

func TestNextBusinessHoursStart_NextDay(t *testing.T) {
	// 18:00 on Tuesday with 09:00 start => returns 09:00 Wednesday
	loc := time.UTC
	seq := makeSeq("UTC", "09:00", "17:00")
	from := fixedTime(loc, time.Tuesday, 18, 0)

	got := nextBusinessHoursStartAt(seq, from)
	if got.Weekday() != time.Wednesday {
		t.Errorf("expected Wednesday, got %v", got.Weekday())
	}
	if got.Hour() != 9 || got.Minute() != 0 {
		t.Errorf("expected 09:00, got %02d:%02d", got.Hour(), got.Minute())
	}
}

func TestNextBusinessHoursStart_SkipsWeekend(t *testing.T) {
	// 18:00 on Friday with 09:00 start => returns 09:00 Monday
	loc := time.UTC
	seq := makeSeq("UTC", "09:00", "17:00")
	from := fixedTime(loc, time.Friday, 18, 0)

	got := nextBusinessHoursStartAt(seq, from)
	if got.Weekday() != time.Monday {
		t.Errorf("expected Monday, got %v", got.Weekday())
	}
	if got.Hour() != 9 || got.Minute() != 0 {
		t.Errorf("expected 09:00, got %02d:%02d", got.Hour(), got.Minute())
	}
}
