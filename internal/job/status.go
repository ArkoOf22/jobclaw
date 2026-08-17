package job

type Status string

const (
	StatusDiscovered  Status = "DISCOVERED"
	StatusScored      Status = "SCORED"
	StatusShortlisted Status = "SHORTLISTED"
	StatusApproved    Status = "APPROVED"
	StatusApplied     Status = "APPLIED"
	StatusInterview   Status = "INTERVIEW"
	StatusOffer       Status = "OFFER"
	StatusRejected    Status = "REJECTED"
)
