package job

import (
	"testing"
	"time"
)

func TestJobCanRepresentMissingSalaryAndPostedAt(t *testing.T) {
	j := Job{
		Source:     "greenhouse",
		ExternalID: "123",
		Company:    "Example Product",
		Title:      "Backend Engineer",
		URL:        "https://example.com/jobs/123",
	}

	if j.SalaryMin != nil {
		t.Fatal("expected missing salary minimum to be nil")
	}

	if j.SalaryMax != nil {
		t.Fatal("expected missing salary maximum to be nil")
	}

	if j.PostedAt != nil {
		t.Fatal("expected missing posted date to be nil")
	}

	j.PostedAt = ptrTime(time.Now())

	if j.PostedAt == nil {
		t.Fatal("expected posted date to be set")
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
