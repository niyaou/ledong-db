package service

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"ledong-db/internal/database"
	"ledong-db/internal/logger"
	"ledong-db/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const pendingCourseTimeFormat = "2006-01-02 15:04:05"

// PendingCourseService owns the pending queue and delegates all formal writes
// to the existing CourseService.
type PendingCourseService struct {
	db            *gorm.DB
	courseService *CourseService
}

func NewPendingCourseService(courseService *CourseService) *PendingCourseService {
	return &PendingCourseService{
		db:            database.DB,
		courseService: courseService,
	}
}

// PendingCourseMemberDTO contains both the stored business values and all
// display/balance fields needed by the administrator page.
type PendingCourseMemberDTO struct {
	MemberID     uint64  `json:"memberId"`
	MemberName   string  `json:"memberName"`
	MemberNumber string  `json:"memberNumber"`
	Charge       float32 `json:"charge"`
	Times        float32 `json:"times"`
	AnnualTimes  float32 `json:"annualTimes"`
	Description  float32 `json:"description"`
	Quantities   int     `json:"quantities"`
	RestCharge   float32 `json:"restCharge"`
	TimesCount   float32 `json:"timesCount"`
	AnnualCount  float32 `json:"annualCount"`
}

// PendingCourseDTO is intentionally flat. Grouping by court is a frontend
// responsibility.
type PendingCourseDTO struct {
	ID          uint64                   `json:"id"`
	CoachID     uint64                   `json:"coachId"`
	CoachName   string                   `json:"coachName"`
	CourtID     uint64                   `json:"courtId"`
	CourtName   string                   `json:"courtName"`
	StartTime   string                   `json:"startTime"`
	EndTime     string                   `json:"endTime"`
	Duration    float32                  `json:"duration"`
	CourseType  int                      `json:"courseType"`
	IsAdult     *int                     `json:"isAdult"`
	Description string                   `json:"description"`
	MembersData []PendingCourseMemberDTO `json:"membersData"`
	CreatedAt   string                   `json:"createdAt"`
	UpdatedAt   string                   `json:"updatedAt"`
}

// ListAll returns all queue messages and batch-assembles every display field.
func (s *PendingCourseService) ListAll() ([]PendingCourseDTO, error) {
	var pendingCourses []model.PendingCourse
	if err := s.db.Order("start_time DESC").Order("id DESC").Find(&pendingCourses).Error; err != nil {
		return nil, newPendingCourseError(PendingErrorInternal, "查询待审课程失败", err)
	}
	return s.buildDTOs(pendingCourses)
}

// listByIDs supports the unified administrator inbox while preserving the
// legacy all-record endpoint. Associations are still loaded in batches.
func (s *PendingCourseService) listByIDs(ids []uint64) ([]PendingCourseDTO, error) {
	ids = uniqueUint64(ids)
	if len(ids) == 0 {
		return []PendingCourseDTO{}, nil
	}
	var pendingCourses []model.PendingCourse
	if err := s.db.Where("id IN ?", ids).Find(&pendingCourses).Error; err != nil {
		return nil, newPendingCourseError(PendingErrorInternal, "查询待审课程失败", err)
	}
	return s.buildDTOs(pendingCourses)
}

func (s *PendingCourseService) buildDTOs(pendingCourses []model.PendingCourse) ([]PendingCourseDTO, error) {
	if len(pendingCourses) == 0 {
		return []PendingCourseDTO{}, nil
	}

	memberInputsByPendingID := make(map[uint64][]MemberSpendInput, len(pendingCourses))
	coachIDSet := make(map[uint64]struct{})
	courtIDSet := make(map[uint64]struct{})
	memberIDSet := make(map[uint64]struct{})
	for _, pending := range pendingCourses {
		members, err := decodeStoredPendingMembers(pending.MembersData)
		if err != nil {
			logger.Error("待审课程 members_data 解析失败", "pending_id", pending.ID, "error", err)
			return nil, newPendingCourseError(PendingErrorInternal, "待审课程会员数据损坏", err)
		}
		memberInputsByPendingID[pending.ID] = members
		coachIDSet[pending.CoachID] = struct{}{}
		courtIDSet[pending.CourtID] = struct{}{}
		for _, member := range members {
			memberIDSet[member.MemberID] = struct{}{}
		}
	}

	coachMap, err := s.loadCoachesByID(setKeys(coachIDSet))
	if err != nil {
		return nil, err
	}
	courtMap, err := s.loadCourtsByID(setKeys(courtIDSet))
	if err != nil {
		return nil, err
	}
	memberMap, err := s.loadMembersByID(setKeys(memberIDSet))
	if err != nil {
		return nil, err
	}

	result := make([]PendingCourseDTO, 0, len(pendingCourses))
	for _, pending := range pendingCourses {
		dto := PendingCourseDTO{
			ID:          pending.ID,
			CoachID:     pending.CoachID,
			CourtID:     pending.CourtID,
			StartTime:   pending.StartTime.Format(pendingCourseTimeFormat),
			EndTime:     pending.EndTime.Format(pendingCourseTimeFormat),
			Duration:    pending.Duration,
			CourseType:  pending.CourseType,
			IsAdult:     pending.IsAdult,
			Description: pending.Description,
			MembersData: make([]PendingCourseMemberDTO, 0, len(memberInputsByPendingID[pending.ID])),
			CreatedAt:   pending.CreatedAt.Format(pendingCourseTimeFormat),
			UpdatedAt:   pending.UpdatedAt.Format(pendingCourseTimeFormat),
		}
		if coach, ok := coachMap[pending.CoachID]; ok {
			dto.CoachName = coach.Name
		}
		if court, ok := courtMap[pending.CourtID]; ok {
			dto.CourtName = court.Name
		}
		for _, spend := range memberInputsByPendingID[pending.ID] {
			memberDTO := PendingCourseMemberDTO{
				MemberID:    spend.MemberID,
				Charge:      spend.Charge,
				Times:       spend.Times,
				AnnualTimes: spend.AnnualTimes,
				Description: spend.Description,
				Quantities:  spend.Quantities,
			}
			if member, ok := memberMap[spend.MemberID]; ok {
				memberDTO.MemberName = member.Name
				memberDTO.MemberNumber = member.Number
				memberDTO.RestCharge = member.RestCharge
				memberDTO.TimesCount = member.TimesCount
				memberDTO.AnnualCount = member.AnnualCount
			}
			dto.MembersData = append(dto.MembersData, memberDTO)
		}
		result = append(result, dto)
	}
	return result, nil
}

