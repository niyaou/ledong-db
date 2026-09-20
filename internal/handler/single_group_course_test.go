package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCreateCourseManualEntryValidatesSingleGroupFields(t *testing.T) {
	setCoachCourseTestSecret(t, "test-secret")
	tests := []struct {
		name   string
		values url.Values
	}{
		{
			name:   "requires participant count",
			values: validSingleGroupCourseForm(),
		},
		{
			name: "rejects members object",
			values: func() url.Values {
				values := validSingleGroupCourseForm()
				values.Set("participantCount", "4")
				values.Set("membersObj", `{}`)
				return values
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			request := httptest.NewRequest(http.MethodPost, "/api/prepaidCard/course/create", strings.NewReader(test.values.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.Header.Set("secure", "test-secret")
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = request

			(&CourseHandler{}).CreateCourse(context)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
			}
		})
	}
}

func validSingleGroupCourseForm() url.Values {
	return url.Values{
		"startTime":    {"2026-07-12 19:00:00"},
		"endTime":      {"2026-07-12 20:00:00"},
		"coachName":    {"coach-12"},
		"spendingTime": {"1"},
		"courtName":    {"中心校区"},
		"descript":     {"单次班课"},
		"courseType":   {"3"},
	}
}
