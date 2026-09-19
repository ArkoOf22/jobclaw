package jobspy

import (
	"testing"
	"time"
)

// theCandidatesRoles mirrors config/candidate.yaml: four primary roles followed
// by thirteen secondary ones. The exact list matters to these tests because the
// bug being guarded against was silent truncation of everything past the fifth
// entry.
func theCandidatesRoles() []string {
	return []string{
		// primary
		"Software Development Engineer",
		"Backend Engineer",
		"SDE I",
		"SDE II",
		// secondary
		"Software Engineer",
		"Software Engineer I",
		"Software Engineer II",
		"Software Development Engineer II",
		"Platform Engineer",
		"Distributed Systems Engineer",
		"Backend Developer",
		"Backend Software Engineer",
		"Golang Developer",
		"Go Developer",
		"Member of Technical Staff",
		"Product Engineer",
	}
}

func TestSearchTermsNeverExceedsTheCap(t *testing.T) {
	roles := theCandidatesRoles()

	// A full cycle and then some, so wrapping is exercised.
	for window := 0; window < 40; window++ {
		at := time.Unix(int64(window)*int64(termRotationPeriod/time.Second), 0)

		terms := searchTerms(roles, at)

		if len(terms) > maxSearchTerms {
			t.Fatalf(
				"window %d: got %d terms, cap is %d: %v",
				window,
				len(terms),
				maxSearchTerms,
				terms,
			)
		}

		if len(terms) == 0 {
			t.Fatalf("window %d: got no terms", window)
		}
	}
}

func TestSearchTermsAlwaysIncludesThePinnedPrefix(t *testing.T) {
	roles := theCandidatesRoles()

	// The pinned terms are the primary roles and the most productive queries. A
	// failed run must not push them a full cycle away.
	for window := 0; window < 40; window++ {
		at := time.Unix(int64(window)*int64(termRotationPeriod/time.Second), 0)

		terms := searchTerms(roles, at)

		for i := 0; i < pinnedSearchTerms; i++ {
			if terms[i] != roles[i] {
				t.Fatalf(
					"window %d: position %d is %q, want pinned %q (terms: %v)",
					window,
					i,
					terms[i],
					roles[i],
					terms,
				)
			}
		}
	}
}

// This is the regression test for the actual defect. Twelve configured roles were
// never searched even once, because selection truncated instead of rotating.
func TestSearchTermsEventuallyCoversEveryConfiguredRole(t *testing.T) {
	roles := theCandidatesRoles()

	seen := make(map[string]bool, len(roles))

	// Four scheduled runs a day; two days is comfortably a full cycle.
	const windows = 8

	for window := 0; window < windows; window++ {
		at := time.Unix(int64(window)*int64(termRotationPeriod/time.Second), 0)

		for _, term := range searchTerms(roles, at) {
			seen[term] = true
		}
	}

	var missed []string

	for _, role := range roles {
		if !seen[role] {
			missed = append(missed, role)
		}
	}

	if len(missed) > 0 {
		t.Errorf(
			"these roles were never searched across %d windows: %v",
			windows,
			missed,
		)
	}
}

// Go is the candidate's primary language, and these two terms sat past the old
// truncation point, so they had never been searched. Called out separately from
// the coverage test so a regression names the symptom rather than a count.
func TestSearchTermsReachesTheGoRoles(t *testing.T) {
	roles := theCandidatesRoles()

	wanted := map[string]bool{
		"Golang Developer": false,
		"Go Developer":     false,
	}

	for window := 0; window < 8; window++ {
		at := time.Unix(int64(window)*int64(termRotationPeriod/time.Second), 0)

		for _, term := range searchTerms(roles, at) {
			if _, ok := wanted[term]; ok {
				wanted[term] = true
			}
		}
	}

	for term, found := range wanted {
		if !found {
			t.Errorf("%q was never searched", term)
		}
	}
}