// Admit validates a queue message version, creates the formal course through
// the legacy service, and then physically consumes the queue message.
func (s *PendingCourseService) Admit(id uint64, updatedAt string, input PendingCourseInput) (*model.Course, error) {
	if id == 0 {
		return nil, newPendingCourseError(PendingErrorInvalidRequest, "待审课程ID格式错误", nil)
	}
	requestUpdatedAt, err := time.ParseInLocation(pendingCourseTimeFormat, updatedAt, time.Local)
	if err != nil {
		return nil, newPendingCourseError(PendingErrorInvalidRequest, "updatedAt格式错误", err)
	}

	var pending model.PendingCourse
	if err := s.db.First(&pending, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, newPendingCourseError(PendingErrorNotFound, "待审课程已不存在", nil)
		}
		return nil, newPendingCourseError(PendingErrorInternal, "查询待审课程失败", err)
	}
	if !sameSecond(pending.UpdatedAt, requestUpdatedAt) {
		return nil, newPendingCourseError(PendingErrorUpdated, "数据已更新，请刷新后重新审核", nil)
	}

	validated, err := validatePendingCourseInput(input)
	if err != nil {
		return nil, err
	}

	duplicate, err := s.formalCourseExists(input.CoachID, validated.StartTime, validated.EndTime)
	if err != nil {
		return nil, err
	}
	if duplicate {
		return nil, newPendingCourseError(PendingErrorCourseDuplicate, "正式课程已存在", nil)
	}

	coachMap, err := s.loadCoachesByID([]uint64{input.CoachID})
	if err != nil {
		return nil, err
	}
	coach, ok := coachMap[input.CoachID]
	if !ok || coach.Number == "" {
		return nil, newPendingCourseError(PendingErrorCoachNotFound, "教练不存在", nil)
	}

	courtMap, err := s.loadCourtsByID([]uint64{input.CourtID})
	if err != nil {
		return nil, err
	}
	court, ok := courtMap[input.CourtID]
	if !ok || court.Name == "" {
		return nil, newPendingCourseError(PendingErrorCourtNotFound, "校区不存在", nil)
	}

	memberIDs := make([]uint64, 0, len(input.MembersData))
	for _, member := range input.MembersData {
		memberIDs = append(memberIDs, member.MemberID)
	}
	memberMap, err := s.loadMembersByID(memberIDs)
	if err != nil {
		return nil, err
	}
	for _, memberID := range memberIDs {
		if _, ok := memberMap[memberID]; !ok {
			return nil, newPendingCourseError(PendingErrorMemberNotFound, fmt.Sprintf("会员%d不存在", memberID), nil)
		}
	}

	membersJSON, err := buildLegacyMembersJSON(input.MembersData, memberMap)
	if err != nil {
		return nil, err
	}

	formalCourse, err := s.courseService.CreateCourse(
		validated.StartTime.Format(pendingCourseTimeFormat),
		validated.EndTime.Format(pendingCourseTimeFormat),
		coach.Number,
		input.Duration,
		court.Name,
		input.Description,
		*input.CourseType,
		membersJSON,
		validated.IsAdult,
	)
	if err != nil {
		return nil, newPendingCourseError(PendingErrorInternal, "正式课程创建失败", err)
	}

	if formalCourse == nil {
		return nil, newPendingCourseError(PendingErrorInternal, "正式课程创建失败", errors.New("CreateCourse returned nil course"))
	}

	if err := s.consumeAfterFormalCreation(id, pending, requestUpdatedAt, formalCourse.ID); err != nil {
		return nil, err
	}
	return formalCourse, nil
}

