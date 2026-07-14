package service

import (
	"encoding/json"
	"errors"
	"testing"

	"ledong-db/internal/model"
)

func pendingIntPointer(value int) *int { return &value }

func validPendingInput() PendingCourseInput {
	return PendingCourseInput{
		CoachID:    12,
		CourtID:    3,
		StartTime:  "2026-07-12 19:00:00",
		EndTime:    "2026-07-12 20:30:00",
		Duration:   1.5,
		CourseType: pendingIntPointer(1),
		IsAdult:    pendingIntPointer(1),
		MembersData: []MemberSpendInput{{
			MemberID:    123,
			Charge:      0,
			Times:       1,
			AnnualTimes: 0,
			Description: 200,
			Quantities:  1,
		}},
	}
}

func TestValidatePendingCourseInput(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*PendingCourseInput)
		wantErr string
	}{
		{name: "valid class course"},
		{
			name: "missing course type",
			mutate: func(input *PendingCourseInput) {
				input.CourseType = nil
			},
			wantErr: PendingErrorInvalidRequest,
		},
		{
			name: "booking defaults adult value",
			mutate: func(input *PendingCourseInput) {
				input.CourseType = pendingIntPointer(0)
				input.IsAdult = nil
			},
		},
		{
			name: "experience has no members",
			mutate: func(input *PendingCourseInput) {
				input.CourseType = pendingIntPointer(-2)
				input.MembersData = nil
			},
		},
		{
			name: "cross day",
			mutate: func(input *PendingCourseInput) {
				input.EndTime = "2026-07-13 00:30:00"
				input.Duration = 5.5
			},
			wantErr: PendingErrorInvalidRequest,
		},
		{
			name: "not half hour boundary",
			mutate: func(input *PendingCourseInput) {
				input.StartTime = "2026-07-12 19:15:00"
				input.Duration = 1.25
			},
			wantErr: PendingErrorInvalidRequest,
		},
		{
			name: "non-contract time format",
			mutate: func(input *PendingCourseInput) {
				input.StartTime = "2026-07-12T19:00:00"
			},
			wantErr: PendingErrorInvalidRequest,
		},
		{
			name: "duration mismatch",
			mutate: func(input *PendingCourseInput) {
				input.Duration = 1
			},
			wantErr: PendingErrorInvalidRequest,
		},
		{
			name: "duplicate member",
			mutate: func(input *PendingCourseInput) {
				input.MembersData = append(input.MembersData, input.MembersData[0])
			},
			wantErr: PendingErrorDuplicateMember,
		},
		{
			name: "times below half step",
			mutate: func(input *PendingCourseInput) {
				input.MembersData[0].Times = 0.3
			},
			wantErr: PendingErrorInvalidMemberSpend,
		},
		{
			name: "multiple deduction types",
			mutate: func(input *PendingCourseInput) {
				input.MembersData[0].Charge = 10
			},
			wantErr: PendingErrorInvalidMemberSpend,
		},
		{
			name: "charge description mismatch",
			mutate: func(input *PendingCourseInput) {
				input.MembersData[0].Times = 0
				input.MembersData[0].Charge = 10
				input.MembersData[0].Description = 9
			},
			wantErr: PendingErrorInvalidMemberSpend,
		},
		{
			name: "invalid quantities",
			mutate: func(input *PendingCourseInput) {
				input.MembersData[0].Quantities = 0
			},
			wantErr: PendingErrorInvalidMemberSpend,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validPendingInput()
			if test.mutate != nil {
				test.mutate(&input)
			}
			_, err := validatePendingCourseInput(input)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected %s", test.wantErr)
			}
			var pendingErr *PendingCourseError
			if !errors.As(err, &pendingErr) {
				t.Fatalf("unexpected error type: %T", err)
			}
			if pendingErr.Code != test.wantErr {
				t.Fatalf("got %s, want %s", pendingErr.Code, test.wantErr)
			}
		})
	}
}

func TestValidatePendingCourseInputBookingPreservesAdult(t *testing.T) {
	input := validPendingInput()
	input.CourseType = pendingIntPointer(0)
	input.IsAdult = pendingIntPointer(0)

	validated, err := validatePendingCourseInput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if validated.IsAdult == nil || *validated.IsAdult != 0 {
		t.Fatalf("booking adult value = %v, want 0", validated.IsAdult)
	}
}

func TestSameSecond(t *testing.T) {
	left, _ := parsePendingBusinessTime("2026-07-12 18:30:00")
	right := left.Add(999999999)
	if !sameSecond(left, right) {
		t.Fatal("timestamps in the same second must match")
	}
	if sameSecond(left, left.Add(1_000_000_000)) {
		t.Fatal("timestamps in different seconds must not match")
	}
}

func TestDecodeStoredPendingMembers(t *testing.T) {
	raw := []byte(`[{"member_id":123,"charge":0,"times":0,"annual_times":1.5,"description":200,"quantities":2}]`)
	members, err := decodeStoredPendingMembers(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("got %d members, want 1", len(members))
	}
	member := members[0]
	if member.MemberID != 123 || member.AnnualTimes != 1.5 || member.Quantities != 2 {
		t.Fatalf("unexpected decoded member: %+v", member)
	}
}

func TestBuildLegacyMembersJSON(t *testing.T) {
	members := []MemberSpendInput{{
		MemberID:    123,
		Charge:      0,
		Times:       1,
		AnnualTimes: 0,
		Description: 200,
		Quantities:  2,
	}}
	memberMap := map[uint64]model.PrepaidCard{
		123: {ID: 123, Number: "13800000000"},
	}
	raw, err := buildLegacyMembersJSON(members, memberMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var decoded map[string][]float64
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	values := decoded["13800000000"]
	if len(values) != 5 || values[1] != 1 || values[3] != 200 || values[4] != 2 {
		t.Fatalf("unexpected legacy values: %#v", values)
	}
}
