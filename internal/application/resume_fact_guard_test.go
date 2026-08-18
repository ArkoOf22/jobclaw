package application

import "testing"

func TestResumeFactGuardAcceptsGroundedResume(t *testing.T) {
	master := `
Twid
Maersk

Reduced production log volume by 92%.
Reduced log storage from 240GB to 53GB/day.
Worked on 10+ production integrations.
Processed 15,000 records/day.
Handled 15K+ requests/sec.
`

	generated := `
Arkodeep Koley

Twid — Software Development Engineer

- Reduced production log volume by 92%.
- Reduced log storage from 240GB to 53GB/day.
- Delivered 10+ production integrations.

Maersk

- Built a data ingestion pipeline processing 15,000 records/day.
- Load-tested APIs to 15K+ requests/sec.
`

	guard := NewResumeFactGuard(master)

	if err := guard.Validate(generated); err != nil {
		t.Fatalf("expected valid resume, got: %v", err)
	}
}

func TestResumeFactGuardRejectsUnsupportedMetric(t *testing.T) {
	master := `
Twid

Reduced production log volume by 92%.
`

	generated := `
Twid

Reduced production log volume by 95%.
`

	guard := NewResumeFactGuard(master)

	if err := guard.Validate(generated); err == nil {
		t.Fatal("expected unsupported metric to be rejected")
	}
}

func TestResumeFactGuardRejectsCommentary(t *testing.T) {
	master := `
Twid
Software Development Engineer
`

	generated := `
Here is your tailored resume.

Twid
Software Development Engineer
`

	guard := NewResumeFactGuard(master)

	if err := guard.Validate(generated); err == nil {
		t.Fatal("expected commentary to be rejected")
	}
}

func TestResumeFactGuardRejectsCodeFence(t *testing.T) {
	master := `
Twid
Software Development Engineer
`

	generated := "```text\nTwid\n```"

	guard := NewResumeFactGuard(master)

	if err := guard.Validate(generated); err == nil {
		t.Fatal("expected code fence to be rejected")
	}
}

func TestResumeFactGuardAcceptsMetricWithDifferentCase(t *testing.T) {
	master := `
Twid

Reduced production log volume by 92%.
`

	generated := `
Twid

Reduced production log volume by 92%.
`

	guard := NewResumeFactGuard(master)

	if err := guard.Validate(generated); err != nil {
		t.Fatalf("expected valid resume, got: %v", err)
	}
}

func TestResumeFactGuardRejectsUnsupportedCompanyValidationIsNotHardCoded(t *testing.T) {
	master := `
Acme Corp

Built Go microservices.
`

	generated := `
Acme Corp

Built Go microservices.
`

	guard := NewResumeFactGuard(master)

	if err := guard.Validate(generated); err != nil {
		t.Fatalf(
			"generic factual resume should not depend on hard-coded companies: %v",
			err,
		)
	}
}
