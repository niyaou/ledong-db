package service

import (
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newRemoveCourseTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	mock.MatchExpectationsInOrder(false)

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open GORM test database: %v", err)
	}

	return db, mock
}

func expectRemoveCourseReads(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("SELECT \\* FROM `course`.*FOR UPDATE").
		WithArgs(uint64(10), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "deleted_at"}).AddRow(10, nil))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `spend` WHERE `spend`.`course_id` = ? AND `spend`.`deleted_at` IS NULL")).
		WithArgs(uint64(10)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "charge", "times", "annual_times", "description", "quantities",
			"prepaid_card_id", "course_id", "deleted_at",
		}).
			AddRow(101, 30, 1.5, 0, 30, 1, 1, 10, nil).
			AddRow(102, 0, 0, 2.5, 200.5, 1, 2, 10, nil))
	mock.ExpectQuery("SELECT \\* FROM `prepaid_card` WHERE `prepaid_card`.`id` IN \\(\\?,\\?\\) AND `prepaid_card`.`deleted_at` IS NULL").
		WithArgs(uint64(1), uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "number", "rest_charge", "annual_count", "times_count",
			"equivalent_balance", "deleted_at",
		}).
			AddRow(1, "member one", "10001", 100, 3, 2, 50, nil).
			AddRow(2, "member two", "10002", 20, 4, 1, 10, nil))
}

func expectRemoveCourseWrites(mock sqlmock.Sqlmock) {
	mock.ExpectExec("UPDATE `prepaid_card` SET .* WHERE id = \\? AND `prepaid_card`.`deleted_at` IS NULL").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `prepaid_card` SET .* WHERE id = \\? AND `prepaid_card`.`deleted_at` IS NULL").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `spend` SET `deleted_at`=\\? WHERE course_id = \\? AND `spend`.`deleted_at` IS NULL").
		WithArgs(sqlmock.AnyArg(), uint64(10)).
		WillReturnResult(sqlmock.NewResult(0, 2))
}

func TestRemoveCourseRestoresBalancesAndSoftDeletesActiveSpends(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	mock.ExpectBegin()
	expectRemoveCourseReads(mock)
	expectRemoveCourseWrites(mock)
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM course_member WHERE course_id = ?")).
		WithArgs(uint64(10)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE `course` SET `deleted_at`=\\? WHERE `course`.`id` = \\? AND `course`.`deleted_at` IS NULL").
		WithArgs(sqlmock.AnyArg(), uint64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	removed, err := (&CourseService{db: db}).RemoveCourse(10)
	if err != nil {
		t.Fatalf("RemoveCourse returned error: %v", err)
	}
	if removed.ID != 10 || !removed.DeletedAt.Valid {
		t.Fatalf("removed course = %+v, want soft-deleted course 10", removed)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestRemoveCourseRollsBackAllChangesWhenMemberCleanupFails(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	mock.ExpectBegin()
	expectRemoveCourseReads(mock)
	expectRemoveCourseWrites(mock)
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM course_member WHERE course_id = ?")).
		WithArgs(uint64(10)).
		WillReturnError(errors.New("member cleanup failed"))
	mock.ExpectRollback()

	if _, err := (&CourseService{db: db}).RemoveCourse(10); err == nil {
		t.Fatal("RemoveCourse returned nil error, want member cleanup failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
