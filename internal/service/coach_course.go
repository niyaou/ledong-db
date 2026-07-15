package service

import (
	"errors"
	"fmt"
	"time"

	"ledong-db/internal/constants"
	"ledong-db/internal/model"

	"gorm.io/gorm"
)

const (
	coachCourseDateFormat = "2006-01-02"
	coachCourseTimeFormat = "2006-01-02 15:04:05"
	CoachCoursePageSize   = 50
)

var (
	ErrCoachCourseInvalidDate = errors.New("日期格式错误，请使用YYYY-MM-DD")
	ErrCoachCourseDateRange   = errors.New("开始日期不能晚于结束日期")
	ErrCoachCourseCoachAbsent = errors.New("教练不存在或已停用")
)

// CoachCourseMemberDTO contains the member and consumption fields displayed
// by the read-only administrator course page.
type CoachCourseMemberDTO struct {
	MemberID     uint64  `json:"memberId"`
	MemberName   string  `json:"memberName"`
	MemberNumber string  `json:"memberNumber"`
	Charge       float32 `json:"charge"`
	Times        float32 `json:"times"`
	AnnualTimes  float32 `json:"annualTimes"`
	Description  float32 `json:"description"`
	Quantities   int     `json:"quantities"`
}

// CoachCourseDTO mirrors the formal-course data shown in the coach mini
// program while keeping this API independent from all course write flows.
type CoachCourseDTO struct {
	ID          uint64                 `json:"id"`
	CoachID     uint64                 `json:"coachId"`
	CoachName   string                 `json:"coachName"`
	CourtID     uint64                 `json:"courtId"`
	CourtName   string                 `json:"courtName"`
	StartTime   string                 `json:"startTime"`
	EndTime     string                 `json:"endTime"`
	Duration    float32                `json:"duration"`
	CourseType  int                    `json:"courseType"`
	IsAdult     *int                   `json:"isAdult"`
	Description string                 `json:"description"`
	MembersData []CoachCourseMemberDTO `json:"membersData"`
}

type coachCourseRow struct {
	ID          uint64
	CoachID     uint64
	CourtID     uint64
	CourtName   string
	StartTime   time.Time
	EndTime     time.Time
	Duration    float32
	CourseType  int
	IsAdult     *int
	Description string
}

type coachCourseMemberRow struct {
	CourseID     uint64
	MemberID     uint64
	MemberName   string
	MemberNumber string
	Charge       float32
	Times        float32
	AnnualTimes  float32
	Description  float32
	Quantities   int
}

