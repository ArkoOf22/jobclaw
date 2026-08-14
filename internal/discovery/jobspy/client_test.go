package jobspy

import (
	"context"
	"os"
	"testing"
	"time"

	"jobclaw/internal/discovery"
)

func TestClientImplementsSource(t *testing.T) {
	var _ discovery.Source = (*Client)(nil)
}

func TestClientDiscoverLive(t *testing.T) {
	if os.Getenv("JOBCLAW_JOBSPY_LIVE_TEST") != "1" {
		t.Skip("set JOBCLAW_JOBSPY_LIVE_TEST=1 to run against live JobSpy MCP server")
	}

	client := NewClient(os.Getenv("JOBCLAW_JOBSPY_URL"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	jobs, err := client.Discover(ctx, discovery.Request{
		Keywords:   []string{"golang backend engineer"},
		Locations:  []string{"Bengaluru, Karnataka, India"},
		RemoteOnly: false,
		HoursOld:   168,
		Limit:      3,
	})
	if err != nil {
		t.Fatalf("discover jobs: %v", err)
	}

	if len(jobs) == 0 {
		t.Fatal("expected at least one job")
	}

	for _, j := range jobs {
		t.Logf(
			"job: source=%s external_id=%s company=%s title=%s location=%s url=%s description_len=%d",
			j.Source,
			j.ExternalID,
			j.Company,
			j.Title,
			j.Location,
			j.URL,
			len(j.Description),
		)

		if j.Source == "" {
			t.Error("job source is empty")
		}
		if j.ExternalID == "" {
			t.Error("job external ID is empty")
		}
		if j.Company == "" {
			t.Error("job company is empty")
		}
		if j.Title == "" {
			t.Error("job title is empty")
		}
		if j.URL == "" {
			t.Error("job URL is empty")
		}
		if j.Description == "" {
			t.Error("job description is empty")
		}
	}
}
