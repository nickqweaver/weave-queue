package store

import "time"

type Status int

const (
	Ready Status = iota
	InFlight
	Failed
	Succeeded
)

func (s Status) String() string {
	switch s {
	case Ready:
		return "Ready"
	case InFlight:
		return "InFlight"
	case Failed:
		return "Failed"
	case Succeeded:
		return "Succeeded"
	default:
		return "Unknown"
	}
}

type Job struct {
	ID       string
	Queue    string
	Status   Status
	Timeout  int
	LeasedAt *time.Time
	RetryAt  *time.Time
	Retries  int
}

type JobUpdate struct {
	Status  Status
	Retries *int
}

type Store interface {
	FetchJobs(status Status, limit int) []Job
	FetchAndClaim(curr Status, to Status, limit int) []Job
	FailJob(id string) error
	AddJob(job Job) error
	UpdateJob(id string, update JobUpdate) error
	GetAllJobs() []Job
}
