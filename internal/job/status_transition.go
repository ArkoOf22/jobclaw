package job

func CanTransition(from, to Status) bool {
	if from == to {
		return true
	}

	switch from {
	case StatusDiscovered:
		return to == StatusScored ||
			to == StatusShortlisted ||
			to == StatusRejected

	case StatusScored:
		return to == StatusShortlisted ||
			to == StatusApproved ||
			to == StatusRejected

	case StatusShortlisted:
		return to == StatusApproved ||
			to == StatusRejected

	case StatusApproved:
		return to == StatusApplied ||
			to == StatusRejected

	case StatusApplied:
		return to == StatusInterview ||
			to == StatusRejected

	case StatusInterview:
		return to == StatusOffer ||
			to == StatusRejected

	case StatusOffer:
		return false

	case StatusRejected:
		return false

	default:
		return false
	}
}
