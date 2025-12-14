package service

import (
	"ledong-db/internal/database"
	"ledong-db/internal/model"
	"time"

	"gorm.io/gorm"
)

type CourseService struct {
	db *gorm.DB
}

func NewCourseService() *CourseService {
	return &CourseService{db: database.DB}
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
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, format := range formats {
		parsedTime, err = time.Parse(format, startTime)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}

	query := s.db.Model(&model.Course{}).
		Where("start_time >= ?", parsedTime).
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
