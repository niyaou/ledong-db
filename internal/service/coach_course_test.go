package service

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestParseCoachCourseMonthUsesShanghaiCalendarBoundaries(t *testing.T) {
	start, endExclusive, err := parseCoachCourseMonth("2026-07")
	if err != nil {
		t.Fatalf("parse month: %v", err)
	}
	if got := start.Format(time.RFC3339); got != "2026-07-01T00:00:00+08:00" {
		t.Fatalf("start = %s", got)
	}
	if got := endExclusive.Format(time.RFC3339); got != "2026-08-01T00:00:00+08:00" {
		t.Fatalf("exclusive end = %s", got)
	}
}

func TestParseCoachCourseMonthRejectsInvalidInput(t *testing.T) {
	for _, month := range []string{"", "2026-7", "2026-00", "2026-13", "2026-07-01"} {
		t.Run(month, func(t *testing.T) {
			_, _, err := parseCoachCourseMonth(month)
			if !errors.Is(err, ErrCoachCourseInvalidMonth) {
				t.Fatalf("error = %v, want ErrCoachCourseInvalidMonth", err)
			}
		})
	}
}

func TestCoachCoursesReturnsCompleteMonthSummaryAndBatchMembers(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, shanghai)
	endExclusive := time.Date(2026, 8, 1, 0, 0, 0, 0, shanghai)

	mock.ExpectQuery("SELECT .* FROM `coach` WHERE .*coach_id = \\? AND is_active = \\? AND deleted_at IS NULL.*LIMIT \\?").
		WithArgs(uint64(12), 1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"coach_id", "name"}).AddRow(12, "张教练"))
	mock.ExpectQuery("SELECT course.id, course.coach_id, course.court_id,.*FROM `course` LEFT JOIN court.*WHERE course.coach_id = \\?.*course.start_time >= \\? AND course.start_time < \\?.*deleted_at.*ORDER BY course.start_time ASC,course.id ASC").
		WithArgs(uint64(12), start, endExclusive).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "coach_id", "court_id", "court_name", "start_time", "end_time",
			"duration", "course_type", "is_adult", "description",
		}).
			AddRow(81, 12, 3, "麓坊校区", time.Date(2026, 7, 1, 9, 0, 0, 0, shanghai), time.Date(2026, 7, 1, 10, 0, 0, 0, shanghai), 1, -2, 1, "体验未成单").
			AddRow(82, 12, 3, "麓坊校区", time.Date(2026, 7, 2, 9, 0, 0, 0, shanghai), time.Date(2026, 7, 2, 9, 30, 0, 0, shanghai), 0.5, -1, 1, "体验成单").
			AddRow(83, 12, 4, "中心校区", time.Date(2026, 7, 3, 9, 0, 0, 0, shanghai), time.Date(2026, 7, 3, 11, 0, 0, 0, shanghai), 2, 0, nil, "订场").
			AddRow(84, 12, 4, "中心校区", time.Date(2026, 7, 4, 9, 0, 0, 0, shanghai), time.Date(2026, 7, 4, 10, 30, 0, 0, shanghai), 1.5, 1, 0, "班课").
			AddRow(85, 12, 4, "中心校区", time.Date(2026, 7, 5, 9, 0, 0, 0, shanghai), time.Date(2026, 7, 5, 11, 0, 0, 0, shanghai), 2, 2, 1, "私教").
			AddRow(86, 12, 4, "中心校区", time.Date(2026, 7, 6, 9, 0, 0, 0, shanghai), time.Date(2026, 7, 6, 10, 0, 0, 0, shanghai), 10, 99, 1, "历史未知类型"))
	mock.ExpectQuery("SELECT cm.course_id, pc.id AS member_id,.*FROM course_member AS cm JOIN prepaid_card AS pc.*LEFT JOIN spend AS s.*WHERE cm.course_id IN \\(\\?,\\?,\\?,\\?,\\?,\\?\\).*ORDER BY cm.course_id ASC,pc.id ASC").
		WithArgs(uint64(81), uint64(82), uint64(83), uint64(84), uint64(85), uint64(86)).
		WillReturnRows(sqlmock.NewRows([]string{
			"course_id", "member_id", "member_name", "member_number", "charge", "times",
			"annual_times", "description", "quantities",
		}).
			AddRow(81, 101, "王会员", "member-101", 0, 1, 0, 200, 1).
			AddRow(84, 102, "李会员", "member-102", 50, 0, 0, 0, 2))

	result, err := (&CourseService{db: db}).CoachCourses(12, "2026-07")
	if err != nil {
		t.Fatalf("CoachCourses: %v", err)
	}
	if result.CoachID != 12 || result.CoachName != "张教练" || result.Month != "2026-07" {
		t.Fatalf("unexpected result identity: %+v", result)
	}
	if result.Summary.TotalHours != 5 || result.Summary.TrialHours != 1.5 || result.Summary.GroupHours != 1.5 || result.Summary.PrivateHours != 2 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if len(result.Courses) != 6 {
		t.Fatalf("course count = %d, want 6", len(result.Courses))
	}
	if result.Courses[0].ID != 81 || result.Courses[5].ID != 86 {
		t.Fatalf("courses not returned in ascending order: %+v", result.Courses)
	}
	if result.Courses[0].StartTime != "2026-07-01 09:00:00" || result.Courses[5].EndTime != "2026-07-06 10:00:00" {
		t.Fatalf("unexpected formatted times: first=%+v last=%+v", result.Courses[0], result.Courses[5])
	}
	if len(result.Courses[0].MembersData) != 1 || result.Courses[0].MembersData[0].MemberID != 101 {
		t.Fatalf("unexpected first course members: %+v", result.Courses[0].MembersData)
	}
	if result.Courses[5].MembersData == nil || len(result.Courses[5].MembersData) != 0 {
		t.Fatalf("course without members must use an empty array: %+v", result.Courses[5].MembersData)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestCoachCoursesRejectsMissingInactiveOrDeletedCoach(t *testing.T) {
	for _, name := range []string{"missing", "inactive", "deleted"} {
		t.Run(name, func(t *testing.T) {
			db, mock := newRemoveCourseTestDB(t)
			mock.ExpectQuery("SELECT .* FROM `coach` WHERE .*coach_id = \\? AND is_active = \\? AND deleted_at IS NULL.*LIMIT \\?").
				WithArgs(uint64(99), 1, 1).
				WillReturnRows(sqlmock.NewRows([]string{"coach_id", "name"}))

			_, err := (&CourseService{db: db}).CoachCourses(99, "2026-07")
			if !errors.Is(err, ErrCoachCourseCoachAbsent) {
				t.Fatalf("error = %v, want ErrCoachCourseCoachAbsent", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet SQL expectations: %v", err)
			}
		})
	}
}

func TestCoachCoursesEmptyMonthDoesNotQueryMembers(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	mock.ExpectQuery("SELECT .* FROM `coach` WHERE .*coach_id = \\? AND is_active = \\? AND deleted_at IS NULL.*LIMIT \\?").
		WithArgs(uint64(12), 1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"coach_id", "name"}).AddRow(12, "张教练"))
	mock.ExpectQuery("SELECT course.id, course.coach_id, course.court_id,.*FROM `course` LEFT JOIN court.*deleted_at.*ORDER BY course.start_time ASC,course.id ASC").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "coach_id", "court_id", "court_name", "start_time", "end_time",
			"duration", "course_type", "is_adult", "description",
		}))

	result, err := (&CourseService{db: db}).CoachCourses(12, "2026-07")
	if err != nil {
		t.Fatalf("CoachCourses: %v", err)
	}
	if result.Courses == nil || len(result.Courses) != 0 || result.Summary != (CoachCourseSummaryDTO{}) {
		t.Fatalf("unexpected empty result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestLoadCoachCourseMembersSkipsQueryForNoCourses(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	members, err := (&CourseService{db: db}).loadCoachCourseMembers(nil)
	if err != nil {
		t.Fatalf("load members: %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("members = %+v, want empty", members)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestCoachCourseMemberQueryIsBatchScoped(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	mock.ExpectQuery(regexp.QuoteMeta("WHERE cm.course_id IN (?,?)")).
		WithArgs(uint64(1), uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{
			"course_id", "member_id", "member_name", "member_number", "charge", "times",
			"annual_times", "description", "quantities",
		}))

	if _, err := (&CourseService{db: db}).loadCoachCourseMembers([]uint64{1, 2}); err != nil {
		t.Fatalf("load members: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
