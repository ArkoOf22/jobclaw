package company

import "strings"

type EvidenceClassifier struct {
	nameClassifier Classifier
}

func NewEvidenceClassifier(nameClassifier Classifier) *EvidenceClassifier {
	return &EvidenceClassifier{
		nameClassifier: nameClassifier,
	}
}

func (c *EvidenceClassifier) Classify(
	company Company,
	evidence Evidence,
) (Classification, string) {
	// Strong evidence from actual job descriptions wins first.
	if len(evidence.ServiceSignals) > len(evidence.ProductSignals) &&
		len(evidence.ServiceSignals) > len(evidence.InternalTechSignals) {
		return ClassificationServices,
			"job-description evidence indicates a services-oriented business"
	}

	if len(evidence.ProductSignals) > len(evidence.ServiceSignals) &&
		len(evidence.ProductSignals) > len(evidence.InternalTechSignals) {
		return ClassificationProduct,
			"job-description evidence indicates a product/platform-oriented business"
	}

	// Internal technology should not automatically be treated as a services
	// company. Keep it separate so downstream job scoring can make the decision.
	if len(evidence.InternalTechSignals) > 0 &&
		len(evidence.InternalTechSignals) >= len(evidence.ProductSignals) &&
		len(evidence.InternalTechSignals) >= len(evidence.ServiceSignals) {
		return ClassificationInternalTech,
			"job-description evidence indicates internal technology systems"
	}

	// Fall back to company-name classification.
	if c.nameClassifier != nil {
		classification, reason := c.nameClassifier.Classify(company)

		if classification != ClassificationUnknown {
			return classification, "name-based classification: " + reason
		}
	}

	return ClassificationUnknown,
		"available company and job-description evidence is insufficient"
}

func containsAny(text string, signals []string) bool {
	text = strings.ToLower(text)

	for _, signal := range signals {
		if strings.Contains(text, strings.ToLower(signal)) {
			return true
		}
	}

	return false
}
