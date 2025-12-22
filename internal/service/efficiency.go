package service

import (
	"ledong-db/internal/constants"
	"ledong-db/internal/database"
	"sort"
	"time"

	"gorm.io/gorm"
)

type EfficiencyService struct {
	db *gorm.DB
}

func NewEfficiencyService() *EfficiencyService {
	return &EfficiencyService{db: database.DB}
}

type AnalyseData struct {
	WorkTime float32 `json:"workTime"`
	Courses  float32 `json:"courses"`
	Members  float32 `json:"members"`
	Analyse  float32 `json:"analyse"`
	Trial    float32 `json:"trial"`
	Deal     float32 `json:"deal"`
}

type RevenueData struct {
	Spend   float32 `json:"spend"`
	Charge  float32 `json:"charge"`
	Equival float32 `json:"equival"`
}

type EfficiencyResponse struct {
	Analyse []map[string]AnalyseData `json:"analyse"`
	Revenue []map[string]RevenueData `json:"revenue"`
}

type chargeStat struct {
	Court  string
	Coach  string
	Charge float32
}

type courtEquivalStat struct {
	Court   string
	Equival float32
}

type courseStat struct {
	CoachName string
	CourtName string
	WorkTime  float32
	Courses   float32
	Members   float32
	Trial     float32
	Deal      float32
	Spend     float32
}

