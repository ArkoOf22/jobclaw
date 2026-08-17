package application

type ArtifactType string

const (
	ArtifactTailoredResume     ArtifactType = "TAILORED_RESUME"
	ArtifactCoverLetter        ArtifactType = "COVER_LETTER"
	ArtifactReferralMessage    ArtifactType = "REFERRAL_MESSAGE"
	ArtifactApplicationAnswers ArtifactType = "APPLICATION_ANSWERS"
)

type Artifact struct {
	Type ArtifactType
	Path string
}
