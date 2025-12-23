package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"ledong-db/internal/constants"
	"ledong-db/internal/database"
	"ledong-db/internal/model"
	"strings"
	"time"

	"gorm.io/gorm"
)

type CourseService struct {
	db          *gorm.DB
	userService *UserService
	smsService  *SmsService
	courseCache []model.Course
}

func NewCourseService(userService *UserService, smsService *SmsService) *CourseService {
	return &CourseService{
		db:          database.DB,
		userService: userService,
		smsService:  smsService,
		courseCache: make([]model.Course, 0),
	}
}

type Page[T any] struct {
	Content          []T   `json:"content"`
	TotalElements    int64 `json:"totalElements"`
	TotalPages       int   `json:"totalPages"`
	Size             int   `json:"size"`
	Number           int   `json:"number"`
	NumberOfElements int   `json:"numberOfElements"`
	First            bool  `json:"first"`
	Last             bool  `json:"last"`
}

type CoursePage = Page[model.Course]

func (s *CourseService) TotalCourse(startTime string, number *string, pageNum, pageSize int) (*Page[model.Course], error) {
	var parsedTime time.Time
	var err error
	for _, format := range constants.TimeFormats {
		parsedTime, err = time.ParseInLocation(format, startTime, time.Local)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}

	query := s.db.Model(&model.Course{}).
		Where("start_time >= ?", parsedTime).
		Where("course.deleted_at IS NULL").
		Order("start_time DESC")

	if number != nil && *number != "" {
		query = query.Joins("JOIN course_member ON course.id = course_member.course_id").
			Joins("JOIN prepaid_card ON course_member.member_id = prepaid_card.id").
			Where("prepaid_card.number = ?", *number).
			Group("course.id")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var courses []model.Course
	offset := (pageNum - 1) * pageSize
	findQuery := s.db.Model(&model.Course{}).
		Where("start_time >= ?", parsedTime).
		Where("course.deleted_at IS NULL")

	if number != nil && *number != "" {
		var courseIDs []uint64
		if err := s.db.Table("course_member").
			Select("DISTINCT course_member.course_id").
			Joins("JOIN prepaid_card ON course_member.member_id = prepaid_card.id").
			Joins("JOIN course ON course_member.course_id = course.id").
			Where("prepaid_card.number = ? AND course.deleted_at IS NULL", *number).
			Pluck("course_member.course_id", &courseIDs).Error; err != nil {
			return nil, err
		}
		if len(courseIDs) == 0 {
			return &CoursePage{
				Content:          []model.Course{},
				TotalElements:    0,
				TotalPages:       0,
				Size:             pageSize,
				Number:           pageNum - 1,
				NumberOfElements: 0,
				First:            pageNum == 1,
				Last:             true,
			}, nil
		}
		findQuery = findQuery.Where("course.id IN ?", courseIDs)
	}

	if err := findQuery.
		Preload("Court").
		Preload("Members").
		Preload("Spends").
		Order("start_time DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&courses).Error; err != nil {
		return nil, err
	}

	s.loadCoachesForCourses(courses)

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	numberOfElements := len(courses)

	return &CoursePage{
		Content:          courses,
		TotalElements:    total,
		TotalPages:       totalPages,
		Size:             pageSize,
		Number:           pageNum - 1,
		NumberOfElements: numberOfElements,
		First:            pageNum == 1,
		Last:             totalPages == 0 || pageNum >= totalPages,
	}, nil
}

func (s *CourseService) CreateCourse(startTime, endTime, coachName string, spendingTime float32, courtName, descript string, courseType int, membersObj string, isAdult *int) (*model.Course, error) {
	var coach model.Coach
	if err := s.db.Where("number = ?", coachName).First(&coach).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("教练不存在")
		}
		return nil, err
	}

	var court model.Court
	if err := s.db.Where("name = ?", courtName).First(&court).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("场地不存在")
		}
		return nil, err
	}

	var parsedStartTime, parsedEndTime time.Time
	var err error
	for _, format := range constants.TimeFormats {
		parsedStartTime, err = time.ParseInLocation(format, startTime, time.Local)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("开始时间格式错误: %w", err)
	}

	for _, format := range constants.TimeFormats {
		parsedEndTime, err = time.ParseInLocation(format, endTime, time.Local)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("结束时间格式错误: %w", err)
	}

	course := &model.Course{
		StartTime:   parsedStartTime,
		EndTime:     parsedEndTime,
		Duration:    spendingTime,
		Description: descript,
		CourseType:  courseType,
		CourtID:     court.ID,
		CoachID:     coach.ID,
		Notified:    0,
	}

	if isAdult != nil {
		course.IsAdult = isAdult
	}

	if err := s.db.Create(course).Error; err != nil {
		return nil, err
	}

	if courseType >= 0 && membersObj != "" {
		var membersData map[string][]interface{}
		if err := json.Unmarshal([]byte(membersObj), &membersData); err != nil {
			return nil, fmt.Errorf("解析会员数据失败: %w", err)
		}

		// 批量查询所有会员，避免N+1查询
		memberNumbers := make([]string, 0, len(membersData))
		for memberNumber := range membersData {
			memberNumbers = append(memberNumbers, memberNumber)
		}

		var members []model.PrepaidCard
		if err := s.db.Where("number IN ?", memberNumbers).Find(&members).Error; err != nil {
			return nil, err
		}

		// 构建会员编号到会员的映射
		memberMap := make(map[string]*model.PrepaidCard)
		for i := range members {
			memberMap[members[i].Number] = &members[i]
		}

		// 收集需要添加的会员ID，避免重复
		memberIDs := make([]uint64, 0, len(membersData))
		memberAdded := make(map[uint64]bool)

		for memberNumber, value := range membersData {
			member, ok := memberMap[memberNumber]
			if !ok {
				continue
			}

			if len(value) < 5 {
				continue
			}

			var charge, times, annualTimes, spendDescript float32
			var quantities int

			if v, ok := value[0].(float64); ok {
				charge = float32(v)
			}
			if v, ok := value[1].(float64); ok {
				times = float32(v)
			}
			if v, ok := value[2].(float64); ok {
				annualTimes = float32(v)
			}
			if v, ok := value[3].(float64); ok {
				spendDescript = float32(v)
			}
			if v, ok := value[4].(float64); ok {
				quantities = int(v)
			} else if v, ok := value[4].(int); ok {
				quantities = v
			}

			spend := &model.Spend{
				CourseID:      course.ID,
				PrepaidCardID: member.ID,
				Charge:        charge,
				Times:         times,
				AnnualTimes:   annualTimes,
				Description:   spendDescript,
				Quantities:    quantities,
			}

			if charge != 0 {
				if err := s.userService.SetRestChargeChange(memberNumber, -charge); err != nil {
					return nil, err
				}
			}
			if times != 0 {
				if err := s.userService.SetRestTimesChange(memberNumber, -times); err != nil {
					return nil, err
				}
			}
			if annualTimes != 0 {
				if err := s.userService.SetRestAnnualTimesChange(memberNumber, -annualTimes); err != nil {
					return nil, err
				}
			}
			if spendDescript != 0 {
				if err := s.userService.SetEquivalentChange(memberNumber, -int(spendDescript)); err != nil {
					return nil, err
				}
			}

			expiredTime := time.Now().AddDate(0, 0, 60)
			if err := s.userService.SetTimesExpired(memberNumber, expiredTime); err != nil {
				return nil, err
			}

			if err := s.db.Create(spend).Error; err != nil {
				return nil, err
			}

			// 避免重复添加同一个会员
			if !memberAdded[member.ID] {
				memberIDs = append(memberIDs, member.ID)
				memberAdded[member.ID] = true
			}
		}

		// 直接使用SQL批量插入，先检查已存在的记录避免重复
		if len(memberIDs) > 0 {
			var existing []struct {
				MemberID uint64
			}
			if err := s.db.Table("course_member").
				Select("member_id").
				Where("course_id = ? AND member_id IN ?", course.ID, memberIDs).
				Find(&existing).Error; err != nil {
				return nil, err
			}

			existingMap := make(map[uint64]bool)
			for _, e := range existing {
				existingMap[e.MemberID] = true
			}

			newMemberIDs := make([]uint64, 0)
			for _, memberID := range memberIDs {
				if !existingMap[memberID] {
					newMemberIDs = append(newMemberIDs, memberID)
				}
			}

			if len(newMemberIDs) > 0 {
				values := make([]string, 0, len(newMemberIDs))
				for _, memberID := range newMemberIDs {
					values = append(values, fmt.Sprintf("(%d, %d)", course.ID, memberID))
				}
				sql := fmt.Sprintf("INSERT INTO course_member (course_id, member_id) VALUES %s",
					strings.Join(values, ", "))
				if err := s.db.Exec(sql).Error; err != nil {
					return nil, err
				}
			}
		}
	}

	s.updateCache()
	return course, nil
}

