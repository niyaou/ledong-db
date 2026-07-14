package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"ledong-db/internal/model"
)

const (
	PendingErrorInvalidRequest                 = "INVALID_REQUEST"
	PendingErrorInvalidMemberSpend             = "INVALID_MEMBER_SPEND"
	PendingErrorDuplicateMember                = "DUPLICATE_MEMBER"
	PendingErrorUnauthorized                   = "UNAUTHORIZED"
	PendingErrorNotFound                       = "PENDING_NOT_FOUND"
	PendingErrorCoachNotFound                  = "COACH_NOT_FOUND"
	PendingErrorCourtNotFound                  = "COURT_NOT_FOUND"
	PendingErrorMemberNotFound                 = "MEMBER_NOT_FOUND"
	PendingErrorUpdated                        = "PENDING_UPDATED"
	PendingErrorCourseDuplicate                = "COURSE_DUPLICATE"
	PendingErrorFormalCreatedPendingDeleteFail = "FORMAL_CREATED_PENDING_DELETE_FAILED"
	PendingErrorInternal                       = "INTERNAL_ERROR"
)

// PendingCourseError is a stable business error consumed by the HTTP handler.
type PendingCourseError struct {
	Code     string
	Message  string
	CourseID uint64
	Cause    error
}

func (e *PendingCourseError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *PendingCourseError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func newPendingCourseError(code, message string, cause error) *PendingCourseError {
	return &PendingCourseError{Code: code, Message: message, Cause: cause}
}

// MemberSpendInput is the structured form used by the new pending-course APIs.
type MemberSpendInput struct {
	MemberID    uint64  `json:"memberId"`
	Charge      float32 `json:"charge"`
	Times       float32 `json:"times"`
	AnnualTimes float32 `json:"annualTimes"`
	Description float32 `json:"description"`
	Quantities  int     `json:"quantities"`
}

// PendingCourseInput is the complete course submitted by the administrator.
type PendingCourseInput struct {
	CoachID     uint64             `json:"coachId"`
	CourtID     uint64             `json:"courtId"`
	StartTime   string             `json:"startTime"`
	EndTime     string             `json:"endTime"`
	Duration    float32            `json:"duration"`
	CourseType  *int               `json:"courseType"`
	IsAdult     *int               `json:"isAdult"`
	Description string             `json:"description"`
	MembersData []MemberSpendInput `json:"membersData"`
}

type storedPendingMemberSpend struct {
	MemberID    uint64  `json:"member_id"`
	Charge      float32 `json:"charge"`
	Times       float32 `json:"times"`
	AnnualTimes float32 `json:"annual_times"`
	Description float32 `json:"description"`
	Quantities  int     `json:"quantities"`
}

func decodeStoredPendingMembers(raw model.PendingJSON) ([]MemberSpendInput, error) {
	var stored []storedPendingMemberSpend
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, err
	}
	members := make([]MemberSpendInput, 0, len(stored))
	for _, member := range stored {
		members = append(members, MemberSpendInput{
			MemberID:    member.MemberID,
			Charge:      member.Charge,
			Times:       member.Times,
			AnnualTimes: member.AnnualTimes,
			Description: member.Description,
			Quantities:  member.Quantities,
		})
	}
	return members, nil
}

type validatedPendingCourseInput struct {
	StartTime time.Time
	EndTime   time.Time
	IsAdult   *int
}

func validatePendingCourseInput(input PendingCourseInput) (*validatedPendingCourseInput, error) {
	if input.CoachID == 0 {
		return nil, newPendingCourseError(PendingErrorInvalidRequest, "教练不能为空", nil)
	}
	if input.CourtID == 0 {
		return nil, newPendingCourseError(PendingErrorInvalidRequest, "校区不能为空", nil)
	}
	if input.CourseType == nil || !validCourseType(*input.CourseType) {
		return nil, newPendingCourseError(PendingErrorInvalidRequest, "课程类型不正确", nil)
	}
	courseType := *input.CourseType

	startTime, err := parsePendingBusinessTime(input.StartTime)
	if err != nil {
		return nil, newPendingCourseError(PendingErrorInvalidRequest, "开始时间格式错误", err)
	}
	endTime, err := parsePendingBusinessTime(input.EndTime)
	if err != nil {
		return nil, newPendingCourseError(PendingErrorInvalidRequest, "结束时间格式错误", err)
	}
	if !sameCalendarDay(startTime, endTime) {
		return nil, newPendingCourseError(PendingErrorInvalidRequest, "课程不允许跨天", nil)
	}
	if !endTime.After(startTime) {
		return nil, newPendingCourseError(PendingErrorInvalidRequest, "结束时间必须晚于开始时间", nil)
	}
	if !isHalfHourBoundary(startTime) || !isHalfHourBoundary(endTime) {
		return nil, newPendingCourseError(PendingErrorInvalidRequest, "开始和结束时间必须使用30分钟刻度", nil)
	}

	duration := float64(endTime.Sub(startTime)) / float64(time.Hour)
	if duration <= 0 || !isHalfStep(duration) {
		return nil, newPendingCourseError(PendingErrorInvalidRequest, "课程时长必须按0.5小时递增", nil)
	}
	if !finiteNonNegative(float64(input.Duration)) || !nearlyEqual(duration, float64(input.Duration)) {
		return nil, newPendingCourseError(PendingErrorInvalidRequest, "课程时长与开始结束时间不一致", nil)
	}

	adult := 1
	if input.IsAdult != nil {
		adult = *input.IsAdult
	}
	if adult != 0 && adult != 1 {
		return nil, newPendingCourseError(PendingErrorInvalidRequest, "成人/儿童类型不正确", nil)
	}
	if courseType != 0 && input.IsAdult == nil {
		return nil, newPendingCourseError(PendingErrorInvalidRequest, "请选择成人或儿童课程", nil)
	}
	if courseType < 0 {
		if len(input.MembersData) != 0 {
			return nil, newPendingCourseError(PendingErrorInvalidRequest, "体验课不能包含会员消费", nil)
		}
	} else if len(input.MembersData) == 0 {
		return nil, newPendingCourseError(PendingErrorInvalidRequest, "订场、班课和私教至少需要一名会员", nil)
	}

	seen := make(map[uint64]struct{}, len(input.MembersData))
	for index, member := range input.MembersData {
		if member.MemberID == 0 {
			return nil, newPendingCourseError(PendingErrorInvalidMemberSpend, fmt.Sprintf("第%d位会员无效", index+1), nil)
		}
		if _, exists := seen[member.MemberID]; exists {
			return nil, newPendingCourseError(PendingErrorDuplicateMember, "同一课程不能重复添加会员", nil)
		}
		seen[member.MemberID] = struct{}{}
		if err := validateMemberSpend(member); err != nil {
			var pendingErr *PendingCourseError
			if errors.As(err, &pendingErr) {
				pendingErr.Message = fmt.Sprintf("会员%d：%s", member.MemberID, pendingErr.Message)
			}
			return nil, err
		}
	}

	normalizedAdult := adult
	return &validatedPendingCourseInput{
		StartTime: startTime,
		EndTime:   endTime,
		IsAdult:   &normalizedAdult,
	}, nil
}

