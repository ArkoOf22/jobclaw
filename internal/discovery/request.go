package discovery

type Request struct {
	Keywords []string

	Locations []string

	RemoteOnly bool

	HoursOld int

	Limit int
}
