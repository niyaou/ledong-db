package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// PendingJSON stores the members_data JSON column without introducing an
// additional GORM datatype dependency.
type PendingJSON json.RawMessage

// Scan implements sql.Scanner.
func (j *PendingJSON) Scan(value any) error {
	if j == nil {
		return errors.New("PendingJSON: Scan on nil receiver")
	}

	switch raw := value.(type) {
	case nil:
		*j = PendingJSON([]byte("[]"))
		return nil
	case []byte:
		return j.assign(raw)
	case string:
		return j.assign([]byte(raw))
	default:
		return fmt.Errorf("PendingJSON: unsupported Scan type %T", value)
	}
}

func (j *PendingJSON) assign(raw []byte) error {
	if len(raw) == 0 {
		raw = []byte("[]")
	}
	if !json.Valid(raw) {
		return errors.New("PendingJSON: invalid JSON")
	}
	copied := make([]byte, len(raw))
	copy(copied, raw)
	*j = PendingJSON(copied)
	return nil
}

// Value implements driver.Valuer.
func (j PendingJSON) Value() (driver.Value, error) {
	raw := []byte(j)
	if len(raw) == 0 {
		raw = []byte("[]")
	}
	if !json.Valid(raw) {
		return nil, errors.New("PendingJSON: invalid JSON")
	}
	return raw, nil
}

// PendingCourse is the short-lived message written by the coach mini-program
// and physically deleted after a successful formal course creation.
type PendingCourse struct {
	ID          uint64      `gorm:"primaryKey;column:id" json:"id"`
	CoachID     uint64      `gorm:"column:coach_id;index" json:"coachId"`
	CourtID     uint64      `gorm:"column:court_id;index" json:"courtId"`
	StartTime   time.Time   `gorm:"column:start_time;index" json:"startTime"`
	EndTime     time.Time   `gorm:"column:end_time" json:"endTime"`
	Duration    float32     `gorm:"column:duration" json:"duration"`
	CourseType  int         `gorm:"column:course_type" json:"courseType"`
	IsAdult     *int        `gorm:"column:is_adult;default:1" json:"isAdult"`
	Description string      `gorm:"column:description" json:"description"`
	MembersData PendingJSON `gorm:"column:members_data;type:json" json:"membersData"`
	CreatedAt   time.Time   `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time   `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName specifies the physical queue table.
func (PendingCourse) TableName() string {
	return "pending_course"
}
