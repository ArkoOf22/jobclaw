package jobspy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"jobclaw/internal/discovery"
	"jobclaw/internal/job"
)

const defaultEndpoint = "http://127.0.0.1:8000/mcp"

// Compile-time verification that Client implements discovery.Source.
var _ discovery.Source = (*Client)(nil)

type Client struct {
	endpoint   string
	httpClient *http.Client

	// now supplies the current time for the search-term rotation window. It is a
	// field only so tests can pin it; production always uses time.Now.
	now func() time.Time
}

func NewClient(endpoint string) *Client {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = defaultEndpoint
	}

	return &Client{
		endpoint: endpoint,
		httpClient: &http.Client{
			// Each discovery run now issues several sequential scrapes over one
			// connection rather than a single call, so the ceiling is generous.
			Timeout: 10 * time.Minute,
		},
		now: time.Now,
	}
}

func (c *Client) Name() string {
	return "jobspy"
}

// searchSites is the set of boards JobSpy scrapes. Glassdoor is included at the
// candidate's request; Naukri may fail internally with a recaptcha error, but
// that failure is contained inside the MCP server and does not stop the others.
var searchSites = []string{"linkedin", "indeed", "glassdoor", "naukri"}

// maxSearchTerms bounds how many distinct queries a single discovery run issues.
// Each term is a separate scrape across every site and takes real time, so the
// list is capped rather than run for all seventeen configured roles.
//
// The cap is a rate-limit decision, not a coverage decision. It used to be both:
// searchTerms took the first five keywords and stopped, so the twelve roles after
// them were configured and never searched once. "Golang Developer" and "Go
// Developer" were among them, on a Go candidate's profile. See rotation below.
const maxSearchTerms = 5

// pinnedSearchTerms is how many leading keywords are searched on every run
// regardless of rotation. The keyword list arrives primary-first, so these are
// the highest-value queries and they should not wait a full cycle after a failed
// run. The rest of the window rotates.
const pinnedSearchTerms = 2

// termRotationPeriod is how long one rotation window lasts. It matches the
// discovery timer's six-hour cadence so that consecutive scheduled runs advance
// to the next slice instead of repeating the same one.
//
// Deriving the slice from the clock keeps this stateless: no cursor column, no
// migration, and an ad-hoc run in between simply lands in whichever window it
// falls in rather than stealing the next scheduled run's slice.
const termRotationPeriod = 6 * time.Hour

func (c *Client) Discover(ctx context.Context, req discovery.Request) ([]job.Job, error) {
	now := c.now
	if now == nil {
		now = time.Now
	}

	terms := searchTerms(req.Keywords, now())
	if len(terms) == 0 {
		return nil, fmt.Errorf("at least one keyword is required")
	}

	location := searchLocation(req.Locations)

	// One search_term per query. Concatenating every role into a single string
	// produced an incoherent mega-query that job boards could not act on, which
	// is why a manual single-term search returned far better results. Each term
	// is now its own clean search, exactly as a person would type it.
	perTerm := req.Limit / len(terms)
	if perTerm < 10 {
		perTerm = 10
	}

	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "jobclaw",
			Version: "0.1.0",
		},
		nil,
	)

	transport := &mcp.StreamableClientTransport{
		Endpoint:             c.endpoint,
		HTTPClient:           c.httpClient,
		DisableStandaloneSSE: true,
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to JobSpy MCP: %w", err)
	}
	defer session.Close()

	// Relevance is enforced on keywords only. Geography is already constrained by
	// the search location and country_indeed below, and a job's location string
	// is often phrased in a way the location matcher would wrongly reject, so
	// applying the full request here would discard genuine India roles. Foreign
	// postings that slip through are vetoed later at scoring.
	relevance := discovery.Request{Keywords: req.Keywords}

	seen := make(map[string]bool)
	collected := make([]job.Job, 0, req.Limit)

	for _, term := range terms {
		args := searchJobsRequest{
			SearchTerm:         term,
			Sites:              searchSites,
			Location:           location,
			ResultsWanted:      perTerm,
			IsRemote:           req.RemoteOnly,
			HoursOld:           req.HoursOld,
			CountryIndeed:      "India",
			IncludeDescription: true,
		}

		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "search_jobs",
			Arguments: args,
		})
		if err != nil {
			return nil, fmt.Errorf("call search_jobs for %q: %w", term, err)
		}

		if result.IsError {
			return nil, fmt.Errorf("JobSpy returned a tool error for %q", term)
		}

		response, err := decodeSearchResponse(result)
		if err != nil {
			return nil, fmt.Errorf("decode JobSpy response for %q: %w", term, err)
		}

		for _, candidate := range normalizeJobs(response.Jobs) {
			if seen[candidate.ExternalID] {
				continue
			}

			if !discovery.MatchesRequest(candidate, relevance) {
				continue
			}

			seen[candidate.ExternalID] = true
			collected = append(collected, candidate)
		}
	}

	return collected, nil
}