func validateMemberSpend(member MemberSpendInput) error {
	values := []float64{
		float64(member.Charge),
		float64(member.Times),
		float64(member.AnnualTimes),
		float64(member.Description),
	}
	for _, value := range values {
		if !finiteNonNegative(value) {
			return newPendingCourseError(PendingErrorInvalidMemberSpend, "扣费数值必须为有限的非负数", nil)
		}
	}
	if member.Quantities <= 0 {
		return newPendingCourseError(PendingErrorInvalidMemberSpend, "上课人数必须为大于0的整数", nil)
	}
	if !hasAtMostOneDecimal(float64(member.Charge)) || !hasAtMostOneDecimal(float64(member.Description)) {
		return newPendingCourseError(PendingErrorInvalidMemberSpend, "课时费和等效价格最多保留1位小数", nil)
	}

	switch {
	case member.Times > 0:
		if member.Times < 0.5 || !isHalfStep(float64(member.Times)) {
			return newPendingCourseError(PendingErrorInvalidMemberSpend, "次卡扣除必须至少0.5并按0.5递增", nil)
		}
		if member.Charge != 0 || member.AnnualTimes != 0 {
			return newPendingCourseError(PendingErrorInvalidMemberSpend, "每位会员只能选择一种扣费方式", nil)
		}
	case member.AnnualTimes > 0:
		if member.AnnualTimes < 0.5 || !isHalfStep(float64(member.AnnualTimes)) {
			return newPendingCourseError(PendingErrorInvalidMemberSpend, "年卡扣除必须至少0.5并按0.5递增", nil)
		}
		if member.Charge != 0 || member.Times != 0 {
			return newPendingCourseError(PendingErrorInvalidMemberSpend, "每位会员只能选择一种扣费方式", nil)
		}
	default:
		if member.Times != 0 || member.AnnualTimes != 0 {
			return newPendingCourseError(PendingErrorInvalidMemberSpend, "每位会员只能选择一种扣费方式", nil)
		}
		if !nearlyEqual(float64(member.Description), float64(member.Charge)) {
			return newPendingCourseError(PendingErrorInvalidMemberSpend, "课时费扣费的等效价格必须等于课时费", nil)
		}
	}
	return nil
}

func buildLegacyMembersJSON(members []MemberSpendInput, memberMap map[uint64]model.PrepaidCard) (string, error) {
	membersObj := make(map[string][]interface{}, len(members))
	for _, spend := range members {
		member, ok := memberMap[spend.MemberID]
		if !ok || member.Number == "" {
			return "", newPendingCourseError(PendingErrorMemberNotFound, fmt.Sprintf("会员%d不存在", spend.MemberID), nil)
		}
		membersObj[member.Number] = []interface{}{
			spend.Charge,
			spend.Times,
			spend.AnnualTimes,
			spend.Description,
			spend.Quantities,
		}
	}
	raw, err := json.Marshal(membersObj)
	if err != nil {
		return "", newPendingCourseError(PendingErrorInternal, "转换会员消费数据失败", err)
	}
	return string(raw), nil
}

func parsePendingBusinessTime(value string) (time.Time, error) {
	return time.ParseInLocation(pendingCourseTimeFormat, value, time.Local)
}

func validCourseType(courseType int) bool {
	switch courseType {
	case -2, -1, 0, 1, 2:
		return true
	default:
		return false
	}
}

func sameCalendarDay(left, right time.Time) bool {
	leftYear, leftMonth, leftDay := left.Date()
	rightYear, rightMonth, rightDay := right.Date()
	return leftYear == rightYear && leftMonth == rightMonth && leftDay == rightDay
}

func isHalfHourBoundary(value time.Time) bool {
	return value.Second() == 0 && value.Nanosecond() == 0 && (value.Minute() == 0 || value.Minute() == 30)
}

func finiteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func hasAtMostOneDecimal(value float64) bool {
	return nearlyEqual(value*10, math.Round(value*10))
}

func isHalfStep(value float64) bool {
	return nearlyEqual(value*2, math.Round(value*2))
}

func nearlyEqual(left, right float64) bool {
	return math.Abs(left-right) < 0.0001
}

func sameSecond(left, right time.Time) bool {
	return left.Truncate(time.Second).Equal(right.Truncate(time.Second))
}
