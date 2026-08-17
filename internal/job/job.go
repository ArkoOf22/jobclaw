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

	DiscoveredAt time.Time
}