func parseTime(timeStr string) (time.Time, error) {
	for _, format := range constants.TimeFormats {
		if t, err := time.ParseInLocation(format, timeStr, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, gorm.ErrInvalidData
}

func (s *EfficiencyService) getChargeStats(startTime, endTime time.Time) ([]chargeStat, error) {
	var stats []chargeStat

	err := s.db.Table("charge").
		Select("charge.court, COALESCE(coach.name, '') as coach, SUM(COALESCE(NULLIF(charge.charge, 0), charge.worth)) as charge").
		Joins("LEFT JOIN coach ON charge.coach_id = coach.coach_id").
		Where("charge.charged_time >= ? AND charge.charged_time <= ?", startTime, endTime).
		Where("(charge.coach_id IS NULL OR coach.is_active > 0)").
		Group("charge.court, coach.name").
		Scan(&stats).Error

	return stats, err
}

func (s *EfficiencyService) getCourtEquival() ([]courtEquivalStat, error) {
	var stats []courtEquivalStat

	err := s.db.Table("prepaid_card").
		Select("court, SUM(equivalent_balance + rest_charge) as equival").
		Where("deleted_at IS NULL").
		Group("court").
		Scan(&stats).Error

	return stats, err
}

func (s *EfficiencyService) getCourseStats(startTime, endTime time.Time) ([]courseStat, error) {
	type courseDetail struct {
		CoachName  string
		CourtName  string
		CourseType int
		Duration   float32
		Quantities int
		Spend      float32
	}

	var details []courseDetail

	err := s.db.Table("course").
		Select(`
			COALESCE(coach.name, '') as coach_name,
			COALESCE(court.name, '') as court_name,
			course.course_type,
			course.duration,
			COALESCE(spend_sum.quantities_sum, 0) as quantities,
			COALESCE(spend_sum.spend_amount, 0) as spend
		`).
		Joins("LEFT JOIN coach ON course.coach_id = coach.coach_id").
		Joins("LEFT JOIN court ON course.court_id = court.id").
		Joins(`LEFT JOIN (
			SELECT 
				course_id,
				SUM(quantities) as quantities_sum,
				SUM(COALESCE(NULLIF(charge, 0), description)) as spend_amount
			FROM spend
			WHERE deleted_at IS NULL
			GROUP BY course_id
		) spend_sum ON course.id = spend_sum.course_id`).
		Where("course.start_time >= ? AND course.start_time <= ? AND course.deleted_at IS NULL", startTime, endTime).
		Where("(course.coach_id IS NULL OR coach.is_active > 0)").
		Scan(&details).Error

	if err != nil {
		return nil, err
	}

	statsMap := make(map[string]*courseStat)

	for _, detail := range details {
		coachKey := detail.CoachName
		courtKey := detail.CourtName

		if detail.CourseType < 0 {
			if coachKey != "" {
				if stat, ok := statsMap["coach:"+coachKey]; ok {
					stat.WorkTime += detail.Duration
					stat.Trial += 1
					if detail.CourseType == -1 {
						stat.Deal += 1
					}
				} else {
					deal := float32(0)
					if detail.CourseType == -1 {
						deal = 1
					}
					statsMap["coach:"+coachKey] = &courseStat{
						CoachName: coachKey,
						WorkTime:  detail.Duration,
						Courses:   0,
						Members:   0,
						Trial:     1,
						Deal:      deal,
						Spend:     0,
					}
				}
			}

			if courtKey != "" {
				if stat, ok := statsMap["court:"+courtKey]; ok {
					stat.WorkTime += detail.Duration
					stat.Trial += 1
					if detail.CourseType == -1 {
						stat.Deal += 1
					}
				} else {
					deal := float32(0)
					if detail.CourseType == -1 {
						deal = 1
					}
					statsMap["court:"+courtKey] = &courseStat{
						CourtName: courtKey,
						WorkTime:  detail.Duration,
						Courses:   0,
						Members:   0,
						Trial:     1,
						Deal:      deal,
						Spend:     0,
					}
				}
			}
		} else if detail.CourseType > 0 {
			courses := float32(0)
			members := float32(0)
			if detail.Quantities > 0 {
				courses = 1
				if detail.Quantities > 1 {
					members = float32(detail.Quantities)
				} else {
					members = float32(detail.CourseType) * float32(detail.Quantities)
				}
			}

			if coachKey != "" {
				if stat, ok := statsMap["coach:"+coachKey]; ok {
					stat.WorkTime += detail.Duration
					stat.Courses += courses
					stat.Members += members
					stat.Spend += detail.Spend
				} else {
					statsMap["coach:"+coachKey] = &courseStat{
						CoachName: coachKey,
						WorkTime:  detail.Duration,
						Courses:   courses,
						Members:   members,
						Trial:     0,
						Deal:      0,
						Spend:     detail.Spend,
					}
				}
			}

			if courtKey != "" {
				if stat, ok := statsMap["court:"+courtKey]; ok {
					stat.WorkTime += detail.Duration
					stat.Courses += courses
					stat.Members += members
					stat.Spend += detail.Spend
				} else {
					statsMap["court:"+courtKey] = &courseStat{
						CourtName: courtKey,
						WorkTime:  detail.Duration,
						Courses:   courses,
						Members:   members,
						Trial:     0,
						Deal:      0,
						Spend:     detail.Spend,
					}
				}
			}
		}
	}

	var stats []courseStat
	for _, stat := range statsMap {
		stats = append(stats, *stat)
	}

	return stats, nil
}

func (s *EfficiencyService) AnalyseEfficiency(startTime, endTime string) (*EfficiencyResponse, error) {
	start, err := parseTime(startTime)
	if err != nil {
		return nil, err
	}

	end, err := parseTime(endTime)
	if err != nil {
		return nil, err
	}

	chargeStats, err := s.getChargeStats(start, end)
	if err != nil {
		return nil, err
	}

	equivalStats, err := s.getCourtEquival()
	if err != nil {
		return nil, err
	}

	courseStats, err := s.getCourseStats(start, end)
	if err != nil {
		return nil, err
	}

	analysMap := make(map[string]*AnalyseData)
	revenueMap := make(map[string]*RevenueData)

	for _, stat := range chargeStats {
		if stat.Court != "" {
			if item, ok := revenueMap[stat.Court]; ok {
				item.Charge += stat.Charge
			} else {
				revenueMap[stat.Court] = &RevenueData{
					Spend:   0,
					Charge:  stat.Charge,
					Equival: 0,
				}
			}
		}

		if stat.Coach != "" {
			if item, ok := revenueMap[stat.Coach]; ok {
				item.Charge += stat.Charge
			} else {
				revenueMap[stat.Coach] = &RevenueData{
					Spend:   0,
					Charge:  stat.Charge,
					Equival: 0,
				}
			}
		}
	}

	for _, stat := range equivalStats {
		if item, ok := revenueMap[stat.Court]; ok {
			item.Equival += stat.Equival
		} else {
			revenueMap[stat.Court] = &RevenueData{
				Spend:   0,
				Charge:  0,
				Equival: stat.Equival,
			}
		}
	}

	for _, stat := range courseStats {
		if stat.CoachName != "" {
			key := stat.CoachName
			if item, ok := analysMap[key]; ok {
				item.WorkTime += stat.WorkTime
				item.Courses += stat.Courses
				item.Members += stat.Members
				item.Trial += stat.Trial
				item.Deal += stat.Deal
				if item.Courses > 0 {
					item.Analyse = item.Members / item.Courses
				} else {
					item.Analyse = 0
				}
			} else {
				analyse := float32(0)
				if stat.Courses > 0 {
					analyse = stat.Members / stat.Courses
				}
				analysMap[key] = &AnalyseData{
					WorkTime: stat.WorkTime,
					Courses:  stat.Courses,
					Members:  stat.Members,
					Analyse:  analyse,
					Trial:    stat.Trial,
					Deal:     stat.Deal,
				}
			}

			if item, ok := revenueMap[key]; ok {
				item.Spend += stat.Spend
			} else {
				revenueMap[key] = &RevenueData{
					Spend:   stat.Spend,
					Charge:  0,
					Equival: 0,
				}
			}
		}

		if stat.CourtName != "" {
			key := stat.CourtName
			if item, ok := analysMap[key]; ok {
				item.WorkTime += stat.WorkTime
				item.Courses += stat.Courses
				item.Members += stat.Members
				item.Trial += stat.Trial
				item.Deal += stat.Deal
				if item.Courses > 0 {
					item.Analyse = item.Members / item.Courses
				} else {
					item.Analyse = 0
				}
			} else {
				analyse := float32(0)
				if stat.Courses > 0 {
					analyse = stat.Members / stat.Courses
				}
				analysMap[key] = &AnalyseData{
					WorkTime: stat.WorkTime,
					Courses:  stat.Courses,
					Members:  stat.Members,
					Analyse:  analyse,
					Trial:    stat.Trial,
					Deal:     stat.Deal,
				}
			}

			if item, ok := revenueMap[key]; ok {
				item.Spend += stat.Spend
			} else {
				revenueMap[key] = &RevenueData{
					Spend:   stat.Spend,
					Charge:  0,
					Equival: 0,
				}
			}
		}
	}

	type analyseEntry struct {
		key  string
		data AnalyseData
	}

	var analyseEntries []analyseEntry
	for key, item := range analysMap {
		if item.Courses > 0 {
			item.Analyse = item.Members / item.Courses
		} else {
			item.Analyse = 0
		}
		analyseEntries = append(analyseEntries, analyseEntry{key: key, data: *item})
	}

	sort.Slice(analyseEntries, func(i, j int) bool {
		return analyseEntries[i].data.Analyse < analyseEntries[j].data.Analyse
	})

	for i, j := 0, len(analyseEntries)-1; i < j; i, j = i+1, j-1 {
		analyseEntries[i], analyseEntries[j] = analyseEntries[j], analyseEntries[i]
	}

	var analyseList []map[string]AnalyseData
	for _, entry := range analyseEntries {
		analyseList = append(analyseList, map[string]AnalyseData{entry.key: entry.data})
	}

	type revenueEntry struct {
		key  string
		data RevenueData
	}

	totalData := &RevenueData{
		Spend:   0,
		Charge:  0,
		Equival: 0,
	}

	var revenueEntries []revenueEntry
	for key, item := range revenueMap {
		if len(key) >= 6 && key[len(key)-6:] == "校区" {
			totalData.Spend += item.Spend
			totalData.Charge += item.Charge
			totalData.Equival += item.Equival
		}
		revenueEntries = append(revenueEntries, revenueEntry{key: key, data: *item})
	}

	revenueEntries = append(revenueEntries, revenueEntry{key: "总共", data: *totalData})

	sort.Slice(revenueEntries, func(i, j int) bool {
		return revenueEntries[i].data.Spend < revenueEntries[j].data.Spend
	})

	for i, j := 0, len(revenueEntries)-1; i < j; i, j = i+1, j-1 {
		revenueEntries[i], revenueEntries[j] = revenueEntries[j], revenueEntries[i]
	}

	var revenueList []map[string]RevenueData
	for _, entry := range revenueEntries {
		revenueList = append(revenueList, map[string]RevenueData{entry.key: entry.data})
	}

	return &EfficiencyResponse{
		Analyse: analyseList,
		Revenue: revenueList,
	}, nil
}
