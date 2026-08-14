package company

import "testing"

func TestEvidenceClassifier(t *testing.T) {
	classifier := NewEvidenceClassifier(
		NewRuleBasedClassifier(),
	)

	tests := []struct {
		name     string
		evidence Evidence
		want     Classification
	}{
		{
			name: "product evidence",
			evidence: Evidence{
				ProductSignals: []string{
					"software platform",
					"saas",
				},
			},
			want: ClassificationProduct,
		},
		{
			name: "services evidence",
			evidence: Evidence{
				ServiceSignals: []string{
					"consulting",
					"client projects",
				},
			},
			want: ClassificationServices,
		},
		{
			name: "internal technology evidence",
			evidence: Evidence{
				InternalTechSignals: []string{
					"internal platform",
				},
			},
			want: ClassificationInternalTech,
		},
		{
			name:     "unknown",
			evidence: Evidence{},
			want:     ClassificationUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := classifier.Classify(
				Company{Name: "Acme"},
				tt.evidence,
			)

			if result != tt.want {
				t.Fatalf(
					"classification = %q, want %q",
					result,
					tt.want,
				)
			}
		})
	}
}
