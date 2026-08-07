package service

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"ledong-db/internal/model"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"
)

func TestListSubmissionsPaginatesUnionAndBatchLoadsRechargeIdentity(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	businessDate := time.Date(2026, 8, 7, 0, 0, 0, 0, shanghai)
	createdAt := time.Date(2026, 8, 7, 9, 0, 0, 0, shanghai)
	updatedAt := time.Date(2026, 8, 7, 9, 10, 0, 0, shanghai)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT\n\t\t(SELECT COUNT(*) FROM pending_course) +\n\t\t(SELECT COUNT(*) FROM coach_recharge_notice WHERE status = ?) AS total")).
		WithArgs(model.RechargeNoticeStatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
	mock.ExpectQuery("SELECT submission_type, id, business_date, submitted_at FROM .*UNION ALL.*ORDER BY business_date DESC, submitted_at DESC, id DESC, submission_type DESC.*LIMIT \\? OFFSET \\?").
		WithArgs(model.RechargeNoticeStatusPending, 30, 0).
		WillReturnRows(sqlmock.NewRows([]string{"submission_type", "id", "business_date", "submitted_at"}).
			AddRow("RECHARGE_NOTICE", 7, businessDate, updatedAt))
	mock.ExpectQuery("SELECT \\* FROM `coach_recharge_notice` WHERE id IN \\(\\?\\)").
		WithArgs(uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "coach_id", "member_id", "recharge_date", "note", "status", "version", "created_at", "updated_at", "acknowledged_at"}).
			AddRow(7, 12, 99, businessDate, "线下充值", model.RechargeNoticeStatusPending, 2, createdAt, updatedAt, nil))
	mock.ExpectQuery("SELECT `coach_id`,`name`,`is_active`,`deleted_at` FROM `coach` WHERE coach_id IN \\(\\?\\)").
		WithArgs(uint64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"coach_id", "name", "is_active", "deleted_at"}).AddRow(12, "张教练", 1, nil))
	mock.ExpectQuery("SELECT `id`,`name`,`number`,`deleted_at` FROM `prepaid_card` WHERE id IN \\(\\?\\)").
		WithArgs(uint64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "number", "deleted_at"}).AddRow(99, "王会员", "M099", nil))

	svc := &RechargeNoticeService{db: db, pendingCourseService: &PendingCourseService{db: db}}
	page, err := svc.ListSubmissions(1, 30)
	if err != nil {
		t.Fatalf("ListSubmissions: %v", err)
	}
	if page.TotalElements != 1 || page.Number != 0 || len(page.Content) != 1 {
		t.Fatalf("unexpected page: %+v", page)
	}
	submission := page.Content[0]
	if submission.SubmissionType != "RECHARGE_NOTICE" || submission.BusinessDate != "2026-08-07" || submission.SubmittedAt != "2026-08-07 09:10:00" {
		t.Fatalf("unexpected submission envelope: %+v", submission)
	}
	if submission.Course != nil || submission.RechargeNotice == nil || submission.RechargeNotice.MemberNumber != "M099" || !submission.RechargeNotice.CoachActive || !submission.RechargeNotice.MemberActive {
		t.Fatalf("unexpected exclusive payload: %+v", submission)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestAcknowledgeUsesConditionalVersionAndPreservesContentTimestamp(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	now := time.Date(2026, 8, 7, 9, 10, 0, 0, time.Local)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE coach_recharge_notice\n\t\tSET status = ?, acknowledged_at = NOW(), updated_at = updated_at\n\t\tWHERE id = ? AND status = ? AND version = ?")).
		WithArgs(model.RechargeNoticeStatusAcknowledged, uint64(7), model.RechargeNoticeStatusPending, uint64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT \\* FROM `coach_recharge_notice` WHERE `coach_recharge_notice`.`id` = \\? ORDER BY `coach_recharge_notice`.`id` LIMIT \\?").
		WithArgs(uint64(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "coach_id", "member_id", "recharge_date", "note", "status", "version", "created_at", "updated_at", "acknowledged_at"}).
			AddRow(7, 12, 99, now, "线下充值", model.RechargeNoticeStatusAcknowledged, 3, now, now, now))
	mock.ExpectQuery("SELECT `coach_id`,`name`,`is_active`,`deleted_at` FROM `coach` WHERE coach_id IN \\(\\?\\)").
		WithArgs(uint64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"coach_id", "name", "is_active", "deleted_at"}).AddRow(12, "张教练", 0, nil))
	mock.ExpectQuery("SELECT `id`,`name`,`number`,`deleted_at` FROM `prepaid_card` WHERE id IN \\(\\?\\)").
		WithArgs(uint64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "number", "deleted_at"}).AddRow(99, "王会员", "M099", now))

	dto, err := (&RechargeNoticeService{db: db}).Acknowledge(7, 3)
	if err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}
	if dto.Status != model.RechargeNoticeStatusAcknowledged || dto.Version != 3 || dto.CoachActive || dto.MemberActive || dto.AcknowledgedAt == nil {
		t.Fatalf("unexpected acknowledged dto: %+v", dto)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestListAcknowledgedNoticesUsesStableHistoryOrderAndZeroBasedPage(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	now := time.Date(2026, 8, 7, 9, 10, 0, 0, shanghai)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `coach_recharge_notice` WHERE status = ? AND recharge_date >= ? AND recharge_date <= ?")).
		WithArgs(model.RechargeNoticeStatusAcknowledged, start, end).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(31))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `coach_recharge_notice` WHERE status = ? AND recharge_date >= ? AND recharge_date <= ? ORDER BY recharge_date DESC,updated_at DESC,id DESC LIMIT ? OFFSET ?")).
		WithArgs(model.RechargeNoticeStatusAcknowledged, start, end, 30, 30).
		WillReturnRows(sqlmock.NewRows([]string{"id", "coach_id", "member_id", "recharge_date", "note", "status", "version", "created_at", "updated_at", "acknowledged_at"}).
			AddRow(7, 12, 99, now, "线下充值", model.RechargeNoticeStatusAcknowledged, 3, now, now, now))
	mock.ExpectQuery("SELECT `coach_id`,`name`,`is_active`,`deleted_at` FROM `coach`").
		WillReturnRows(sqlmock.NewRows([]string{"coach_id", "name", "is_active", "deleted_at"}).AddRow(12, "张教练", 1, nil))
	mock.ExpectQuery("SELECT `id`,`name`,`number`,`deleted_at` FROM `prepaid_card`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "number", "deleted_at"}).AddRow(99, "王会员", "M099", nil))

	page, err := (&RechargeNoticeService{db: db}).ListNotices(model.RechargeNoticeStatusAcknowledged, "2026-08-01", "2026-08-31", 2, 30)
	if err != nil {
		t.Fatalf("ListNotices: %v", err)
	}
	if page.Number != 1 || page.TotalPages != 2 || page.First || !page.Last || len(page.Content) != 1 {
		t.Fatalf("unexpected page: %+v", page)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestAcknowledgeIsIdempotentForSameAcknowledgedVersion(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	now := time.Date(2026, 8, 7, 9, 10, 0, 0, time.Local)
	mock.ExpectExec("UPDATE coach_recharge_notice").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT \\* FROM `coach_recharge_notice`.*LIMIT \\?").
		WithArgs(uint64(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "coach_id", "member_id", "recharge_date", "note", "status", "version", "created_at", "updated_at", "acknowledged_at"}).
			AddRow(7, 12, 99, now, "线下充值", model.RechargeNoticeStatusAcknowledged, 3, now, now, now))
	mock.ExpectQuery("SELECT `coach_id`,`name`,`is_active`,`deleted_at` FROM `coach`").
		WillReturnRows(sqlmock.NewRows([]string{"coach_id", "name", "is_active", "deleted_at"}).AddRow(12, "张教练", 1, nil))
	mock.ExpectQuery("SELECT `id`,`name`,`number`,`deleted_at` FROM `prepaid_card`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "number", "deleted_at"}).AddRow(99, "王会员", "M099", nil))

	if _, err := (&RechargeNoticeService{db: db}).Acknowledge(7, 3); err != nil {
		t.Fatalf("same acknowledged version must be idempotent: %v", err)
	}
}

func TestAcknowledgeRejectsStaleVersion(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	now := time.Date(2026, 8, 7, 9, 10, 0, 0, time.Local)
	mock.ExpectExec("UPDATE coach_recharge_notice").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT \\* FROM `coach_recharge_notice`.*LIMIT \\?").
		WithArgs(uint64(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "coach_id", "member_id", "recharge_date", "note", "status", "version", "created_at", "updated_at", "acknowledged_at"}).
			AddRow(7, 12, 99, now, "教练已修改", model.RechargeNoticeStatusPending, 4, now, now, nil))

	_, err := (&RechargeNoticeService{db: db}).Acknowledge(7, 3)
	var noticeErr *RechargeNoticeError
	if !errors.As(err, &noticeErr) || noticeErr.Code != RechargeNoticeErrorVersionConflict {
		t.Fatalf("error = %v, want version conflict", err)
	}
}

func TestApplyRechargeDateRangeRejectsReverseOrInvalidRange(t *testing.T) {
	db, _ := newRemoveCourseTestDB(t)
	for _, tc := range []struct {
		start string
		end   string
	}{
		{start: "2026/08/01"},
		{end: "2026-08-40"},
		{start: "2026-08-08", end: "2026-08-07"},
	} {
		_, err := applyRechargeDateRange(db.Model(&model.RechargeNotice{}), tc.start, tc.end)
		var noticeErr *RechargeNoticeError
		if !errors.As(err, &noticeErr) || noticeErr.Code != RechargeNoticeErrorInvalidRequest {
			t.Fatalf("range %+v error = %v", tc, err)
		}
	}
}

func TestAcknowledgeReturnsNotFound(t *testing.T) {
	db, mock := newRemoveCourseTestDB(t)
	mock.ExpectExec("UPDATE coach_recharge_notice").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT \\* FROM `coach_recharge_notice`.*LIMIT \\?").
		WithArgs(uint64(7), 1).
		WillReturnError(gorm.ErrRecordNotFound)
	_, err := (&RechargeNoticeService{db: db}).Acknowledge(7, 3)
	var noticeErr *RechargeNoticeError
	if !errors.As(err, &noticeErr) || noticeErr.Code != RechargeNoticeErrorNotFound {
		t.Fatalf("error = %v, want not found", err)
	}
}
