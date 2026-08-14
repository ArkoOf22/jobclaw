package company

import (
	"context"
	"fmt"
	"strings"

	"jobclaw/internal/database"
)

type EvidenceCollector struct {
	db *database.DB
}

func NewEvidenceCollector(db *database.DB) *EvidenceCollector {
	return &EvidenceCollector{db: db}
}

func (c *EvidenceCollector) Collect(
	ctx context.Context,
	companyID int64,
) (Evidence, error) {
	if companyID <= 0 {
		return Evidence{}, fmt.Errorf("company ID must be positive")
	}

	rows, err := c.db.QueryContext(ctx, `
		SELECT title, description
		FROM jobs
		WHERE company_id = ?
		ORDER BY id
	`, companyID)
	if err != nil {
		return Evidence{}, fmt.Errorf("query company jobs: %w", err)
	}
	defer rows.Close()

	var evidence Evidence

	for rows.Next() {
		var (
			title       string
			description string
		)

		if err := rows.Scan(&title, &description); err != nil {
			return Evidence{}, fmt.Errorf("scan company job: %w", err)
		}

		evidence.JobCount++
		evidence.Titles = append(evidence.Titles, title)

		text := strings.ToLower(title + " " + description)

		evidence.ProductSignals = append(
			evidence.ProductSignals,
			findSignals(text, productIndicators)...,
		)

		evidence.ServiceSignals = append(
			evidence.ServiceSignals,
			findSignals(text, serviceIndicators)...,
		)

		evidence.InternalTechSignals = append(
			evidence.InternalTechSignals,
			findSignals(text, internalTechIndicators)...,
		)
	}

	if err := rows.Err(); err != nil {
		return Evidence{}, fmt.Errorf("iterate company jobs: %w", err)
	}

	return evidence, nil
}

var productIndicators = []string{
	"software product",
	"saas",
	"software platform",
	"platform",
	"product",
	"products",
	"cloud platform",
	"managed service",
	"marketplace",
	"payment platform",
	"payments platform",
	"consumer app",
	"mobile app",
	"application security",
	"data platform",
	"ai platform",
}

var serviceIndicators = []string{
	"consulting",
	"consultancy",
	"it services",
	"professional services",
	"managed services",
	"outsourcing",
	"staffing",
	"implementation services",
	"systems integrator",
	"client projects",
	"client engagement",
	"customer projects",
}

var internalTechIndicators = []string{
	"internal platform",
	"internal tools",
	"internal systems",
	"internal technology",
	"internal applications",
	"internal engineering",
	"supporting our business",
	"support our business",
	"internal infrastructure",
}

func findSignals(text string, indicators []string) []string {
	var signals []string

	for _, indicator := range indicators {
		if strings.Contains(text, indicator) {
			signals = append(signals, indicator)
		}
	}

	return signals
}
