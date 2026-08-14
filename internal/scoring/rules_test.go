package scoring

import "testing"

func TestExtractMinimumYears(t *testing.T) {
	tests := []struct {
		name string
		text string
		want float64
		ok   bool
	}{
		{
			name: "plain years",
			text: "2 years experience",
			want: 2,
			ok:   true,
		},
		{
			name: "plus years",
			text: "2+ years experience",
			want: 2,
			ok:   true,
		},
		{
			name: "minimum years",
			text: "minimum 3 years of experience",
			want: 3,
			ok:   true,
		},
		{
			name: "range",
			text: "2-4 years experience",
			want: 2,
			ok:   true,
		},
		{
			name: "unicode range",
			text: "2–4 years experience",
			want: 2,
			ok:   true,
		},
		{
			name: "no experience requirement",
			text: "Build scalable backend services using Go and Kafka.",
			want: 0,
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractMinimumYears(tt.text)

			if ok != tt.ok {
				t.Fatalf("found = %v, want %v", ok, tt.ok)
			}

			if got != tt.want {
				t.Fatalf("years = %.1f, want %.1f", got, tt.want)
			}
		})
	}
}
