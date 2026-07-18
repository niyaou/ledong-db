package handler

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"ledong-db/internal/service"

	"github.com/gin-gonic/gin"
)

func TestCoachCoursesRequiresSecureHeader(t *testing.T) {
	setCoachCourseTestSecret(t, "test-secret")
	response := invokeCoachCoursesHandler("12", "month=2026-07", "wrong-secret", nil)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestCoachCoursesValidatesRequestBeforeDatabaseAccess(t *testing.T) {
	setCoachCourseTestSecret(t, "test-secret")
	tests := []struct {
		name    string
		coachID string
		query   string
	}{
		{name: "invalid coach", coachID: "abc", query: "month=2026-07"},
		{name: "zero coach", coachID: "0", query: "month=2026-07"},
		{name: "missing month", coachID: "12", query: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := invokeCoachCoursesHandler(test.coachID, test.query, "test-secret", nil)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestCoachCoursesRejectsInvalidMonth(t *testing.T) {
	setCoachCourseTestSecret(t, "test-secret")
	response := invokeCoachCoursesHandler("12", "month=2026-13", "test-secret", service.NewCourseService(nil, nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func invokeCoachCoursesHandler(coachID, query, secure string, svc *service.CourseService) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest(http.MethodGet, "/api/prepaidCard/course/coach/"+coachID+"?"+query, nil)
	request.Header.Set("secure", secure)
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = request
	context.Params = gin.Params{{Key: "coachId", Value: coachID}}
	(&CourseHandler{service: svc}).CoachCourses(context)
	return response
}

func setCoachCourseTestSecret(t *testing.T, secret string) {
	t.Helper()
	secretKey = secret
	secretKeyOnce = sync.Once{}
	secretKeyOnce.Do(func() {})
	t.Cleanup(func() {
		secretKey = ""
		secretKeyOnce = sync.Once{}
	})
}