// CoachCourses returns one active coach's formal courses in a date range.
// The end date is inclusive; internally it is represented as the next day's
// exclusive midnight so sub-second course timestamps are not omitted.
func (s *CourseService) CoachCourses(coachID uint64, startDate, endDate string, pageNum int) (*Page[CoachCourseDTO], error) {
	start, endExclusive, err := parseCoachCourseDateRange(startDate, endDate)
	if err != nil {
		return nil, err
	}
	if coachID == 0 {
		return nil, ErrCoachCourseCoachAbsent
	}
	if pageNum < 1 {
		pageNum = 1
	}

	var coach model.Coach
	if err := s.db.
		Select("coach_id", "name").
		Where("coach_id = ? AND is_active = ? AND deleted_at IS NULL", coachID, 1).
		First(&coach).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCoachCourseCoachAbsent
		}
		return nil, fmt.Errorf("查询教练失败: %w", err)
	}

	baseQuery := s.db.Model(&model.Course{}).
		Where("course.coach_id = ?", coachID).
		Where("course.start_time >= ? AND course.start_time < ?", start, endExclusive).
		Where("course.deleted_at IS NULL")

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("统计教练课程失败: %w", err)
	}

	var rows []coachCourseRow
	offset := (pageNum - 1) * CoachCoursePageSize
	if err := baseQuery.
		Select(`course.id, course.coach_id, course.court_id,
			COALESCE(court.name, '') AS court_name, course.start_time,
			course.end_time, course.duration, course.course_type,
			course.is_adult, course.description`).
		Joins("LEFT JOIN court ON court.id = course.court_id AND court.deleted_at IS NULL").
		Order("course.start_time DESC").
		Order("course.id DESC").
		Offset(offset).
		Limit(CoachCoursePageSize).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询教练课程失败: %w", err)
	}

	courseIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		courseIDs = append(courseIDs, row.ID)
	}
	membersByCourse, err := s.loadCoachCourseMembers(courseIDs)
	if err != nil {
		return nil, err
	}

	content := make([]CoachCourseDTO, 0, len(rows))
	for _, row := range rows {
		members := membersByCourse[row.ID]
		if members == nil {
			members = []CoachCourseMemberDTO{}
		}
		content = append(content, CoachCourseDTO{
			ID:          row.ID,
			CoachID:     row.CoachID,
			CoachName:   coach.Name,
			CourtID:     row.CourtID,
			CourtName:   row.CourtName,
			StartTime:   row.StartTime.In(start.Location()).Format(coachCourseTimeFormat),
			EndTime:     row.EndTime.In(start.Location()).Format(coachCourseTimeFormat),
			Duration:    row.Duration,
			CourseType:  row.CourseType,
			IsAdult:     row.IsAdult,
			Description: row.Description,
			MembersData: members,
		})
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + CoachCoursePageSize - 1) / CoachCoursePageSize)
	}
	return &Page[CoachCourseDTO]{
		Content:          content,
		TotalElements:    total,
		TotalPages:       totalPages,
		Size:             CoachCoursePageSize,
		Number:           pageNum - 1,
		NumberOfElements: len(content),
		First:            pageNum == 1,
		Last:             totalPages == 0 || pageNum >= totalPages,
	}, nil
}

func parseCoachCourseDateRange(startDate, endDate string) (time.Time, time.Time, error) {
	location, err := time.LoadLocation(constants.BusinessTimeZone)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("加载业务时区失败: %w", err)
	}
	start, err := time.ParseInLocation(coachCourseDateFormat, startDate, location)
	if err != nil {
		return time.Time{}, time.Time{}, ErrCoachCourseInvalidDate
	}
	end, err := time.ParseInLocation(coachCourseDateFormat, endDate, location)
	if err != nil {
		return time.Time{}, time.Time{}, ErrCoachCourseInvalidDate
	}
	if start.After(end) {
		return time.Time{}, time.Time{}, ErrCoachCourseDateRange
	}
	return start, end.AddDate(0, 0, 1), nil
}

func (s *CourseService) loadCoachCourseMembers(courseIDs []uint64) (map[uint64][]CoachCourseMemberDTO, error) {
	result := make(map[uint64][]CoachCourseMemberDTO, len(courseIDs))
	if len(courseIDs) == 0 {
		return result, nil
	}

	var rows []coachCourseMemberRow
	if err := s.db.Table("course_member AS cm").
		Select(`cm.course_id, pc.id AS member_id, pc.name AS member_name,
			pc.number AS member_number, COALESCE(s.charge, 0) AS charge,
			COALESCE(s.times, 0) AS times,
			COALESCE(s.annual_times, 0) AS annual_times,
			COALESCE(s.description, 0) AS description,
			COALESCE(s.quantities, 1) AS quantities`).
		Joins("JOIN prepaid_card AS pc ON pc.id = cm.member_id").
		Joins(`LEFT JOIN spend AS s ON s.course_id = cm.course_id
			AND s.prepaid_card_id = cm.member_id AND s.deleted_at IS NULL`).
		Where("cm.course_id IN ?", courseIDs).
		Order("cm.course_id ASC").
		Order("pc.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询课程会员消费失败: %w", err)
	}

	for _, row := range rows {
		result[row.CourseID] = append(result[row.CourseID], CoachCourseMemberDTO{
			MemberID:     row.MemberID,
			MemberName:   row.MemberName,
			MemberNumber: row.MemberNumber,
			Charge:       row.Charge,
			Times:        row.Times,
			AnnualTimes:  row.AnnualTimes,
			Description:  row.Description,
			Quantities:   row.Quantities,
		})
	}
	return result, nil
}
