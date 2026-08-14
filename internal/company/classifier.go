package company

import "strings"

type Classifier interface {
	Classify(company Company) (Classification, string)
}

type RuleBasedClassifier struct{}

func NewRuleBasedClassifier() *RuleBasedClassifier {
	return &RuleBasedClassifier{}
}

func (c *RuleBasedClassifier) Classify(company Company) (Classification, string) {
	name := strings.ToLower(company.Name)

	serviceIndicators := []string{
		"consulting",
		"consultancy",
		"solutions",
		"services",
		"technologies",
		"technology services",
		"outsourcing",
		"staffing",
		"systems integrator",
	}

	productIndicators := []string{
		"software",
		"platform",
		"saas",
		"fintech",
		"payments",
		"bank",
		"banking",
		"commerce",
		"marketplace",
		"cloud",
	}

	for _, indicator := range serviceIndicators {
		if strings.Contains(name, indicator) {
			return ClassificationServices,
				"company name contains service-industry indicator: " + indicator
		}
	}

	for _, indicator := range productIndicators {
		if strings.Contains(name, indicator) {
			return ClassificationProduct,
				"company name contains product-industry indicator: " + indicator
		}
	}

	return ClassificationUnknown,
		"company name does not provide enough evidence"
}
