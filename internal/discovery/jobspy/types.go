package jobspy

type searchJobsRequest struct {
	SearchTerm         string   `json:"search_term"`
	Sites              []string `json:"sites,omitempty"`
	Location           string   `json:"location,omitempty"`
	ResultsWanted      int      `json:"results_wanted,omitempty"`
	JobType            string   `json:"job_type,omitempty"`
	IsRemote           bool     `json:"is_remote,omitempty"`
	HoursOld           int      `json:"hours_old,omitempty"`
	Distance           int      `json:"distance,omitempty"`
	Offset             int      `json:"offset,omitempty"`
	CountryIndeed      string   `json:"country_indeed,omitempty"`
	GoogleSearchTerm   string   `json:"google_search_term,omitempty"`
	IncludeDescription bool     `json:"include_description,omitempty"`
}

type searchJobsResponse struct {
	Count          int      `json:"count"`
	Offset         int      `json:"offset"`
	ResultsPerSite int      `json:"results_per_site"`
	SitesQueried   []string `json:"sites_queried"`
	Truncated      bool     `json:"truncated"`
	NextOffset     int      `json:"next_offset"`
	Jobs           []rawJob `json:"jobs"`
}

type rawJob struct {
	ID          string `json:"id"`
	Site        string `json:"site"`
	Title       string `json:"title"`
	Company     string `json:"company"`
	Location    string `json:"location"`
	DatePosted  string `json:"date_posted"`
	JobType     string `json:"job_type"`
	IsRemote    bool   `json:"is_remote"`
	JobURL      string `json:"job_url"`
	CompanyURL  string `json:"company_url"`
	Description string `json:"description"`
}
