package discovery

import (
	"testing"

	"jobclaw/internal/job"
)

func TestMatchesRequestFiltersIrrelevantJobs(t *testing.T) {
	candidate := job.Job{
		Title:       "Account Executive",
		Description: "Own enterprise sales relationships.",
		Location:    "San Francisco",
	}

	if MatchesRequest(candidate, Request{
		Keywords: []string{"Backend Engineer", "Golang"},
	}) {
		t.Fatal("expected irrelevant sales job to be excluded")
	}
}

func TestMatchesRequestMatchesBackendJob(t *testing.T) {
	candidate := job.Job{
		Title:       "Senior Backend Engineer",
		Description: "Build distributed backend systems.",
		Location:    "Bengaluru, India",
	}

	if !MatchesRequest(candidate, Request{
		Keywords:  []string{"Backend Engineer", "Golang"},
		Locations: []string{"Bengaluru"},
	}) {
		t.Fatal("expected backend job to match")
	}
}

func TestMatchesRequestMatchesDescriptionKeyword(t *testing.T) {
	candidate := job.Job{
		Title:       "Software Engineer",
		Description: "Build high-performance services using Golang.",
		Location:    "Bangalore, India",
	}

	if !MatchesRequest(candidate, Request{
		Keywords:  []string{"Golang"},
		Locations: []string{"Bangalore"},
	}) {
		t.Fatal("expected Golang description match")
	}
}

func TestMatchesRequestDoesNotMatchEngineeringManagerForBackendEngineer(
	t *testing.T,
) {
	candidate := job.Job{
		Title:       "ARG Engineering Manager",
		Description: "Lead a platform engineering organization.",
		Location:    "US - Remote",
	}

	if MatchesRequest(candidate, Request{
		Keywords: []string{"Backend Engineer"},
	}) {
		t.Fatal("expected Engineering Manager not to match Backend Engineer")
	}
}

func TestMatchesRequestDoesNotMatchAccountExecutiveForBackendEngineer(
	t *testing.T,
) {
	candidate := job.Job{
		Title:       "Account Executive",
		Description: "Own enterprise customer relationships.",
		Location:    "Bengaluru",
	}

	if MatchesRequest(candidate, Request{
		Keywords: []string{"Backend Engineer"},
	}) {
		t.Fatal("expected Account Executive not to match Backend Engineer")
	}
}

func TestMatchesRequestMatchesTokensAcrossPunctuation(t *testing.T) {
	candidate := job.Job{
		Title:       "Backend-Engineer",
		Description: "Build distributed systems.",
		Location:    "Bengaluru",
	}

	if !MatchesRequest(candidate, Request{
		Keywords: []string{"Backend Engineer"},
	}) {
		t.Fatal("expected punctuation-normalized role match")
	}
}

func TestMatchesRequestMatchesGolangKeyword(t *testing.T) {
	candidate := job.Job{
		Title:       "Software Engineer",
		Description: "Build services in Go and Golang.",
		Location:    "Bengaluru",
	}

	if !MatchesRequest(candidate, Request{
		Keywords: []string{"Golang"},
	}) {
		t.Fatal("expected Golang keyword to match")
	}
}

func TestMatchesRequestDoesNotMatchRoleMentionedOnlyInDescription(
	t *testing.T,
) {
	candidate := job.Job{
		Title: "Product Counsel",
		Description: `
			Partner with Backend Engineers and the infrastructure
			organization to support product development.
		`,
		Location: "US Remote",
	}

	if MatchesRequest(candidate, Request{
		Keywords: []string{"Backend Engineer"},
	}) {
		t.Fatal(
			"expected non-backend role to be excluded when role appears only in description",
		)
	}
}

func TestMatchesRequestMatchesSkillInDescription(t *testing.T) {
	candidate := job.Job{
		Title: "Software Engineer",
		Description: `
			Build distributed services and internal APIs using Golang.
		`,
		Location: "Bengaluru",
	}

	if !MatchesRequest(candidate, Request{
		Keywords: []string{"Golang"},
	}) {
		t.Fatal("expected skill keyword in description to match")
	}
}

func TestMatchesRequestFiltersLocationMismatch(t *testing.T) {
	candidate := job.Job{
		Title:    "Backend Engineer",
		Location: "San Francisco",
	}

	if MatchesRequest(candidate, Request{
		Keywords:  []string{"Backend Engineer"},
		Locations: []string{"Bengaluru"},
	}) {
		t.Fatal("expected location mismatch to be excluded")
	}
}

func TestMatchesRequestAllowsRemoteJobForPreferredLocation(t *testing.T) {
	candidate := job.Job{
		Title:    "Backend Engineer",
		Location: "Remote",
	}

	if !MatchesRequest(candidate, Request{
		Keywords:  []string{"Backend Engineer"},
		Locations: []string{"Bengaluru"},
	}) {
		t.Fatal("expected remote job to match preferred location")
	}
}

func TestMatchesRequestRemoteOnly(t *testing.T) {
	candidate := job.Job{
		Title:    "Backend Engineer",
		Location: "Bengaluru",
	}

	if MatchesRequest(candidate, Request{
		Keywords:   []string{"Backend Engineer"},
		RemoteOnly: true,
	}) {
		t.Fatal("expected non-remote job to fail remote-only request")
	}

	candidate.Location = "Bengaluru / Remote"

	if !MatchesRequest(candidate, Request{
		Keywords:   []string{"Backend Engineer"},
		RemoteOnly: true,
	}) {
		t.Fatal("expected remote job to match remote-only request")
	}
}
