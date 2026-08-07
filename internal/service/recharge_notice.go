package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"ledong-db/internal/database"
	"ledong-db/internal/model"

	"gorm.io/gorm"
)

const (
	RechargeNoticeErrorInvalidRequest  = "RECHARGE_NOTICE_INVALID_REQUEST"
	RechargeNoticeErrorUnauthorized    = "RECHARGE_NOTICE_UNAUTHORIZED"
	RechargeNoticeErrorNotFound        = "RECHARGE_NOTICE_NOT_FOUND"
	RechargeNoticeErrorVersionConflict = "RECHARGE_NOTICE_VERSION_CONFLICT"
	RechargeNoticeErrorInternal        = "INTERNAL_ERROR"

	rechargeNoticeDateFormat = "2006-01-02"
	rechargeNoticeTimeFormat = "2006-01-02 15:04:05"
)

type RechargeNoticeError struct {
	Code    string
	Message string
	Cause   error
}

func (e *RechargeNoticeError) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Cause)
}

func (e *RechargeNoticeError) Unwrap() error { return e.Cause }

type RechargeNoticeDTO struct {
	ID             uint64  `json:"id"`
	CoachID        uint64  `json:"coachId"`
	CoachName      string  `json:"coachName"`
	CoachActive    bool    `json:"coachActive"`
	MemberID       uint64  `json:"memberId"`
	MemberName     string  `json:"memberName"`
	MemberNumber   string  `json:"memberNumber"`
	MemberActive   bool    `json:"memberActive"`
	RechargeDate   string  `json:"rechargeDate"`
	Note           string  `json:"note"`
	Status         string  `json:"status"`
	Version        uint64  `json:"version"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
	AcknowledgedAt *string `json:"acknowledgedAt"`
}

type CoachSubmission struct {
	SubmissionType string             `json:"submissionType"`
	BusinessDate   string             `json:"businessDate"`
	SubmittedAt    string             `json:"submittedAt"`
	Course         *PendingCourseDTO  `json:"course,omitempty"`
	RechargeNotice *RechargeNoticeDTO `json:"rechargeNotice,omitempty"`
}

type CoachSubmissionPage = Page[CoachSubmission]
type RechargeNoticePage = Page[RechargeNoticeDTO]

type submissionIndexRow struct {
	SubmissionType string    `gorm:"column:submission_type"`
	ID             uint64    `gorm:"column:id"`
	BusinessDate   time.Time `gorm:"column:business_date"`
	SubmittedAt    time.Time `gorm:"column:submitted_at"`
}

type RechargeNoticeService struct {
	db                   *gorm.DB
	pendingCourseService *PendingCourseService
}

func NewRechargeNoticeService(pendingCourseService *PendingCourseService) *RechargeNoticeService {
	return &RechargeNoticeService{db: database.DB, pendingCourseService: pendingCourseService}
}

func (s *RechargeNoticeService) ListSubmissions(pageNum, pageSize int) (*Page[CoachSubmission], error) {
	if pageNum < 1 || pageSize < 1 || pageSize > 100 {
		return nil, newRechargeNoticeError(RechargeNoticeErrorInvalidRequest, "分页参数错误", nil)
	}

	var total int64
	countSQL := `SELECT
		(SELECT COUNT(*) FROM pending_course) +
		(SELECT COUNT(*) FROM coach_recharge_notice WHERE status = ?) AS total`
	if err := s.db.Raw(countSQL, model.RechargeNoticeStatusPending).Scan(&total).Error; err != nil {
		return nil, newRechargeNoticeError(RechargeNoticeErrorInternal, "查询教练填报总数失败", err)
	}

	rows := make([]submissionIndexRow, 0, pageSize)
	querySQL := `SELECT submission_type, id, business_date, submitted_at FROM (
		SELECT 'COURSE' AS submission_type, id, DATE(start_time) AS business_date, updated_at AS submitted_at
		FROM pending_course
		UNION ALL
		SELECT 'RECHARGE_NOTICE' AS submission_type, id, recharge_date AS business_date, updated_at AS submitted_at
		FROM coach_recharge_notice WHERE status = ?
	) AS submissions
	ORDER BY business_date DESC, submitted_at DESC, id DESC, submission_type DESC
	LIMIT ? OFFSET ?`
	if err := s.db.Raw(querySQL, model.RechargeNoticeStatusPending, pageSize, (pageNum-1)*pageSize).Scan(&rows).Error; err != nil {
		return nil, newRechargeNoticeError(RechargeNoticeErrorInternal, "查询教练填报失败", err)
	}

	courseIDs := make([]uint64, 0, len(rows))
	noticeIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		switch row.SubmissionType {
		case "COURSE":
			courseIDs = append(courseIDs, row.ID)
		case "RECHARGE_NOTICE":
			noticeIDs = append(noticeIDs, row.ID)
		}
	}

	courseMap := make(map[uint64]PendingCourseDTO, len(courseIDs))
	if len(courseIDs) > 0 {
		courses, err := s.pendingCourseService.listByIDs(courseIDs)
		if err != nil {
			return nil, err
		}
		for _, course := range courses {
			courseMap[course.ID] = course
		}
	}
	noticeMap, err := s.loadNoticeDTOMap(noticeIDs)
	if err != nil {
		return nil, err
	}

	content := make([]CoachSubmission, 0, len(rows))
	for _, row := range rows {
		submission := CoachSubmission{
			SubmissionType: row.SubmissionType,
			BusinessDate:   row.BusinessDate.Format(rechargeNoticeDateFormat),
			SubmittedAt:    row.SubmittedAt.Format(rechargeNoticeTimeFormat),
		}
		switch row.SubmissionType {
		case "COURSE":
			course, ok := courseMap[row.ID]
			if !ok {
				continue
			}
			submission.Course = &course
		case "RECHARGE_NOTICE":
			notice, ok := noticeMap[row.ID]
			if !ok {
				continue
			}
			submission.RechargeNotice = &notice
		default:
			continue
		}
		content = append(content, submission)
	}
	return newPage(content, total, pageNum, pageSize), nil
}

func (s *RechargeNoticeService) ListNotices(status, startDate, endDate string, pageNum, pageSize int) (*Page[RechargeNoticeDTO], error) {
	if pageNum < 1 || pageSize < 1 || pageSize > 100 {
		return nil, newRechargeNoticeError(RechargeNoticeErrorInvalidRequest, "分页参数错误", nil)
	}
	status = strings.ToUpper(strings.TrimSpace(status))
	if status != model.RechargeNoticeStatusPending && status != model.RechargeNoticeStatusAcknowledged {
		return nil, newRechargeNoticeError(RechargeNoticeErrorInvalidRequest, "status必须是PENDING或ACKNOWLEDGED", nil)
	}

	query := s.db.Model(&model.RechargeNotice{}).Where("status = ?", status)
	var err error
	query, err = applyRechargeDateRange(query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, newRechargeNoticeError(RechargeNoticeErrorInternal, "查询充值待办总数失败", err)
	}
	var notices []model.RechargeNotice
	if err := query.Order("recharge_date DESC").Order("updated_at DESC").Order("id DESC").Limit(pageSize).Offset((pageNum - 1) * pageSize).Find(&notices).Error; err != nil {
		return nil, newRechargeNoticeError(RechargeNoticeErrorInternal, "查询充值待办失败", err)
	}
	ids := make([]uint64, 0, len(notices))
	for _, notice := range notices {
		ids = append(ids, notice.ID)
	}
	dtoMap, err := s.buildNoticeDTOMap(notices)
	if err != nil {
		return nil, err
	}
	content := make([]RechargeNoticeDTO, 0, len(notices))
	for _, id := range ids {
		content = append(content, dtoMap[id])
	}
	return newPage(content, total, pageNum, pageSize), nil
}

func (s *RechargeNoticeService) Acknowledge(id, version uint64) (*RechargeNoticeDTO, error) {
	if id == 0 || version == 0 {
		return nil, newRechargeNoticeError(RechargeNoticeErrorInvalidRequest, "id和version必须为正整数", nil)
	}
	result := s.db.Exec(`UPDATE coach_recharge_notice
		SET status = ?, acknowledged_at = NOW(), updated_at = updated_at
		WHERE id = ? AND status = ? AND version = ?`,
		model.RechargeNoticeStatusAcknowledged, id, model.RechargeNoticeStatusPending, version)
	if result.Error != nil {
		return nil, newRechargeNoticeError(RechargeNoticeErrorInternal, "确认充值待办失败", result.Error)
	}

	var notice model.RechargeNotice
	if err := s.db.First(&notice, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, newRechargeNoticeError(RechargeNoticeErrorNotFound, "充值待办不存在", nil)
		}
		return nil, newRechargeNoticeError(RechargeNoticeErrorInternal, "查询充值待办失败", err)
	}
	if result.RowsAffected == 0 && !(notice.Status == model.RechargeNoticeStatusAcknowledged && notice.Version == version) {
		return nil, newRechargeNoticeError(RechargeNoticeErrorVersionConflict, "充值待办已更新，请刷新后重试", nil)
	}
	if notice.Status != model.RechargeNoticeStatusAcknowledged || notice.Version != version {
		return nil, newRechargeNoticeError(RechargeNoticeErrorVersionConflict, "充值待办已更新，请刷新后重试", nil)
	}
	dtoMap, err := s.buildNoticeDTOMap([]model.RechargeNotice{notice})
	if err != nil {
		return nil, err
	}
	dto := dtoMap[id]
	return &dto, nil
}

func (s *RechargeNoticeService) loadNoticeDTOMap(ids []uint64) (map[uint64]RechargeNoticeDTO, error) {
	result := make(map[uint64]RechargeNoticeDTO, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var notices []model.RechargeNotice
	if err := s.db.Where("id IN ?", uniqueUint64(ids)).Find(&notices).Error; err != nil {
		return nil, newRechargeNoticeError(RechargeNoticeErrorInternal, "查询充值待办失败", err)
	}
	return s.buildNoticeDTOMap(notices)
}

func (s *RechargeNoticeService) buildNoticeDTOMap(notices []model.RechargeNotice) (map[uint64]RechargeNoticeDTO, error) {
	result := make(map[uint64]RechargeNoticeDTO, len(notices))
	if len(notices) == 0 {
		return result, nil
	}
	coachIDs := make([]uint64, 0, len(notices))
	memberIDs := make([]uint64, 0, len(notices))
	for _, notice := range notices {
		coachIDs = append(coachIDs, notice.CoachID)
		memberIDs = append(memberIDs, notice.MemberID)
	}
	var coaches []model.Coach
	if err := s.db.Unscoped().Select("coach_id", "name", "is_active", "deleted_at").Where("coach_id IN ?", uniqueUint64(coachIDs)).Find(&coaches).Error; err != nil {
		return nil, newRechargeNoticeError(RechargeNoticeErrorInternal, "查询充值待办教练失败", err)
	}
	coachMap := make(map[uint64]model.Coach, len(coaches))
	for _, coach := range coaches {
		coachMap[coach.ID] = coach
	}
	var members []model.PrepaidCard
	if err := s.db.Unscoped().Select("id", "name", "number", "deleted_at").Where("id IN ?", uniqueUint64(memberIDs)).Find(&members).Error; err != nil {
		return nil, newRechargeNoticeError(RechargeNoticeErrorInternal, "查询充值待办会员失败", err)
	}
	memberMap := make(map[uint64]model.PrepaidCard, len(members))
	for _, member := range members {
		memberMap[member.ID] = member
	}

	for _, notice := range notices {
		dto := RechargeNoticeDTO{
			ID:           notice.ID,
			CoachID:      notice.CoachID,
			MemberID:     notice.MemberID,
			RechargeDate: notice.RechargeDate.Format(rechargeNoticeDateFormat),
			Note:         notice.Note,
			Status:       notice.Status,
			Version:      notice.Version,
			CreatedAt:    notice.CreatedAt.Format(rechargeNoticeTimeFormat),
			UpdatedAt:    notice.UpdatedAt.Format(rechargeNoticeTimeFormat),
		}
		if notice.AcknowledgedAt != nil {
			formatted := notice.AcknowledgedAt.Format(rechargeNoticeTimeFormat)
			dto.AcknowledgedAt = &formatted
		}
		if coach, ok := coachMap[notice.CoachID]; ok {
			dto.CoachName = coach.Name
			dto.CoachActive = coach.IsActive == 1 && !coach.DeletedAt.Valid
		}
		if member, ok := memberMap[notice.MemberID]; ok {
			dto.MemberName = member.Name
			dto.MemberNumber = member.Number
			dto.MemberActive = !member.DeletedAt.Valid
		}
		result[notice.ID] = dto
	}
	return result, nil
}

func applyRechargeDateRange(query *gorm.DB, startDate, endDate string) (*gorm.DB, error) {
	if startDate != "" {
		start, err := time.ParseInLocation(rechargeNoticeDateFormat, startDate, time.Local)
		if err != nil {
			return nil, newRechargeNoticeError(RechargeNoticeErrorInvalidRequest, "startDate格式错误", err)
		}
		query = query.Where("recharge_date >= ?", start)
	}
	if endDate != "" {
		end, err := time.ParseInLocation(rechargeNoticeDateFormat, endDate, time.Local)
		if err != nil {
			return nil, newRechargeNoticeError(RechargeNoticeErrorInvalidRequest, "endDate格式错误", err)
		}
		query = query.Where("recharge_date <= ?", end)
	}
	if startDate != "" && endDate != "" && startDate > endDate {
		return nil, newRechargeNoticeError(RechargeNoticeErrorInvalidRequest, "startDate不能晚于endDate", nil)
	}
	return query, nil
}

func newRechargeNoticeError(code, message string, cause error) *RechargeNoticeError {
	return &RechargeNoticeError{Code: code, Message: message, Cause: cause}
}

func newPage[T any](content []T, total int64, pageNum, pageSize int) *Page[T] {
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	if content == nil {
		content = []T{}
	}
	return &Page[T]{
		Content:          content,
		TotalElements:    total,
		TotalPages:       totalPages,
		Size:             pageSize,
		Number:           pageNum - 1,
		NumberOfElements: len(content),
		First:            pageNum == 1,
		Last:             totalPages == 0 || pageNum >= totalPages,
	}
}