// cleanKeywords normalises the configured keywords into a de-duplicated list,
// preserving order so the primary roles stay at the front.
func cleanKeywords(keywords []string) []string {
	seen := make(map[string]bool)
	cleaned := make([]string, 0, len(keywords))

	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}

		key := strings.ToLower(keyword)
		if seen[key] {
			continue
		}

		seen[key] = true
		cleaned = append(cleaned, keyword)
	}

	return cleaned
}

// searchTerms selects which keywords this run searches: a pinned prefix plus a
// rotating slice of the remainder.
//
// A run cannot search every configured role -- each term is a separate scrape
// across four boards, and issuing seventeen of them would both lengthen the run
// past its timeout and scrape harder than the boards tolerate from a datacenter
// address. But capping by truncation meant the tail was never searched at all.
//
// Rotation resolves that without scraping more per run. The first
// pinnedSearchTerms keywords go out every time, because they are the primary
// roles and the most productive queries; the rest of the window advances by one
// slice every termRotationPeriod, so with four scheduled runs a day the whole
// list is covered in roughly a day.
//
// The slice wraps, so coverage is a rolling cycle rather than a fixed schedule.
// Consecutive runs never repeat the same rotating terms unless the list is short
// enough to fit in a single window, in which case rotation is a no-op and every
// term is searched every run.
func searchTerms(keywords []string, at time.Time) []string {
	cleaned := cleanKeywords(keywords)

	// Everything fits: rotation would only introduce gaps for no benefit.
	if len(cleaned) <= maxSearchTerms {
		return cleaned
	}

	pinned := pinnedSearchTerms
	if pinned > len(cleaned) {
		pinned = len(cleaned)
	}

	terms := make([]string, 0, maxSearchTerms)
	terms = append(terms, cleaned[:pinned]...)

	rotating := cleaned[pinned:]

	slots := maxSearchTerms - pinned
	if slots <= 0 || len(rotating) == 0 {
		return terms
	}

	// Window index from the clock rather than stored state. Negative times cannot
	// occur in practice but would produce a negative modulus, so clamp.
	window := at.UTC().Unix() / int64(termRotationPeriod/time.Second)
	if window < 0 {
		window = 0
	}

	start := int(window*int64(slots)) % len(rotating)

	for i := 0; i < slots && i < len(rotating); i++ {
		terms = append(terms, rotating[(start+i)%len(rotating)])
	}

	return terms
}

// searchLocation reduces the preferred locations to a single clean locality and
// guarantees a country, so the scraper searches India rather than defaulting a
// bare city to somewhere else.
func searchLocation(locations []string) string {
	for _, location := range locations {
		location = strings.TrimSpace(location)
		if location == "" {
			continue
		}

		if !strings.Contains(strings.ToLower(location), "india") {
			location += ", India"
		}

		return location
	}

	return "India"
}

func decodeSearchResponse(result *mcp.CallToolResult) (searchJobsResponse, error) {
	var response searchJobsResponse

	for _, content := range result.Content {
		text, ok := content.(*mcp.TextContent)
		if !ok {
			continue
		}

		if err := json.Unmarshal([]byte(text.Text), &response); err == nil {
			return response, nil
		}
	}

	return searchJobsResponse{}, fmt.Errorf("no valid JSON response in tool result")
}

func normalizeJobs(rawJobs []rawJob) []job.Job {
	now := time.Now().UTC()
	jobs := make([]job.Job, 0, len(rawJobs))

	for _, raw := range rawJobs {
		if strings.TrimSpace(raw.ID) == "" ||
			strings.TrimSpace(raw.Company) == "" ||
			strings.TrimSpace(raw.Title) == "" ||
			strings.TrimSpace(raw.JobURL) == "" {
			continue
		}

		postedAt := parsePostedAt(raw.DatePosted)

		remoteType := "ONSITE"
		if raw.IsRemote {
			remoteType = "REMOTE"
		}

		externalID := raw.ID
		if raw.Site != "" {
			externalID = raw.Site + ":" + raw.ID
		}

		jobs = append(jobs, job.Job{
			Source:         raw.Site,
			ExternalID:     externalID,
			Company:        raw.Company,
			Title:          raw.Title,
			Description:    raw.Description,
			Location:       raw.Location,
			RemoteType:     remoteType,
			EmploymentType: raw.JobType,
			URL:            raw.JobURL,
			PostedAt:       postedAt,
			DiscoveredAt:   now,
		})
	}

	return jobs
}

func parsePostedAt(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil
	}

	t = t.UTC()
	return &t
}
