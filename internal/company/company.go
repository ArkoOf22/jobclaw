package company

import "time"

type Classification string

const (
	ClassificationProduct      Classification = "PRODUCT"
	ClassificationServices     Classification = "SERVICES"
	ClassificationInternalTech Classification = "INTERNAL_TECH"
	ClassificationUnknown      Classification = "UNKNOWN"
)

type Company struct {
	ID                   int64
	Name                 string
	NormalizedName       string
	Classification       Classification
	ClassificationReason string
	ClassifiedAt         *time.Time
	CreatedAt            time.Time
}
