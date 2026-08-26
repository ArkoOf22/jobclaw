package job

import "time"

type Job struct {
	ID     int64
	Status Status

	Source         string
	ExternalID     string
	Company        string
	Title          string
	Description    string
	Location       string
	RemoteType     string
	EmploymentType string

	SalaryMin *int
	SalaryMax *int
	Currency  string

	URL      string
	PostedAt *time.Time

	// BoardToken identifies the ATS board this job came from, when the source
	// has one. Recorded at discovery time because it cannot reliably be
	// recovered later: employers often host their board on their own domain,
	// where the token never appears in the job URL.
	BoardToken string

	DiscoveredAt time.Time
}
