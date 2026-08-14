package company

import "testing"

func TestRuleBasedClassifier(t *testing.T) {
	classifier := NewRuleBasedClassifier()

	tests := []struct {
		name      string
		wantClass Classification
	}{
		{
			name:      "Acme Consulting",
			wantClass: ClassificationServices,
		},
		{
			name:      "Acme Technology Services",
			wantClass: ClassificationServices,
		},
		{
			name:      "Acme Software",
			wantClass: ClassificationProduct,
		},
		{
			name:      "Acme Payments",
			wantClass: ClassificationProduct,
		},
		{
			name:      "Acme",
			wantClass: ClassificationUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := classifier.Classify(Company{
				Name: tt.name,
			})

			if result != tt.wantClass {
				t.Fatalf(
					"classification = %q, want %q",
					result,
					tt.wantClass,
				)
			}
		})
	}
}
