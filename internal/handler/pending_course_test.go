package handler

import (
	"net/http"
	"testing"

	"ledong-db/internal/service"
)

func TestPendingCourseHTTPStatus(t *testing.T) {
	tests := map[string]int{
		service.PendingErrorInvalidRequest:                 http.StatusBadRequest,
		service.PendingErrorInvalidMemberSpend:             http.StatusBadRequest,
		service.PendingErrorDuplicateMember:                http.StatusBadRequest,
		service.PendingErrorUnauthorized:                   http.StatusUnauthorized,
		service.PendingErrorNotFound:                       http.StatusNotFound,
		service.PendingErrorCoachNotFound:                  http.StatusNotFound,
		service.PendingErrorCourtNotFound:                  http.StatusNotFound,
		service.PendingErrorMemberNotFound:                 http.StatusNotFound,
		service.PendingErrorUpdated:                        http.StatusConflict,
		service.PendingErrorCourseDuplicate:                http.StatusConflict,
		service.PendingErrorFormalCreatedPendingDeleteFail: http.StatusInternalServerError,
		service.PendingErrorInternal:                       http.StatusInternalServerError,
		"unexpected":                                       http.StatusInternalServerError,
	}

	for code, want := range tests {
		if got := pendingCourseHTTPStatus(code); got != want {
			t.Errorf("pendingCourseHTTPStatus(%q) = %d, want %d", code, got, want)
		}
	}
}
