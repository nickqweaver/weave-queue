package store

type Status int

const (
	Ready Status = iota
	InFlight
	Failed
	Succeeded
)

type Job struct {
	ID     string
	Queue  string
	Status Status
}

type UpdateJob struct {
	Status Status
}

type Store interface {
	FetchJobs(status Status, offset int, limit int) []Job
	FailJob(id string)
	AddJob(job Job)
	UpdateJob(id string, update UpdateJob)
}
