package store

type Status int

const (
	Ready Status = iota
	InFlight
	Failed
	Succeeded
)

type Job struct {
	ID      string
	Queue   string
	Status  Status
	Timeout int
}

type JobUpdate struct {
	Status  Status
	Retries *int
}

type Store interface {
	FetchJobs(status Status, limit int) []Job
	FailJob(id string)
	AddJob(job Job)
	UpdateJob(id string, update JobUpdate)
}
