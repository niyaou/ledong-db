package service

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestParseCoachCourseDateRangeUsesShanghaiAndInclusiveEndDate(t *testing.T) {
	start, endExclusive, err := parseCoachCourseDateRange("2026-05-01", "2026-07-31")
	if err != nil {
		t.Fatalf("parse date range: %v", err)
	}
	if got := start.Format(time.RFC3339); got != "2026-05-01T00:00:00+08:00" {
		t.Fatalf("start = %s", got)
	}
	if got := endExclusive.Format(time.RFC3339); got != "2026-08-01T00:00:00+08:00" {
		t.Fatalf("exclusive end = %s", got)
	}
}

func TestParseCoachCourseDateRangeRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name      string
		startDate string
		endDate   string
		want      error
	}{
		{name: "missing start", startDate: "", endDate: "2026-07-31", want: ErrCoachCourseInvalidDate},
		{name: "invalid end", startDate: "2026-05-01", endDate: "2026-07-32", want: ErrCoachCourseInvalidDate},
		{name: "reverse range", startDate: "2026-08-01", endDate: "2026-07-31", want: ErrCoachCourseDateRange},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := parseCoachCourseDateRange(test.startDate, test.endDate)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestCoachCoursesReturnsPagedReadOnlyDTOWithBatchMembers(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, shanghai)
	endExclusive := time.Date(2026, 8, 1, 0, 0, 0, 0, shanghai)

	mock.ExpectQuery("SELECT .* FROM `coach` WHERE .*coach_id = \\? AND is_active = \\? AND deleted_at IS NULL.*LIMIT \\?").
		WithArgs(uint64(12), 1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"coach_id", "name"}).AddRow(12, "张教练"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `course` WHERE course.coach_id = ? AND (course.start_time >= ? AND course.start_time < ?) AND course.deleted_at IS NULL AND `course`.`deleted_at` IS NULL")).
		WithArgs(uint64(12), start, endExclusive).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(51))
	mock.ExpectQuery("SELECT course.id, course.coach_id, course.court_id,.*FROM `course` LEFT JOIN court.*WHERE course.coach_id = \\?.*ORDER BY course.start_time DESC,course.id DESC LIMIT \\? OFFSET \\?").
		WithArgs(uint64(12), start, endExclusive, CoachCoursePageSize, CoachCoursePageSize).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "coach_id", "court_id", "court_name", "start_time", "end_time",
			"duration", "course_type", "is_adult", "description",
		}).AddRow(88, 12, 3, "麓坊校区",
			time.Date(2026, 7, 31, 23, 30, 0, 0, shanghai),
			time.Date(2026, 7, 31, 23, 59, 0, 0, shanghai),
			0.5, 2, 1, "夜间私教"))
	mock.ExpectQuery("SELECT cm.course_id, pc.id AS member_id,.*FROM course_member AS cm JOIN prepaid_card AS pc.*LEFT JOIN spend AS s.*WHERE cm.course_id IN \\(\\?\\).*ORDER BY cm.course_id ASC,pc.id ASC").
		WithArgs(uint64(88)).
		WillReturnRows(sqlmock.NewRows([]string{
			"course_id", "member_id", "member_name", "member_number", "charge", "times",
			"annual_times", "description", "quantities",
		}).AddRow(88, 101, "王会员", "member-101", 0, 1.5, 0, 200, 2))

	page, err := (&CourseService{db: db}).CoachCourses(12, "2026-05-01", "2026-07-31", 2)
	if err != nil {
		t.Fatalf("CoachCourses: %v", err)
	}
	if page.TotalElements != 51 || page.TotalPages != 2 || page.Size != 50 || page.Number != 1 || page.First || !page.Last {
		t.Fatalf("unexpected page metadata: %+v", page)
	}
	if page.NumberOfElements != 1 || len(page.Content) != 1 {
		t.Fatalf("unexpected page content count: %+v", page)
	}
	course := page.Content[0]
	if course.ID != 88 || course.CoachID != 12 || course.CoachName != "张教练" || course.CourtName != "麓坊校区" {
		t.Fatalf("unexpected course identity: %+v", course)
	}
	if course.StartTime != "2026-07-31 23:30:00" || course.EndTime != "2026-07-31 23:59:00" {
		t.Fatalf("unexpected course time: %+v", course)
	}
	if len(course.MembersData) != 1 || course.MembersData[0].MemberID != 101 || course.MembersData[0].Times != 1.5 || course.MembersData[0].Quantities != 2 {
		t.Fatalf("unexpected member consumption: %+v", course.MembersData)
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

			_, err := (&CourseService{db: db}).CoachCourses(99, "2026-05-01", "2026-07-31", 1)
			if !errors.Is(err, ErrCoachCourseCoachAbsent) {
				t.Fatalf("error = %v, want ErrCoachCourseCoachAbsent", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet SQL expectations: %v", err)
			}
		})
	}
}

func TestCoachCoursesEmptyPageDoesNotQueryMembers(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	mock.ExpectQuery("SELECT .* FROM `coach` WHERE .*coach_id = \\? AND is_active = \\? AND deleted_at IS NULL.*LIMIT \\?").
		WithArgs(uint64(12), 1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"coach_id", "name"}).AddRow(12, "张教练"))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `course` WHERE course.coach_id = \\?.*").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(0))
	mock.ExpectQuery("SELECT course.id, course.coach_id, course.court_id,.*FROM `course` LEFT JOIN court.*LIMIT \\?").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "coach_id", "court_id", "court_name", "start_time", "end_time",
			"duration", "course_type", "is_adult", "description",
		}))

	page, err := (&CourseService{db: db}).CoachCourses(12, "2026-05-01", "2026-07-31", 1)
	if err != nil {
		t.Fatalf("CoachCourses: %v", err)
	}
	if page.TotalPages != 0 || !page.First || !page.Last || page.Content == nil || len(page.Content) != 0 {
		t.Fatalf("unexpected empty page: %+v", page)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
