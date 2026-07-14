package constants

import (
	"time"
	_ "time/tzdata"
)

const (
	BusinessTimeZone   = "Asia/Shanghai"
	BusinessTimeOffset = "+08:00"
)

var TimeFormats = []string{
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02T15:04:05",
	"2006-01-02",
}

// ConfigureBusinessTime makes every existing time.Local-based legacy flow use
// the business timezone before database connections and services are created.
func ConfigureBusinessTime() error {
	location, err := time.LoadLocation(BusinessTimeZone)
	if err != nil {
		return err
	}
	time.Local = location
	return nil
}
