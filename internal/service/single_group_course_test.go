package service

import (
	"regexp"
	"testing"
	"time"

	"ledong-db/internal/model"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPendingCourseDTOCarriesSingleGroupParticipantCount(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	mock.ExpectQuery("SELECT .*coach_id.*name.*number.* FROM `coach` WHERE coach_id IN \\(\\?\\).*").
		WithArgs(uint64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"coach_id", "name", "number"}).AddRow(12, "张教练", "coach-12"))
	mock.ExpectQuery("SELECT .*id.*name.* FROM `court` WHERE id IN \\(\\?\\).*").
		WithArgs(uint64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(4, "中心校区"))
	now := time.Date(2026, 7, 12, 18, 0, 0, 0, time.Local)

	result, err := (&PendingCourseService{db: db}).buildDTOs([]model.PendingCourse{{
		ID:               9,
		CoachID:          12,
		CourtID:          4,
		StartTime:        now.Add(time.Hour),
		EndTime:          now.Add(2 * time.Hour),
		Duration:         1,
		CourseType:       CourseTypeSingleGroup,
		ParticipantCount: 6,
		MembersData:      model.PendingJSON(`[]`),
		CreatedAt:        now,
		UpdatedAt:        now,
	}})
	if err != nil {
		t.Fatalf("buildDTOs: %v", err)
	}
	if len(result) != 1 || result[0].ParticipantCount != 6 || result[0].CourseType != CourseTypeSingleGroup || len(result[0].MembersData) != 0 {
		t.Fatalf("pending course response = %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestCreateCourseWithParticipantCountValidatesCourseContractBeforeDatabaseAccess(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	service := &CourseService{db: db}

	tests := []struct {
		name             string
		courseType       int
		membersObj       string
		participantCount int
	}{
		{name: "single group requires participants", courseType: CourseTypeSingleGroup},
		{name: "single group rejects members", courseType: CourseTypeSingleGroup, membersObj: `{"member": [0, 1, 0, 200, 1]}`, participantCount: 4},
		{name: "other course rejects participant count", courseType: 1, membersObj: `{}`, participantCount: 4},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.CreateCourseWithParticipantCount(
				"2026-07-12 19:00:00",
				"2026-07-12 20:00:00",
				"coach-12",
				1,
				"中心校区",
				"test",
				test.courseType,
				test.membersObj,
				nil,
				test.participantCount,
			)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("validation unexpectedly accessed database: %v", err)
	}
}

func TestCreateSingleGroupCourseWritesOnlyFormalCourse(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	mock.ExpectQuery("SELECT \\* FROM `coach` WHERE number = \\?.*LIMIT \\?").
		WithArgs("coach-12", 1).
		WillReturnRows(sqlmock.NewRows([]string{"coach_id", "name", "number", "deleted_at"}).AddRow(12, "张教练", "coach-12", nil))
	mock.ExpectQuery("SELECT \\* FROM `court` WHERE name = \\?.*LIMIT \\?").
		WithArgs("中心校区", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "deleted_at"}).AddRow(4, "中心校区", nil))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `course`") + ".*participant_count.*").
		WillReturnResult(sqlmock.NewResult(87, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT \\* FROM `course` WHERE .*start_time >= \\?.*start_time < \\?.*").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	course, err := (&CourseService{db: db}).CreateCourseWithParticipantCount(
		"2026-07-12 19:00:00",
		"2026-07-12 20:00:00",
		"coach-12",
		1,
		"中心校区",
		"单次班课",
		CourseTypeSingleGroup,
		"",
		nil,
		6,
	)
	if err != nil {
		t.Fatalf("CreateCourseWithParticipantCount: %v", err)
	}
	if course.ID != 87 || course.CourseType != CourseTypeSingleGroup || course.ParticipantCount != 6 {
		t.Fatalf("created course = %+v", course)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected spend/member write or unmet course write: %v", err)
	}
}