func (s *CourseService) RemoveCourseMember(courseId uint64, number string) (*model.Course, error) {
	var course model.Course
	if err := s.db.Where("deleted_at IS NULL").Preload("Members").Preload("Spends.PrepaidCard").First(&course, courseId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	member, err := s.userService.FindByNumber(number)
	if err != nil {
		return nil, err
	}

	// 从已Preload的Spends中查找，避免额外查询
	var spend *model.Spend
	for i := range course.Spends {
		if course.Spends[i].PrepaidCardID == member.ID {
			spend = &course.Spends[i]
			break
		}
	}

	if spend != nil {
		if spend.Charge != 0 {
			if err := s.userService.SetRestChargeChange(number, spend.Charge); err != nil {
				return nil, err
			}
		}
		if spend.Times != 0 {
			if err := s.userService.SetRestTimesChange(number, spend.Times); err != nil {
				return nil, err
			}
		}
		if spend.AnnualTimes != 0 {
			if err := s.userService.SetRestAnnualTimesChange(number, spend.AnnualTimes); err != nil {
				return nil, err
			}
		}
		if err := s.db.Delete(&spend).Error; err != nil {
			return nil, err
		}
	}

	if err := s.db.Model(&course).Association("Members").Delete(member); err != nil {
		return nil, err
	}

	return &course, nil
}

func (s *CourseService) RemoveCourse(courseId uint64) (*model.Course, error) {
	var course model.Course
	if err := s.db.Where("deleted_at IS NULL").Preload("Members").Preload("Spends.PrepaidCard").First(&course, courseId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	for _, spend := range course.Spends {
		member := spend.PrepaidCard
		if member.ID == 0 {
			continue
		}

		if spend.Charge != 0 {
			if err := s.userService.SetRestChargeChange(member.Number, spend.Charge); err != nil {
				return nil, err
			}
		}
		if spend.Times != 0 {
			if err := s.userService.SetRestTimesChange(member.Number, spend.Times); err != nil {
				return nil, err
			}
		}
		if spend.AnnualTimes != 0 {
			if err := s.userService.SetRestAnnualTimesChange(member.Number, spend.AnnualTimes); err != nil {
				return nil, err
			}
		}
		if spend.Description != 0 {
			if err := s.userService.SetEquivalentChange(member.Number, int(spend.Description)); err != nil {
				return nil, err
			}
		}
	}

	if err := s.db.Model(&course).Association("Members").Clear(); err != nil {
		return nil, err
	}

	if err := s.db.Delete(&course).Error; err != nil {
		return nil, err
	}

	return &course, nil
}

func (s *CourseService) TrialCourseUpdate(courseId uint64) (*model.Course, error) {
	var course model.Course
	if err := s.db.Where("deleted_at IS NULL").First(&course, courseId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	course.CourseType++
	if err := s.db.Save(&course).Error; err != nil {
		return nil, err
	}

	return &course, nil
}

func (s *CourseService) Notify(courseId uint64) error {
	if s.smsService == nil {
		return errors.New("短信服务未初始化")
	}
	return s.smsService.Notify(courseId)
}

func (s *CourseService) MemberCourse(startTime, number string, pageNum, pageSize int) (*CoursePage, error) {
	var parsedTime time.Time
	var err error
	for _, format := range constants.TimeFormats {
		parsedTime, err = time.ParseInLocation(format, startTime, time.Local)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}

	member, err := s.userService.FindByNumber(number)
	if err != nil {
		return nil, err
	}

	query := s.db.Model(&model.Course{}).
		Joins("JOIN course_member ON course.id = course_member.course_id").
		Where("course_member.member_id = ? AND course.start_time >= ? AND course.deleted_at IS NULL", member.ID, parsedTime).
		Order("course.start_time DESC").
		Group("course.id")

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var courses []model.Course
	offset := (pageNum - 1) * pageSize
	if err := query.
		Preload("Coach").
		Preload("Court").
		Preload("Members").
		Preload("Spends").
		Offset(offset).
		Limit(pageSize).
		Find(&courses).Error; err != nil {
		return nil, err
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return &CoursePage{
		Content:          courses,
		TotalElements:    total,
		TotalPages:       totalPages,
		Size:             pageSize,
		Number:           pageNum - 1,
		NumberOfElements: len(courses),
		First:            pageNum == 1,
		Last:             totalPages == 0 || pageNum >= totalPages,
	}, nil
}

func (s *CourseService) courseExist(item []interface{}) []interface{} {
	if len(item) < 4 {
		return nil
	}

	coachName, ok1 := item[0].(string)
	startTimeStr, ok2 := item[2].(string)
	endTimeStr, ok3 := item[3].(string)

	if !ok1 || !ok2 || !ok3 {
		return nil
	}

	var startTime, endTime time.Time
	var err error

	for _, format := range constants.TimeFormats {
		startTime, err = time.ParseInLocation(format, startTimeStr, time.Local)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil
	}

	for _, format := range constants.TimeFormats {
		endTime, err = time.ParseInLocation(format, endTimeStr, time.Local)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil
	}

	for _, c := range s.courseCache {
		if c.Coach.Name == coachName && c.StartTime.Equal(startTime) && c.EndTime.Equal(endTime) {
			return nil
		}
	}

	return item
}

func (s *CourseService) DuplicatedCheck(params [][]interface{}) [][]interface{} {
	s.updateCache()
	var notExisted [][]interface{}
	for _, item := range params {
		if dup := s.courseExist(item); dup != nil {
			notExisted = append(notExisted, dup)
		}
	}
	return notExisted
}

func (s *CourseService) updateCache() {
	now := time.Now()
	firstDayOfThisMonth := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
	firstDayOfNextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())

	var courses []model.Course
	s.db.Where("start_time >= ? AND start_time < ? AND deleted_at IS NULL", firstDayOfThisMonth, firstDayOfNextMonth).
		Find(&courses)

	s.loadCoachesForCourses(courses)
	s.courseCache = courses
}

func (s *CourseService) loadCoachesForCourses(courses []model.Course) {
	if len(courses) == 0 {
		return
	}

	var coachIDs []uint64
	for _, course := range courses {
		if course.CoachID > 0 {
			coachIDs = append(coachIDs, course.CoachID)
		}
	}
	if len(coachIDs) == 0 {
		return
	}

	var coaches []model.Coach
	if err := s.db.Where("coach_id IN ?", coachIDs).Find(&coaches).Error; err != nil {
		return
	}

	coachMap := make(map[uint64]*model.Coach)
	for i := range coaches {
		coachMap[coaches[i].ID] = &coaches[i]
	}
	for i := range courses {
		if coach, ok := coachMap[courses[i].CoachID]; ok {
			courses[i].Coach = *coach
		}
	}
}
