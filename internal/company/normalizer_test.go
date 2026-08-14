package company

import "testing"

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "Adobe",
			want: "adobe",
		},
		{
			name: "  Adobe  ",
			want: "adobe",
		},
		{
			name: "Walmart Global Tech India",
			want: "walmart global tech india",
		},
		{
			name: "Neo-Wealth & Asset Management",
			want: "neo wealth asset management",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeName(tt.name)

			if got != tt.want {
				t.Fatalf(
					"NormalizeName(%q) = %q, want %q",
					tt.name,
					got,
					tt.want,
				)
			}
		})
	}
}