func TestSearchTermsAdvancesBetweenConsecutiveRuns(t *testing.T) {
	roles := theCandidatesRoles()

	first := searchTerms(roles, time.Unix(0, 0))

	second := searchTerms(
		roles,
		time.Unix(int64(termRotationPeriod/time.Second), 0),
	)

	// The pinned prefix is shared by design; the rotating tail must differ, or
	// the rotation is not doing anything.
	firstTail := first[pinnedSearchTerms:]
	secondTail := second[pinnedSearchTerms:]

	identical := len(firstTail) == len(secondTail)

	if identical {
		for i := range firstTail {
			if firstTail[i] != secondTail[i] {
				identical = false

				break
			}
		}
	}

	if identical {
		t.Errorf(
			"consecutive runs searched the same rotating terms %v; rotation is not advancing",
			firstTail,
		)
	}
}

func TestSearchTermsIsStableWithinOneWindow(t *testing.T) {
	roles := theCandidatesRoles()

	base := int64(termRotationPeriod / time.Second)

	// Two moments inside the same six-hour window must choose the same slice, so
	// a retry after a transient JobSpy failure repeats the run rather than
	// skipping ahead and leaving a gap.
	early := searchTerms(roles, time.Unix(base*3, 0))
	late := searchTerms(roles, time.Unix(base*3+int64(5*time.Hour/time.Second), 0))

	if len(early) != len(late) {
		t.Fatalf("same window produced %d and %d terms", len(early), len(late))
	}

	for i := range early {
		if early[i] != late[i] {
			t.Fatalf(
				"same window produced different terms at position %d: %q vs %q",
				i,
				early[i],
				late[i],
			)
		}
	}
}

func TestSearchTermsReturnsEverythingWhenItFits(t *testing.T) {
	// Rotation would only introduce gaps if the whole list already fits, so it
	// must switch itself off.
	roles := []string{"Backend Engineer", "Go Developer"}

	terms := searchTerms(roles, time.Unix(0, 0))

	if len(terms) != len(roles) {
		t.Fatalf("got %d terms, want %d: %v", len(terms), len(roles), terms)
	}

	for i := range roles {
		if terms[i] != roles[i] {
			t.Errorf("position %d: got %q, want %q", i, terms[i], roles[i])
		}
	}
}

func TestSearchTermsCleansInput(t *testing.T) {
	tests := []struct {
		name     string
		keywords []string
		want     []string
	}{
		{
			name:     "trims whitespace",
			keywords: []string{"  Backend Engineer  ", "Go Developer"},
			want:     []string{"Backend Engineer", "Go Developer"},
		},
		{
			name:     "drops blanks",
			keywords: []string{"Backend Engineer", "   ", "", "Go Developer"},
			want:     []string{"Backend Engineer", "Go Developer"},
		},
		{
			name:     "de-duplicates case-insensitively, keeping first spelling",
			keywords: []string{"Backend Engineer", "backend engineer", "Go Developer"},
			want:     []string{"Backend Engineer", "Go Developer"},
		},
		{
			name:     "no keywords yields none",
			keywords: []string{"  ", ""},
			want:     []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := searchTerms(test.keywords, time.Unix(0, 0))

			if len(got) != len(test.want) {
				t.Fatalf("got %v, want %v", got, test.want)
			}

			for i := range test.want {
				if got[i] != test.want[i] {
					t.Errorf("position %d: got %q, want %q", i, got[i], test.want[i])
				}
			}
		})
	}
}

// Duplicates must be removed before the window is computed, otherwise a repeated
// keyword would shrink an already-tight window.
func TestSearchTermsDoesNotRepeatATermWithinARun(t *testing.T) {
	roles := theCandidatesRoles()

	for window := 0; window < 40; window++ {
		at := time.Unix(int64(window)*int64(termRotationPeriod/time.Second), 0)

		terms := searchTerms(roles, at)

		seen := make(map[string]bool, len(terms))

		for _, term := range terms {
			if seen[term] {
				t.Fatalf("window %d: %q searched twice in one run: %v", window, term, terms)
			}

			seen[term] = true
		}
	}
}