func (s *PendingCourseService) formalCourseExists(coachID uint64, startTime, endTime time.Time) (bool, error) {
	var count int64
	if err := s.db.Model(&model.Course{}).
		Where("coach_id = ? AND start_time = ? AND end_time = ? AND deleted_at IS NULL", coachID, startTime, endTime).
		Count(&count).Error; err != nil {
		return false, newPendingCourseError(PendingErrorInternal, "检查正式课程重复失败", err)
	}
	return count > 0, nil
}

func (s *PendingCourseService) consumeAfterFormalCreation(id uint64, originallyChecked model.PendingCourse, requestedUpdatedAt time.Time, formalCourseID uint64) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var latest model.PendingCourse
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&latest, id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warn(
				"正式课已创建但待审记录已不存在",
				"pending_id", id,
				"coach_id", originallyChecked.CoachID,
				"frontend_updated_at", requestedUpdatedAt.Format(pendingCourseTimeFormat),
				"checked_updated_at", originallyChecked.UpdatedAt.Format(pendingCourseTimeFormat),
				"formal_course_id", formalCourseID,
				"operation_time", time.Now().Format(pendingCourseTimeFormat),
			)
			return nil
		}
		if err != nil {
			return err
		}
		if !sameSecond(latest.UpdatedAt, originallyChecked.UpdatedAt) {
			logger.Warn(
				"录课期间待审记录发生变化，按需求继续消费",
				"pending_id", id,
				"coach_id", originallyChecked.CoachID,
				"frontend_updated_at", requestedUpdatedAt.Format(pendingCourseTimeFormat),
				"checked_updated_at", originallyChecked.UpdatedAt.Format(pendingCourseTimeFormat),
				"latest_updated_at", latest.UpdatedAt.Format(pendingCourseTimeFormat),
				"formal_course_id", formalCourseID,
				"operation_time", time.Now().Format(pendingCourseTimeFormat),
			)
		}
		return tx.Delete(&latest).Error
	})
	if err == nil {
		return nil
	}

	logger.Error(
		"正式课已创建但待审记录删除失败",
		"pending_id", id,
		"coach_id", originallyChecked.CoachID,
		"frontend_updated_at", requestedUpdatedAt.Format(pendingCourseTimeFormat),
		"checked_updated_at", originallyChecked.UpdatedAt.Format(pendingCourseTimeFormat),
		"formal_course_id", formalCourseID,
		"operation_time", time.Now().Format(pendingCourseTimeFormat),
		"error", err,
	)
	return &PendingCourseError{
		Code:     PendingErrorFormalCreatedPendingDeleteFail,
		Message:  "正式课程已创建，但待审记录消费失败，请勿重复录取",
		CourseID: formalCourseID,
		Cause:    err,
	}
}

func (s *PendingCourseService) loadCoachesByID(ids []uint64) (map[uint64]model.Coach, error) {
	result := make(map[uint64]model.Coach, len(ids))
	ids = uniqueUint64(ids)
	if len(ids) == 0 {
		return result, nil
	}
	var coaches []model.Coach
	if err := s.db.Select("coach_id", "name", "number").Where("coach_id IN ?", ids).Find(&coaches).Error; err != nil {
		return nil, newPendingCourseError(PendingErrorInternal, "查询教练信息失败", err)
	}
	for _, coach := range coaches {
		result[coach.ID] = coach
	}
	return result, nil
}

func (s *PendingCourseService) loadCourtsByID(ids []uint64) (map[uint64]model.Court, error) {
	result := make(map[uint64]model.Court, len(ids))
	ids = uniqueUint64(ids)
	if len(ids) == 0 {
		return result, nil
	}
	var courts []model.Court
	if err := s.db.Select("id", "name").Where("id IN ?", ids).Find(&courts).Error; err != nil {
		return nil, newPendingCourseError(PendingErrorInternal, "查询校区信息失败", err)
	}
	for _, court := range courts {
		result[court.ID] = court
	}
	return result, nil
}

func (s *PendingCourseService) loadMembersByID(ids []uint64) (map[uint64]model.PrepaidCard, error) {
	result := make(map[uint64]model.PrepaidCard, len(ids))
	ids = uniqueUint64(ids)
	if len(ids) == 0 {
		return result, nil
	}
	var members []model.PrepaidCard
	if err := s.db.
		Select("id", "name", "number", "rest_charge", "times_count", "annual_count").
		Where("id IN ?", ids).
		Find(&members).Error; err != nil {
		return nil, newPendingCourseError(PendingErrorInternal, "查询会员信息失败", err)
	}
	for _, member := range members {
		result[member.ID] = member
	}
	return result, nil
}

func setKeys(values map[uint64]struct{}) []uint64 {
	result := make([]uint64, 0, len(values))
	for value := range values {
		if value > 0 {
			result = append(result, value)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func uniqueUint64(values []uint64) []uint64 {
	set := make(map[uint64]struct{}, len(values))
	for _, value := range values {
		if value > 0 {
			set[value] = struct{}{}
		}
	}
	return setKeys(set)
}
