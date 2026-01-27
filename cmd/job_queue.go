package main

import queue "github.com/nickqweaver/job-queue/internal"

func main() {
	producer := queue.Producer{}
	for id := 1; id <= 1000000; id++ {
		producer.Enqueue("test", id)
	}

	jobs := make(chan *queue.Job)
	consumer := queue.Consumer{InFlight: jobs}
	consumer.Run("test", 10)
}
