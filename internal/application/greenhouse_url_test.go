package application

import "testing"

func TestParseGreenhouseBoardToken(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "job boards host",
			url:  "https://job-boards.greenhouse.io/acme/jobs/12345",
			want: "acme",
		},
		{
			name: "boards host",
			url:  "https://boards.greenhouse.io/acme/jobs/12345",
			want: "acme",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseGreenhouseBoardToken(test.url)
			if err != nil {
				t.Fatalf("parse URL: %v", err)
			}

			if got != test.want {
				t.Fatalf(
					"board token = %q, want %q",
					got,
					test.want,
				)
			}
		})
	}
}

func TestParseGreenhouseBoardTokenRejectsUnsupportedURL(t *testing.T) {
	_, err := parseGreenhouseBoardToken(
		"https://example.com/jobs/12345",
	)

	if err == nil {
		t.Fatal("expected unsupported URL error")
	}
}
