package job

import "testing"

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name string
		from Status
		to   Status
		want bool
	}{
		{"discovered to scored", StatusDiscovered, StatusScored, true},
		{"discovered to shortlisted", StatusDiscovered, StatusShortlisted, true},
		{"scored to shortlisted", StatusScored, StatusShortlisted, true},
		{"shortlisted to approved", StatusShortlisted, StatusApproved, true},
		{"approved to applied", StatusApproved, StatusApplied, true},
		{"applied to interview", StatusApplied, StatusInterview, true},
		{"interview to offer", StatusInterview, StatusOffer, true},

		{"applied cannot go back to scored", StatusApplied, StatusScored, false},
		{"interview cannot go back to scored", StatusInterview, StatusScored, false},
		{"offer cannot go back", StatusOffer, StatusScored, false},
		{"rejected cannot go back", StatusRejected, StatusScored, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanTransition(tt.from, tt.to); got != tt.want {
				t.Fatalf(
					"CanTransition(%q, %q) = %v, want %v",
					tt.from,
					tt.to,
					got,
					tt.want,
				)
			}
		})
	}
}
