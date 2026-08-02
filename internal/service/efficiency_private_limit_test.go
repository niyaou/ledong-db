package service

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPrivateCourseMonthlyLimitIncludesFirstHundredAndExcludesRest(t *testing.T) {
	counts := make(map[coachMonthKey]int)
	lessonTime := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.Local)

	for i := 1; i <= CoachMonthlyPrivateLimit; i++ {
		if privateCourseExceedsMonthlyLimit(counts, 42, lessonTime, 2) {
			t.Fatalf("private course %d was excluded; first %d must be included", i, CoachMonthlyPrivateLimit)
		}
	}

	if !privateCourseExceedsMonthlyLimit(counts, 42, lessonTime, 2) {
		t.Fatalf("private course %d must be excluded", CoachMonthlyPrivateLimit+1)
	}
	if !privateCourseExceedsMonthlyLimit(counts, 42, lessonTime, 2) {
		t.Fatalf("courses after the limit must remain excluded")
	}
}

func TestPrivateCourseMonthlyLimitAppliesToEveryCoachIndependently(t *testing.T) {
	counts := make(map[coachMonthKey]int)
	lessonTime := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.Local)

	for i := 0; i < CoachMonthlyPrivateLimit; i++ {
		privateCourseExceedsMonthlyLimit(counts, 1, lessonTime, 2)
	}

	if !privateCourseExceedsMonthlyLimit(counts, 1, lessonTime, 2) {
		t.Fatal("coach 1 must be over the monthly limit")
	}
	if privateCourseExceedsMonthlyLimit(counts, 2, lessonTime, 2) {
		t.Fatal("coach 2 must have an independent monthly limit")
	}
}

func TestPrivateCourseMonthlyLimitResetsForMonthAndYear(t *testing.T) {
	counts := make(map[coachMonthKey]int)
	july2026 := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.Local)

	for i := 0; i < CoachMonthlyPrivateLimit; i++ {
		privateCourseExceedsMonthlyLimit(counts, 7, july2026, 2)
	}

	if privateCourseExceedsMonthlyLimit(counts, 7, july2026.AddDate(0, 1, 0), 2) {
		t.Fatal("a new month must start a new limit")
	}
	if privateCourseExceedsMonthlyLimit(counts, 7, july2026.AddDate(1, 0, 0), 2) {
		t.Fatal("the same month number in a new year must start a new limit")
	}
}

func TestNonPrivateAndUnassignedCoursesDoNotConsumePrivateLimit(t *testing.T) {
	counts := make(map[coachMonthKey]int)
	lessonTime := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.Local)

	for i := 0; i < CoachMonthlyPrivateLimit+10; i++ {
		if privateCourseExceedsMonthlyLimit(counts, 9, lessonTime, 1) {
			t.Fatal("group courses must never be excluded by the private-course limit")
		}
		if privateCourseExceedsMonthlyLimit(counts, 0, lessonTime, 2) {
			t.Fatal("courses without a coach must not use a coach limit")
		}
	}

	for i := 1; i <= CoachMonthlyPrivateLimit; i++ {
		if privateCourseExceedsMonthlyLimit(counts, 9, lessonTime, 2) {
			t.Fatalf("private course %d was excluded after unrelated courses", i)
		}
	}
}

func TestCalendarMonthStartPreservesLocation(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	input := time.Date(2026, time.July, 18, 15, 30, 45, 123, shanghai)

	got := calendarMonthStart(input)

	want := time.Date(2026, time.July, 1, 0, 0, 0, 0, shanghai)
	if !got.Equal(want) || got.Location() != shanghai {
		t.Fatalf("calendarMonthStart() = %v (%v), want %v (%v)", got, got.Location(), want, shanghai)
	}
}

func TestGetCourseStatsAppliesSharedMonthlyLimitToCoachAndCampus(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	reportStart := time.Date(2026, time.July, 15, 9, 0, 0, 0, shanghai)
	reportEnd := time.Date(2026, time.July, 31, 23, 59, 59, 0, shanghai)
	rows := sqlmock.NewRows([]string{
		"coach_name", "court_name", "coach_id", "start_time",
		"course_type", "duration", "quantities", "spend",
	})

	// Sixty earlier private courses consume the same coach's allowance without
	// contributing to the requested partial-month report.
	for i := 1; i <= 60; i++ {
		startTime := time.Date(2026, time.July, 1, 9, i, 0, 0, shanghai)
		rows.AddRow("coach", "campus A", 8, startTime, 2, 1, 1, 0)
	}
	// Of the 41 in-range private courses at another campus, the first 40 bring
	// the monthly total to 100 and the final course is excluded.
	for i := 0; i < 41; i++ {
		startTime := reportStart.Add(time.Duration(i) * time.Minute)
		rows.AddRow("coach", "campus B", 8, startTime, 2, 1, 1, 0)
	}

	mock.ExpectQuery(regexp.QuoteMeta("FROM `course`")+".*"+regexp.QuoteMeta("ORDER BY course.start_time, course.id")).
		WithArgs(calendarMonthStart(reportStart), reportEnd).
		WillReturnRows(rows)

	stats, err := (&EfficiencyService{db: db}).getCourseStats(reportStart, reportEnd)
	if err != nil {
		t.Fatalf("getCourseStats: %v", err)
	}

	var coachStat, campusStat *courseStat
	for i := range stats {
		switch {
		case stats[i].CoachName == "coach":
			coachStat = &stats[i]
		case stats[i].CourtName == "campus B":
			campusStat = &stats[i]
		}
	}
	if coachStat == nil || campusStat == nil {
		t.Fatalf("missing coach or campus stats: %+v", stats)
	}
	for label, stat := range map[string]*courseStat{"coach": coachStat, "campus": campusStat} {
		if stat.Courses != 41 || stat.Members != 82 {
			t.Fatalf("%s raw totals = courses %.0f, members %.0f; want 41 and 82", label, stat.Courses, stat.Members)
		}
		if stat.TruncatedCourses != 40 || stat.TruncatedMembers != 80 {
			t.Fatalf("%s adjusted totals = courses %.0f, members %.0f; want 40 and 80", label, stat.TruncatedCourses, stat.TruncatedMembers)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
